package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
)

func TestPrepareCommandPrintsPinnedTaskAndDigest(t *testing.T) {
	repo := t.TempDir()
	commandGit(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commandGit(t, repo, "add", "README.md")
	commandGit(t, repo, "commit", "-m", "initial")

	task := domain.Task{
		ID: "cli-prepare",
		Repository: domain.RepositorySpec{
			Root:    repo,
			BaseRef: "HEAD",
		},
		Objective:    "Update README",
		Rationale:    "Prove CLI",
		Deliverables: []string{"commit"},
		AllowedPaths: []string{"README.md"},
		Acceptance: []domain.CommandSpec{{
			Argv:           []string{"/usr/bin/true"},
			TimeoutSeconds: 10,
		}},
		Delegation: domain.DelegationExecuteReversible,
		Budget:     domain.Budget{MaxAttempts: 1, MaxSeconds: 60},
	}
	taskPath := filepath.Join(t.TempDir(), "task.json")
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(taskPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := execute(
		[]string{"prepare", "--task", taskPath},
		&stdout,
		&stderr,
	); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var output struct {
		Task   domain.Task `json:"task"`
		Digest string      `json:"digest"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Digest == "" || output.Task.Repository.BaseRef == "HEAD" {
		t.Fatalf("unexpected output: %+v", output)
	}
}

func commandGit(t *testing.T, dir string, args ...string) string {
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
