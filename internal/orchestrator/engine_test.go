package orchestrator_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/adapters"
	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/orchestrator"
	"github.com/xinchenglearning/agent-orchestrator/internal/policy"
	"github.com/xinchenglearning/agent-orchestrator/internal/redaction"
)

func TestEngineCompletesVerifiedSingleWriterLoop(t *testing.T) {
	repo, base := engineFixtureRepository(t)
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	task := engineTask(canonicalRepo, base)
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	writer := &fixtureAdapter{}
	engine := orchestrator.Engine{
		Adapter:  writer,
		Mode:     policy.ExecutionTrustedLocal,
		Redactor: redaction.New(nil),
		NewRunID: func() string { return "run-test" },
	}

	run, err := engine.Run(context.Background(), task, digest)
	if err != nil {
		t.Fatal(err)
	}
	if run.State != domain.RunAwaitingApproval {
		t.Fatalf("got state %s", run.State)
	}
	if writer.starts != 1 {
		t.Fatalf("writer starts=%d", writer.starts)
	}
	if run.DecisionPacketDigest == "" || run.EvidenceDigest == "" {
		t.Fatalf("missing evidence digests: %+v", run)
	}
	for _, name := range []string{
		"diff.patch",
		"evidence-bundle.json",
		"decision-packet.json",
	} {
		if _, err := os.Stat(filepath.Join(run.EvidenceDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	approval := domain.Approval{
		Actor:                "maintainer@example.com",
		Decision:             domain.DecisionApprove,
		TaskDigest:           digest,
		DecisionPacketDigest: run.DecisionPacketDigest,
		At:                   time.Now(),
	}
	completed, err := engine.Decide(
		context.Background(),
		canonicalRepo,
		run.ID,
		approval,
	)
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != domain.RunCompleted {
		t.Fatalf("got state %s", completed.State)
	}
}

func TestEngineRejectsStrictModeBeforeWriterStarts(t *testing.T) {
	repo, base := engineFixtureRepository(t)
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	task := engineTask(canonicalRepo, base)
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	writer := &fixtureAdapter{}
	engine := orchestrator.Engine{
		Adapter:  writer,
		Mode:     policy.ExecutionStrict,
		Redactor: redaction.New(nil),
		NewRunID: func() string { return "strict-test" },
	}
	if _, err := engine.Run(context.Background(), task, digest); err == nil {
		t.Fatal("expected strict mode rejection")
	}
	if writer.starts != 0 {
		t.Fatalf("writer unexpectedly started %d times", writer.starts)
	}
}

func TestEnginePersistsFailedStateWhenVerificationFails(t *testing.T) {
	repo, base := engineFixtureRepository(t)
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	task := engineTask(canonicalRepo, base)
	task.Acceptance = []domain.CommandSpec{{
		Argv:           []string{"/usr/bin/false"},
		TimeoutSeconds: 10,
	}}
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	engine := orchestrator.Engine{
		Adapter:  &fixtureAdapter{},
		Mode:     policy.ExecutionTrustedLocal,
		Redactor: redaction.New(nil),
		NewRunID: func() string { return "verification-failure" },
	}

	run, err := engine.Run(context.Background(), task, digest)
	if err == nil {
		t.Fatal("expected verification failure")
	}
	if run.State != domain.RunFailed {
		t.Fatalf("returned state %s, want %s", run.State, domain.RunFailed)
	}
	persisted, statusErr := engine.Status(
		context.Background(),
		canonicalRepo,
		run.ID,
	)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if persisted.State != domain.RunFailed {
		t.Fatalf("persisted state %s, want %s", persisted.State, domain.RunFailed)
	}
}

func TestEngineAppliesBudgetToEntireRun(t *testing.T) {
	repo, base := engineFixtureRepository(t)
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	task := engineTask(canonicalRepo, base)
	task.Budget.MaxSeconds = 1
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	engine := orchestrator.Engine{
		Adapter:  blockingAdapter{},
		Mode:     policy.ExecutionTrustedLocal,
		Redactor: redaction.New(nil),
		NewRunID: func() string { return "budget-timeout" },
	}

	started := time.Now()
	run, err := engine.Run(context.Background(), task, digest)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got error %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("run exceeded budget by too much: %s", elapsed)
	}
	if run.State != domain.RunRecoveryRequired {
		t.Fatalf("got state %s, want %s", run.State, domain.RunRecoveryRequired)
	}
}

func TestEngineRejectsDeniedVerificationCommandBeforeWriterStarts(t *testing.T) {
	repo, base := engineFixtureRepository(t)
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	task := engineTask(canonicalRepo, base)
	task.Acceptance = []domain.CommandSpec{{
		Argv:           []string{"git", "push", "origin", "main"},
		TimeoutSeconds: 10,
	}}
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	writer := &fixtureAdapter{}
	engine := orchestrator.Engine{
		Adapter:  writer,
		Mode:     policy.ExecutionTrustedLocal,
		Redactor: redaction.New(nil),
		NewRunID: func() string { return "denied-command" },
	}

	if _, err := engine.Run(context.Background(), task, digest); err == nil {
		t.Fatal("expected denied command rejection")
	}
	if writer.starts != 0 {
		t.Fatalf("writer unexpectedly started %d times", writer.starts)
	}
}

func TestEnginePersistsRecoveryWorkspaceForAmbiguousWriterFailure(t *testing.T) {
	repo, base := engineFixtureRepository(t)
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	task := engineTask(canonicalRepo, base)
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	engine := orchestrator.Engine{
		Adapter:  dirtyFailAdapter{},
		Mode:     policy.ExecutionTrustedLocal,
		Redactor: redaction.New(nil),
		NewRunID: func() string { return "ambiguous-writer" },
	}

	run, err := engine.Run(context.Background(), task, digest)
	if err == nil {
		t.Fatal("expected ambiguous writer failure")
	}
	if run.State != domain.RunRecoveryRequired {
		t.Fatalf("got state %s, want %s", run.State, domain.RunRecoveryRequired)
	}
	if run.RecoveryWorkspace == "" {
		t.Fatal("recovery workspace was not persisted")
	}
	if _, statErr := os.Stat(filepath.Join(run.RecoveryWorkspace, "UNCOMMITTED.md")); statErr != nil {
		t.Fatalf("recovery workspace is not inspectable: %v", statErr)
	}
	persisted, statusErr := engine.Status(context.Background(), canonicalRepo, run.ID)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if persisted.RecoveryWorkspace != run.RecoveryWorkspace {
		t.Fatalf(
			"persisted recovery workspace %q, want %q",
			persisted.RecoveryWorkspace,
			run.RecoveryWorkspace,
		)
	}
}

func TestEnginePreservesCleanRecoveryWorkspace(t *testing.T) {
	repo, base := engineFixtureRepository(t)
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	task := engineTask(canonicalRepo, base)
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	engine := orchestrator.Engine{
		Adapter:  cleanFailAdapter{},
		Mode:     policy.ExecutionTrustedLocal,
		Redactor: redaction.New(nil),
		NewRunID: func() string { return "clean-ambiguous-writer" },
	}

	run, err := engine.Run(context.Background(), task, digest)
	if err == nil {
		t.Fatal("expected ambiguous writer failure")
	}
	if run.State != domain.RunRecoveryRequired {
		t.Fatalf("got state %s, want %s", run.State, domain.RunRecoveryRequired)
	}
	if _, statErr := os.Stat(run.RecoveryWorkspace); statErr != nil {
		t.Fatalf("clean recovery workspace was removed: %v", statErr)
	}
}

type fixtureAdapter struct {
	starts int
}

func (a *fixtureAdapter) Probe(context.Context) (adapters.Capabilities, error) {
	return adapters.Capabilities{
		StructuredOutput: true,
		Cancellation:     true,
		WorkspaceWrite:   true,
		NativeSandbox:    false,
	}, nil
}

func (a *fixtureAdapter) Start(_ context.Context, job adapters.Job) (adapters.Session, error) {
	a.starts++
	if err := os.WriteFile(
		filepath.Join(job.WorkspaceDir, "README.md"),
		[]byte("after\n"),
		0o600,
	); err != nil {
		return nil, err
	}
	engineGit(nil, job.WorkspaceDir, "add", "README.md")
	engineGit(nil, job.WorkspaceDir, "commit", "-m", "fixture change")
	head := strings.TrimSpace(engineGit(nil, job.WorkspaceDir, "rev-parse", "HEAD"))
	return &fixtureSession{
		result: adapters.Result{
			SessionID:    "fixture",
			ResultCommit: head,
		},
		events: make(chan adapters.Event),
	}, nil
}

type fixtureSession struct {
	result adapters.Result
	events chan adapters.Event
	once   sync.Once
}

func (s *fixtureSession) Events() <-chan adapters.Event {
	s.once.Do(func() { close(s.events) })
	return s.events
}

func (s *fixtureSession) Wait(context.Context) (adapters.Result, error) {
	return s.result, nil
}

func (s *fixtureSession) Cancel(context.Context) error {
	return nil
}

type blockingAdapter struct{}

func (blockingAdapter) Probe(context.Context) (adapters.Capabilities, error) {
	return adapters.Capabilities{WorkspaceWrite: true}, nil
}

func (blockingAdapter) Start(
	_ context.Context,
	_ adapters.Job,
) (adapters.Session, error) {
	return blockingSession{}, nil
}

type blockingSession struct{}

func (blockingSession) Events() <-chan adapters.Event {
	events := make(chan adapters.Event)
	close(events)
	return events
}

func (blockingSession) Wait(ctx context.Context) (adapters.Result, error) {
	<-ctx.Done()
	return adapters.Result{}, ctx.Err()
}

func (blockingSession) Cancel(context.Context) error {
	return nil
}

type dirtyFailAdapter struct{}

func (dirtyFailAdapter) Probe(context.Context) (adapters.Capabilities, error) {
	return adapters.Capabilities{WorkspaceWrite: true}, nil
}

func (dirtyFailAdapter) Start(
	_ context.Context,
	job adapters.Job,
) (adapters.Session, error) {
	if err := os.WriteFile(
		filepath.Join(job.WorkspaceDir, "UNCOMMITTED.md"),
		[]byte("inspect me\n"),
		0o600,
	); err != nil {
		return nil, err
	}
	return failingSession{}, nil
}

type failingSession struct{}

func (failingSession) Events() <-chan adapters.Event {
	events := make(chan adapters.Event)
	close(events)
	return events
}

func (failingSession) Wait(context.Context) (adapters.Result, error) {
	return adapters.Result{}, errors.New("writer result is ambiguous")
}

func (failingSession) Cancel(context.Context) error {
	return nil
}

type cleanFailAdapter struct{}

func (cleanFailAdapter) Probe(context.Context) (adapters.Capabilities, error) {
	return adapters.Capabilities{WorkspaceWrite: true}, nil
}

func (cleanFailAdapter) Start(
	context.Context,
	adapters.Job,
) (adapters.Session, error) {
	return failingSession{}, nil
}

func engineTask(repo, base string) domain.Task {
	return domain.Task{
		ID: "engine-test",
		Repository: domain.RepositorySpec{
			Root:    repo,
			BaseRef: base,
		},
		Objective:    "Change README from before to after",
		Rationale:    "Prove the completion loop",
		Deliverables: []string{"one commit"},
		AllowedPaths: []string{"README.md"},
		Acceptance: []domain.CommandSpec{{
			Argv:           []string{"/usr/bin/grep", "-q", "after", "README.md"},
			TimeoutSeconds: 10,
		}},
		Delegation: domain.DelegationExecuteReversible,
		Budget:     domain.Budget{MaxAttempts: 1, MaxSeconds: 60},
	}
}

func engineFixtureRepository(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	engineGit(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	engineGit(t, repo, "add", "README.md")
	engineGit(t, repo, "commit", "-m", "initial")
	return repo, strings.TrimSpace(engineGit(t, repo, "rev-parse", "HEAD"))
}

func engineGit(t *testing.T, dir string, args ...string) string {
	if t != nil {
		t.Helper()
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Fixture",
		"GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=Fixture",
		"GIT_COMMITTER_EMAIL=fixture@example.com",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if t != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		panic(string(output))
	}
	return string(output)
}
