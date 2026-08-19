//go:build real_agents

package codex_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/adapters"
	"github.com/xinchenglearning/agent-orchestrator/internal/adapters/codex"
	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/policy"
)

func TestCodexContractCompletesRealRepositoryTask(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex CLI is not installed")
	}
	repo, base := realFixtureRepository(t)
	worktree := filepath.Join(t.TempDir(), "writer")
	realGit(t, repo, "worktree", "add", "--detach", worktree, base)

	task := domain.Task{
		ID: "codex-contract",
		Repository: domain.RepositorySpec{
			Root:    repo,
			BaseRef: base,
		},
		Objective:    "Change README.md content from before to after",
		Rationale:    "Prove the real writer adapter",
		Deliverables: []string{"one Git commit"},
		AllowedPaths: []string{"README.md"},
		Constraints:  []string{"change no other file"},
		Acceptance: []domain.CommandSpec{{
			Argv:           []string{"/usr/bin/grep", "-q", "after", "README.md"},
			TimeoutSeconds: 10,
		}},
		Delegation: domain.DelegationExecuteReversible,
		Budget:     domain.Budget{MaxAttempts: 1, MaxSeconds: 180},
	}
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	adapter := codex.Default()
	capabilities, err := adapter.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capabilities.NativeSandbox {
		t.Fatal("native sandbox must remain false until a canary contract proves it")
	}

	session, err := adapter.Start(context.Background(), adapters.Job{
		Task:          task,
		TaskDigest:    digest,
		Role:          "writer",
		ExecutionMode: policy.ExecutionTrustedLocal,
		WorkspaceDir:  worktree,
		EvidenceDir:   filepath.Join(t.TempDir(), "evidence"),
		Budget:        task.Budget,
		Generation:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	result, err := session.Wait(waitCtx)
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(realGit(t, worktree, "rev-parse", "HEAD"))
	if result.ResultCommit != head || head == base {
		t.Fatalf("result commit=%s head=%s base=%s", result.ResultCommit, head, base)
	}
	data, err := os.ReadFile(filepath.Join(worktree, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "after") {
		t.Fatalf("README was not changed: %q", data)
	}
}

func realFixtureRepository(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	realGit(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	realGit(t, repo, "add", "README.md")
	realGit(t, repo, "commit", "-m", "initial")
	return repo, strings.TrimSpace(realGit(t, repo, "rev-parse", "HEAD"))
}

func realGit(t *testing.T, dir string, args ...string) string {
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
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
