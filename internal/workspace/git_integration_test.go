package workspace_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/workspace"
)

func TestGitWorkspaceCreatesIndependentWorktreesAndProtectsDirtyWork(t *testing.T) {
	repo, base := fixtureRepository(t)
	ctx := context.Background()

	resolved, err := workspace.Resolve(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Root != canonicalRepo || resolved.Head != base {
		t.Fatalf("unexpected repository: %+v", resolved)
	}
	resolvedBase, err := workspace.ResolveCommit(ctx, repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if resolvedBase != base {
		t.Fatalf("resolved HEAD %s, want %s", resolvedBase, base)
	}

	writer := filepath.Join(t.TempDir(), "writer")
	verifier := filepath.Join(t.TempDir(), "verifier")
	if err := workspace.Add(ctx, resolved.Root, writer, base); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Add(ctx, resolved.Root, verifier, base); err != nil {
		t.Fatal(err)
	}
	if writer == verifier {
		t.Fatal("writer and verifier paths must differ")
	}

	if err := os.WriteFile(filepath.Join(writer, "dirty.txt"), []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Remove(ctx, resolved.Root, writer); !errors.Is(err, workspace.ErrDirty) {
		t.Fatalf("expected dirty worktree protection, got %v", err)
	}
	if err := workspace.Remove(ctx, resolved.Root, verifier); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAllowedPathsRejectsOutOfScopeChanges(t *testing.T) {
	if err := workspace.ValidateAllowedPaths(
		[]string{"parser/reader.go", "parser_test.go"},
		[]string{"parser/", "parser_test.go"},
	); err != nil {
		t.Fatal(err)
	}
	if err := workspace.ValidateAllowedPaths(
		[]string{"parser/reader.go", "go.mod"},
		[]string{"parser/"},
	); err == nil {
		t.Fatal("expected out-of-scope path rejection")
	}
}

func TestGitWorkspaceReportsHeadChangedPathsAndDiff(t *testing.T) {
	repo, base := fixtureRepository(t)
	worktree := filepath.Join(t.TempDir(), "writer")
	ctx := context.Background()
	if err := workspace.Add(ctx, repo, worktree, base); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "README.md")
	runGit(t, worktree, "commit", "-m", "change")

	head, err := workspace.Head(ctx, worktree)
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.ValidateResultCommit(ctx, worktree, base, head); err != nil {
		t.Fatal(err)
	}
	changed, err := workspace.ChangedPaths(ctx, worktree, base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0] != "README.md" {
		t.Fatalf("unexpected changed paths: %v", changed)
	}
	diff, err := workspace.Diff(ctx, worktree, base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(diff), "+changed") {
		t.Fatalf("unexpected diff: %s", diff)
	}

	if err := os.WriteFile(filepath.Join(worktree, "SECOND.md"), []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "SECOND.md")
	runGit(t, worktree, "commit", "-m", "second")
	secondHead, err := workspace.Head(ctx, worktree)
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.ValidateResultCommit(ctx, worktree, base, secondHead); err == nil {
		t.Fatal("expected multi-commit result rejection")
	}
}

func fixtureRepository(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	return repo, strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
}

func runGit(t *testing.T, dir string, args ...string) string {
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
