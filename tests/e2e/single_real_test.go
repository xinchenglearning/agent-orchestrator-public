//go:build real_agents

package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/adapters/codex"
	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/host"
	"github.com/xinchenglearning/agent-orchestrator/internal/orchestrator"
	"github.com/xinchenglearning/agent-orchestrator/internal/policy"
	"github.com/xinchenglearning/agent-orchestrator/internal/redaction"
)

func TestSingleRealTaskReachesHumanApproval(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex CLI is not installed")
	}
	repo := t.TempDir()
	e2eGit(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e2eGit(t, repo, "add", "README.md")
	e2eGit(t, repo, "commit", "-m", "initial")

	task := domain.Task{
		ID: "single-real-e2e",
		Repository: domain.RepositorySpec{
			Root:    repo,
			BaseRef: "HEAD",
		},
		Objective:    "Replace the entire README.md content with exactly one line: after",
		Rationale:    "Prove the complete real-agent task loop",
		Deliverables: []string{"one Git commit", "passing acceptance command"},
		AllowedPaths: []string{"README.md"},
		Constraints:  []string{"change no other file", "README.md must contain exactly after and a newline"},
		Acceptance: []domain.CommandSpec{{
			Argv:           []string{"/usr/bin/grep", "-Fxq", "after", "README.md"},
			TimeoutSeconds: 10,
		}},
		Delegation: domain.DelegationExecuteReversible,
		Budget:     domain.Budget{MaxAttempts: 1, MaxSeconds: 180},
	}
	service := host.Service{Engine: orchestrator.Engine{
		Adapter:  codex.Default(),
		Mode:     policy.ExecutionTrustedLocal,
		Redactor: redaction.New(environmentSecrets()),
	}}
	prepared, digest, err := service.Prepare(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.Run(context.Background(), prepared, digest)
	if err != nil {
		t.Fatal(err)
	}
	if run.State != domain.RunAwaitingApproval {
		t.Fatalf("got state %s", run.State)
	}
	if run.ResultCommit == prepared.Repository.BaseRef {
		t.Fatal("writer did not produce a new commit")
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

	completed, err := service.Decide(
		context.Background(),
		prepared.Repository.Root,
		run.ID,
		domain.Approval{
			Actor:                "e2e-maintainer",
			Decision:             domain.DecisionApprove,
			TaskDigest:           digest,
			DecisionPacketDigest: run.DecisionPacketDigest,
			At:                   time.Now(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != domain.RunCompleted {
		t.Fatalf("got final state %s", completed.State)
	}
}

func environmentSecrets() []string {
	var result []string
	for _, key := range []string{"OPENAI_API_KEY", "CODEX_API_KEY"} {
		if value, ok := os.LookupEnv(key); ok {
			result = append(result, value)
		}
	}
	return result
}

func e2eGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
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
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
