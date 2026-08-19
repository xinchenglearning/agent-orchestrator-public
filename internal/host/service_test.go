package host_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/host"
)

func TestPreparePinsCanonicalRepositoryAndCommit(t *testing.T) {
	repo := t.TempDir()
	hostGit(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hostGit(t, repo, "add", "README.md")
	hostGit(t, repo, "commit", "-m", "initial")
	head := strings.TrimSpace(hostGit(t, repo, "rev-parse", "HEAD"))

	task := domain.Task{
		ID: "prepare-test",
		Repository: domain.RepositorySpec{
			Root:    repo,
			BaseRef: "HEAD",
		},
		Objective:    "Update README",
		Rationale:    "Prove prepare",
		Deliverables: []string{"commit"},
		AllowedPaths: []string{"README.md"},
		Acceptance: []domain.CommandSpec{{
			Argv:           []string{"/usr/bin/true"},
			TimeoutSeconds: 10,
		}},
		Delegation: domain.DelegationExecuteReversible,
		Budget:     domain.Budget{MaxAttempts: 1, MaxSeconds: 60},
	}
	prepared, digest, err := (host.Service{}).Prepare(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Repository.Root != canonical || prepared.Repository.BaseRef != head {
		t.Fatalf("unexpected prepared task: %+v", prepared.Repository)
	}
	if digest == "" {
		t.Fatal("missing task digest")
	}
}

func hostGit(t *testing.T, dir string, args ...string) string {
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
