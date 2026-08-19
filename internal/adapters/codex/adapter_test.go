package codex_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/adapters"
	"github.com/xinchenglearning/agent-orchestrator/internal/adapters/codex"
	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/policy"
)

func TestAdapterNormalizesTerminalResult(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	adapter := codex.New(codex.Options{
		Path:       executable,
		PrefixArgs: []string{"-test.run=TestCodexHelper", "--"},
		Env: map[string]string{
			"GO_WANT_CODEX_HELPER": "1",
		},
		ResolveHead: func(context.Context, string) (string, error) {
			return "fedcba9876543210fedcba9876543210fedcba98", nil
		},
	})
	evidenceDir := t.TempDir()
	capabilities, err := adapter.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !capabilities.StructuredOutput || capabilities.NativeSandbox {
		t.Fatalf("unexpected capabilities: %+v", capabilities)
	}

	session, err := adapter.Start(context.Background(), adapters.Job{
		Task: domain.Task{
			ID:           "task-1",
			Objective:    "Fix parser",
			Rationale:    "Restore loading",
			Deliverables: []string{"commit"},
			AllowedPaths: []string{"parser.go"},
			Acceptance: []domain.CommandSpec{{
				Argv:           []string{"go", "test", "./..."},
				TimeoutSeconds: 30,
			}},
			Delegation: domain.DelegationExecuteReversible,
			Budget:     domain.Budget{MaxAttempts: 1, MaxSeconds: 60},
		},
		TaskDigest:    "digest",
		Role:          "writer",
		ExecutionMode: policy.ExecutionTrustedLocal,
		WorkspaceDir:  t.TempDir(),
		EvidenceDir:   evidenceDir,
		Generation:    3,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.Wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionID != "fixture-session" ||
		result.ResultCommit != "fedcba9876543210fedcba9876543210fedcba98" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(evidenceDir, "codex-last-message.json")); !os.IsNotExist(err) {
		t.Fatalf("raw model output persisted in evidence directory: %v", err)
	}
}

func TestDefaultPassesCodexAPIKeyToCLI(t *testing.T) {
	binDir := t.TempDir()
	executable := filepath.Join(binDir, "codex")
	if err := os.WriteFile(
		executable,
		[]byte("#!/bin/sh\n[ \"$CODEX_API_KEY\" = \"fixture-key\" ]\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("CODEX_API_KEY", "fixture-key")

	if _, err := codex.Default().Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCodexHelper(t *testing.T) {
	if os.Getenv("GO_WANT_CODEX_HELPER") != "1" {
		return
	}
	for _, arg := range os.Args {
		if arg == "--version" {
			fmt.Println("codex-cli fixture")
			os.Exit(0)
		}
	}
	var output string
	for i, arg := range os.Args {
		if arg == "--output-last-message" && i+1 < len(os.Args) {
			output = os.Args[i+1]
			break
		}
	}
	if output == "" {
		os.Exit(2)
	}
	prompt := os.Args[len(os.Args)-1]
	if !strings.Contains(prompt, "Work exclusively in the current working directory") {
		os.Exit(3)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		os.Exit(2)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(
		output,
		[]byte(`{"sessionId":"fixture-session","resultCommit":"0123456789abcdef0123456789abcdef01234567"}`),
		0o600,
	); err != nil {
		os.Exit(2)
	}
	fmt.Println(`{"type":"turn.completed"}`)
	os.Exit(0)
}
