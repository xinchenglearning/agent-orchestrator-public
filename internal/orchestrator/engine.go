package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/adapters"
	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/evidence"
	"github.com/xinchenglearning/agent-orchestrator/internal/policy"
	"github.com/xinchenglearning/agent-orchestrator/internal/redaction"
	"github.com/xinchenglearning/agent-orchestrator/internal/store"
	"github.com/xinchenglearning/agent-orchestrator/internal/verification"
	"github.com/xinchenglearning/agent-orchestrator/internal/workspace"
)

type Run struct {
	ID                   string               `json:"id"`
	State                domain.RunState      `json:"state"`
	TaskDigest           string               `json:"taskDigest"`
	DecisionPacketDigest string               `json:"decisionPacketDigest,omitempty"`
	EvidenceDigest       string               `json:"evidenceDigest,omitempty"`
	VerificationDigest   string               `json:"verificationDigest,omitempty"`
	RepositoryRoot       string               `json:"repositoryRoot"`
	BaseCommit           string               `json:"baseCommit"`
	ResultCommit         string               `json:"resultCommit,omitempty"`
	RecoveryWorkspace    string               `json:"recoveryWorkspace,omitempty"`
	ExecutionMode        policy.ExecutionMode `json:"executionMode"`
	Generation           uint64               `json:"generation"`
	EvidenceDir          string               `json:"evidenceDir"`
}

type Engine struct {
	Adapter  adapters.Adapter
	Mode     policy.ExecutionMode
	Redactor redaction.Redactor
	NewRunID func() string
}

