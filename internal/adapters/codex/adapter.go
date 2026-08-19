package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/adapters"
	process "github.com/xinchenglearning/agent-orchestrator/internal/runtime/process"
	"github.com/xinchenglearning/agent-orchestrator/internal/workspace"
)

type Options struct {
	Path        string
	PrefixArgs  []string
	Env         map[string]string
	ResolveHead func(context.Context, string) (string, error)
}

type Adapter struct {
	options Options
}

func New(options Options) *Adapter {
	if options.Path == "" {
		options.Path = "codex"
	}
	return &Adapter{options: options}
}

func Default() *Adapter {
	env := make(map[string]string)
	for _, key := range []string{
		"PATH",
		"HOME",
		"TMPDIR",
		"CODEX_HOME",
		"CODEX_API_KEY",
		"OPENAI_API_KEY",
		"HTTPS_PROXY",
		"HTTP_PROXY",
		"NO_PROXY",
		"SSL_CERT_FILE",
	} {
		if value, ok := os.LookupEnv(key); ok {
			env[key] = value
		}
	}
	env["GIT_AUTHOR_NAME"] = "agent-orchestrator"
	env["GIT_AUTHOR_EMAIL"] = "agent-orchestrator@localhost"
	env["GIT_COMMITTER_NAME"] = "agent-orchestrator"
	env["GIT_COMMITTER_EMAIL"] = "agent-orchestrator@localhost"
	return New(Options{Env: env})
}

func (a *Adapter) Probe(ctx context.Context) (adapters.Capabilities, error) {
	result, err := process.Run(ctx, process.Spec{
		Path:    a.options.Path,
		Args:    append(append([]string(nil), a.options.PrefixArgs...), "--version"),
		Env:     a.options.Env,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return adapters.Capabilities{}, fmt.Errorf("probe codex: %w: %s", err, result.Stderr)
	}
	return adapters.Capabilities{
		StructuredOutput: true,
		Cancellation:     true,
		SessionResume:    false,
		ReadOnly:         false,
		WorkspaceWrite:   true,
		NativeSandbox:    false,
		Streaming:        false,
	}, nil
}

func (a *Adapter) Start(parent context.Context, job adapters.Job) (adapters.Session, error) {
	if job.WorkspaceDir == "" || job.EvidenceDir == "" {
		return nil, errors.New("workspace and evidence directories are required")
	}
	if err := os.MkdirAll(job.EvidenceDir, 0o700); err != nil {
		return nil, fmt.Errorf("create evidence directory: %w", err)
	}
	schemaPath := filepath.Join(job.EvidenceDir, "codex-result.schema.json")
	if err := os.WriteFile(schemaPath, []byte(resultSchema), 0o600); err != nil {
		return nil, fmt.Errorf("write result schema: %w", err)
	}
	lastMessage, err := os.CreateTemp("", "agent-orchestrator-codex-result-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary result file: %w", err)
	}
	lastMessagePath := lastMessage.Name()
	if err := lastMessage.Close(); err != nil {
		os.Remove(lastMessagePath)
		return nil, fmt.Errorf("close temporary result file: %w", err)
	}
	prompt, err := buildPrompt(job)
	if err != nil {
		os.Remove(lastMessagePath)
		return nil, err
	}

	ctx, cancel := context.WithCancel(parent)
	session := &session{
		cancel: cancel,
		events: make(chan adapters.Event, 8),
		done:   make(chan struct{}),
	}
	session.events <- adapters.Event{
		Type:       "session_started",
		Message:    "Codex writer started",
		At:         time.Now(),
		Generation: job.Generation,
	}
	go session.run(ctx, a.options, job, schemaPath, lastMessagePath, prompt)
	return session, nil
}

type outcome struct {
	result adapters.Result
	err    error
}

type session struct {
	cancel  context.CancelFunc
	events  chan adapters.Event
	done    chan struct{}
	mu      sync.RWMutex
	outcome outcome
}

func (s *session) Events() <-chan adapters.Event {
	return s.events
}

func (s *session) Wait(ctx context.Context) (adapters.Result, error) {
	select {
	case <-s.done:
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.outcome.result, s.outcome.err
	case <-ctx.Done():
		return adapters.Result{}, ctx.Err()
	}
}

func (s *session) Cancel(context.Context) error {
	s.cancel()
	return nil
}

func (s *session) run(
	ctx context.Context,
	options Options,
	job adapters.Job,
	schemaPath string,
	lastMessagePath string,
	prompt string,
) {
	defer os.Remove(lastMessagePath)
	defer close(s.events)
	defer close(s.done)
	args := append([]string(nil), options.PrefixArgs...)
	args = append(args,
		"exec",
		"--sandbox", "workspace-write",
		"--ephemeral",
		"--json",
		"--color", "never",
		"--cd", job.WorkspaceDir,
		"--output-schema", schemaPath,
		"--output-last-message", lastMessagePath,
		prompt,
	)
	startedAt := time.Now()
	processResult, err := process.Run(ctx, process.Spec{
		Path:    options.Path,
		Args:    args,
		Dir:     job.WorkspaceDir,
		Env:     options.Env,
		Timeout: time.Duration(job.Budget.MaxSeconds) * time.Second,
	})
	if err != nil {
		s.setOutcome(adapters.Result{}, fmt.Errorf("run codex: %w: %s", err, processResult.Stderr))
		return
	}
	data, err := os.ReadFile(lastMessagePath)
	if err != nil {
		s.setOutcome(adapters.Result{}, fmt.Errorf("read codex terminal result: %w", err))
		return
	}
	result, err := adapters.ParseStructuredResult(string(data))
	if err != nil {
		s.setOutcome(adapters.Result{}, err)
		return
	}
	resolveHead := options.ResolveHead
	if resolveHead == nil {
		resolveHead = workspace.Head
	}
	resultCommit, err := resolveHead(ctx, job.WorkspaceDir)
	if err != nil {
		s.setOutcome(adapters.Result{}, fmt.Errorf("resolve writer HEAD: %w", err))
		return
	}
	result.ResultCommit = resultCommit
	result.Usage.WallTime = time.Since(startedAt)
	s.events <- adapters.Event{
		Type:       "session_finished",
		Message:    "Codex writer finished",
		At:         time.Now(),
		Generation: job.Generation,
	}
	s.setOutcome(result, nil)
}

func (s *session) setOutcome(result adapters.Result, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outcome = outcome{result: result, err: err}
}

func buildPrompt(job adapters.Job) (string, error) {
	taskJSON, err := json.Marshal(job.Task)
	if err != nil {
		return "", fmt.Errorf("marshal task prompt: %w", err)
	}
	return fmt.Sprintf(
		"You are the writer for a bounded repository task.\n"+
			"Task digest: %s\nTask contract: %s\n"+
			"Work exclusively in the current working directory. "+
			"Repository.Root identifies the source repository, not a writable path.\n"+
			"Modify only allowed paths. Do not push, merge, deploy, or access production systems.\n"+
			"Run relevant local checks, create exactly one Git commit, then return the required JSON "+
			"with a non-empty sessionId and resultCommit equal to git rev-parse HEAD.",
		job.TaskDigest,
		taskJSON,
	), nil
}

const resultSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "sessionId": {"type": "string", "minLength": 1},
    "resultCommit": {"type": "string", "pattern": "^[0-9a-f]{40}$"}
  },
  "required": ["sessionId", "resultCommit"]
}
`