func (e Engine) Run(
	ctx context.Context,
	task domain.Task,
	approvedTaskDigest string,
) (Run, error) {
	if e.Adapter == nil {
		return Run{}, errors.New("writer adapter is required")
	}
	if err := policy.ValidateApprovedTask(task, approvedTaskDigest); err != nil {
		return Run{}, err
	}
	ctx, cancel := context.WithTimeout(
		ctx,
		time.Duration(task.Budget.MaxSeconds)*time.Second,
	)
	defer cancel()
	runPolicy := policy.Policy{
		AllowWriteRole: "writer",
		DeniedActions: map[string]struct{}{
			"deploy": {},
			"merge":  {},
			"push":   {},
		},
	}
	for _, command := range task.Acceptance {
		if err := runPolicy.ValidateCommand(command.Argv); err != nil {
			return Run{}, err
		}
	}
	repository, err := workspace.Resolve(ctx, task.Repository.Root)
	if err != nil {
		return Run{}, err
	}
	if repository.Root != task.Repository.Root {
		return Run{}, fmt.Errorf(
			"task repository root %q is not canonical %q",
			task.Repository.Root,
			repository.Root,
		)
	}
	capabilities, err := e.Adapter.Probe(ctx)
	if err != nil {
		return Run{}, err
	}
	if err := runPolicy.ValidateWriteRole("writer"); err != nil {
		return Run{}, err
	}
	if err := runPolicy.ValidateExecution(e.Mode, policy.Capabilities{
		WorkspaceWrite: capabilities.WorkspaceWrite,
		NativeSandbox:  capabilities.NativeSandbox,
	}); err != nil {
		return Run{}, err
	}

	runID := e.runID()
	fileStore := store.New(filepath.Join(repository.CommonDir, "agent-orchestrator"))
	run := Run{
		ID:             runID,
		State:          domain.RunCreated,
		TaskDigest:     approvedTaskDigest,
		RepositoryRoot: repository.Root,
		BaseCommit:     task.Repository.BaseRef,
		ExecutionMode:  e.Mode,
		Generation:     1,
		EvidenceDir:    filepath.Join(fileStore.RunDir(runID), "evidence"),
	}
	lock, err := store.AcquireLock(fileStore.RunDir(runID), store.LockOwner{
		PID:        os.Getpid(),
		Command:    "run",
		Generation: run.Generation,
		AcquiredAt: time.Now(),
	})
	if err != nil {
		return Run{}, err
	}
	defer lock.Release()
	if err := fileStore.WriteState(runID, run); err != nil {
		return Run{}, err
	}
	if err := transition(fileStore, &run, domain.EventPreparationStarted); err != nil {
		return Run{}, err
	}

	workRoot, err := os.MkdirTemp("", "agent-orchestrator-"+runID+"-")
	if err != nil {
		return e.failed(
			fileStore,
			run,
			fmt.Errorf("create run workspace root: %w", err),
		)
	}
	writerDir := filepath.Join(workRoot, "writer")
	verifierDir := filepath.Join(workRoot, "verifier")
	if err := workspace.Add(ctx, repository.Root, writerDir, task.Repository.BaseRef); err != nil {
		return e.failed(fileStore, run, err)
	}
	preserveWriter := false
	defer func() {
		if !preserveWriter {
			_ = workspace.Remove(context.Background(), repository.Root, writerDir)
		}
	}()
	if err := transition(fileStore, &run, domain.EventImplementationStarted); err != nil {
		return Run{}, err
	}

	effectID := "writer-1"
	if err := fileStore.AppendEvent(runID, store.Event{
		Type:       store.EventEffectIntent,
		EffectID:   effectID,
		Generation: run.Generation,
	}); err != nil {
		return Run{}, err
	}
	session, err := e.Adapter.Start(ctx, adapters.Job{
		Task:          task,
		TaskDigest:    approvedTaskDigest,
		Role:          "writer",
		ExecutionMode: e.Mode,
		WorkspaceDir:  writerDir,
		EvidenceDir:   run.EvidenceDir,
		Budget:        task.Budget,
		Generation:    run.Generation,
	})
	if err != nil {
		preserveWriter = true
		return e.recoveryRequired(fileStore, run, writerDir, err)
	}
	go func() {
		for range session.Events() {
		}
	}()
	result, err := session.Wait(ctx)
	if err != nil {
		preserveWriter = true
		return e.recoveryRequired(fileStore, run, writerDir, err)
	}
	if err := fileStore.AppendEvent(runID, store.Event{
		Type:       store.EventEffectSettlement,
		EffectID:   effectID,
		Generation: run.Generation,
	}); err != nil {
		preserveWriter = true
		return e.recoveryRequired(fileStore, run, writerDir, err)
	}
	head, err := workspace.Head(ctx, writerDir)
	if err != nil {
		return e.failed(fileStore, run, err)
	}
	if head != result.ResultCommit {
		return e.failed(fileStore, run, fmt.Errorf(
			"adapter result commit %s does not match writer HEAD %s",
			result.ResultCommit,
			head,
		))
	}
	if result.ResultCommit == task.Repository.BaseRef {
		return e.failed(
			fileStore,
			run,
			errors.New("writer did not produce a new commit"),
		)
	}
	if err := workspace.ValidateResultCommit(
		ctx,
		writerDir,
		task.Repository.BaseRef,
		result.ResultCommit,
	); err != nil {
		return e.failed(fileStore, run, err)
	}
	changedPaths, err := workspace.ChangedPaths(
		ctx,
		writerDir,
		task.Repository.BaseRef,
		result.ResultCommit,
	)
	if err != nil {
		return e.failed(fileStore, run, err)
	}
	if err := workspace.ValidateAllowedPaths(changedPaths, task.AllowedPaths); err != nil {
		return e.failed(fileStore, run, err)
	}
	diff, err := workspace.Diff(ctx, writerDir, task.Repository.BaseRef, result.ResultCommit)
	if err != nil {
		return e.failed(fileStore, run, err)
	}
	run.ResultCommit = result.ResultCommit
	if err := transition(fileStore, &run, domain.EventImplementationFinished); err != nil {
		return Run{}, err
	}

	if err := workspace.Add(ctx, repository.Root, verifierDir, result.ResultCommit); err != nil {
		return e.failed(fileStore, run, err)
	}
	defer workspace.Remove(context.Background(), repository.Root, verifierDir)
	verificationResults, err := verification.Run(
		ctx,
		verifierDir,
		task.Acceptance,
		verificationEnvironment(),
	)
	if err != nil {
		return e.failed(fileStore, run, fmt.Errorf(
			"verify result commit %s with changed paths %v: %w",
			result.ResultCommit,
			changedPaths,
			err,
		))
	}
	for i := range verificationResults {
		verificationResults[i].Stdout = e.Redactor.Redact(verificationResults[i].Stdout)
		verificationResults[i].Stderr = e.Redactor.Redact(verificationResults[i].Stderr)
	}
	sanitizedDiff := []byte(e.Redactor.Redact(string(diff)))
	verificationDigest, err := evidence.Digest(verificationResults)
	if err != nil {
		return Run{}, err
	}
	bundle := evidence.EvidenceBundle{
		RunID:         runID,
		TaskDigest:    approvedTaskDigest,
		ExecutionMode: string(e.Mode),
		BaseCommit:    task.Repository.BaseRef,
		ResultCommit:  result.ResultCommit,
		Artifacts: []evidence.Artifact{
			evidence.ArtifactFromBytes("diff.patch", sanitizedDiff),
		},
		Verification: verificationResults,
	}
	evidenceDigest, err := evidence.Digest(bundle)
	if err != nil {
		return Run{}, err
	}
	packet := evidence.DecisionPacket{
		TaskDigest:         approvedTaskDigest,
		EvidenceDigest:     evidenceDigest,
		VerificationDigest: verificationDigest,
	}
	packetDigest, err := evidence.Digest(packet)
	if err != nil {
		return Run{}, err
	}
	if err := writeEvidence(run.EvidenceDir, "diff.patch", sanitizedDiff); err != nil {
		return Run{}, err
	}
	if err := writeEvidenceJSON(run.EvidenceDir, "evidence-bundle.json", bundle); err != nil {
		return Run{}, err
	}
	if err := writeEvidenceJSON(run.EvidenceDir, "decision-packet.json", packet); err != nil {
		return Run{}, err
	}
	run.VerificationDigest = verificationDigest
	run.EvidenceDigest = evidenceDigest
	run.DecisionPacketDigest = packetDigest
	if err := transition(fileStore, &run, domain.EventVerificationFinished); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (e Engine) Decide(
	ctx context.Context,
	repositoryRoot string,
	runID string,
	approval domain.Approval,
) (Run, error) {
	repository, err := workspace.Resolve(ctx, repositoryRoot)
	if err != nil {
		return Run{}, err
	}
	fileStore := store.New(filepath.Join(repository.CommonDir, "agent-orchestrator"))
	var run Run
	if err := fileStore.ReadState(runID, &run); err != nil {
		return Run{}, err
	}
	lock, err := store.AcquireLock(fileStore.RunDir(runID), store.LockOwner{
		PID:        os.Getpid(),
		Command:    "decide",
		Generation: run.Generation,
		AcquiredAt: time.Now(),
	})
	if err != nil {
		return Run{}, err
	}
	defer lock.Release()
	if run.State != domain.RunAwaitingApproval {
		return Run{}, fmt.Errorf("run %s is not awaiting approval", runID)
	}
	if err := domain.ValidateApproval(
		approval,
		run.TaskDigest,
		run.DecisionPacketDigest,
	); err != nil {
		return Run{}, err
	}
	var event domain.EventType
	switch approval.Decision {
	case domain.DecisionApprove:
		event = domain.EventApproved
	case domain.DecisionRequestRework:
		event = domain.EventReworkRequested
	case domain.DecisionReject:
		event = domain.EventRejected
	case domain.DecisionCancel:
		event = domain.EventCanceled
	default:
		return Run{}, fmt.Errorf("unsupported decision %q", approval.Decision)
	}
	if err := transition(fileStore, &run, event); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (e Engine) Status(
	ctx context.Context,
	repositoryRoot string,
	runID string,
) (Run, error) {
	repository, err := workspace.Resolve(ctx, repositoryRoot)
	if err != nil {
		return Run{}, err
	}
	fileStore := store.New(filepath.Join(repository.CommonDir, "agent-orchestrator"))
	var run Run
	if err := fileStore.ReadState(runID, &run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (e Engine) recoveryRequired(
	fileStore *store.FileStore,
	run Run,
	recoveryWorkspace string,
	cause error,
) (Run, error) {
	run.RecoveryWorkspace = recoveryWorkspace
	if err := transition(fileStore, &run, domain.EventWriteAmbiguous); err != nil {
		return Run{}, errors.Join(cause, err)
	}
	return run, cause
}

func (e Engine) failed(
	fileStore *store.FileStore,
	run Run,
	cause error,
) (Run, error) {
	if err := transition(fileStore, &run, domain.EventFailed); err != nil {
		return Run{}, errors.Join(cause, err)
	}
	return run, cause
}

func transition(
	fileStore *store.FileStore,
	run *Run,
	event domain.EventType,
) error {
	next, err := domain.Transition(run.State, event)
	if err != nil {
		return err
	}
	if err := fileStore.AppendEvent(run.ID, store.Event{
		Type:       string(event),
		Generation: run.Generation,
	}); err != nil {
		return err
	}
	run.State = next
	return fileStore.WriteState(run.ID, run)
}

func (e Engine) runID() string {
	if e.NewRunID != nil {
		return e.NewRunID()
	}
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("generate run ID: %v", err))
	}
	return hex.EncodeToString(value[:])
}

func verificationEnvironment() map[string]string {
	result := make(map[string]string)
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "GOCACHE"} {
		if value, ok := os.LookupEnv(key); ok {
			result[key] = value
		}
	}
	return result
}

func writeEvidenceJSON(directory, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evidence %s: %w", name, err)
	}
	return writeEvidence(directory, name, append(data, '\n'))
}

func writeEvidence(directory, name string, data []byte) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}
	file, err := os.OpenFile(
		filepath.Join(directory, name),
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return fmt.Errorf("create evidence %s: %w", name, err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write evidence %s: %w", name, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync evidence %s: %w", name, err)
	}
	return nil
}
