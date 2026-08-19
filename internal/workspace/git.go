package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	process "github.com/xinchenglearning/agent-orchestrator/internal/runtime/process"
)

var ErrDirty = errors.New("worktree is dirty")

type Repository struct {
	Root      string
	CommonDir string
	Head      string
}

func Resolve(ctx context.Context, repository string) (Repository, error) {
	root, err := gitOutput(ctx, repository, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repository{}, err
	}
	root = filepath.Clean(root)
	commonDir, err := gitOutput(ctx, root, "rev-parse", "--git-common-dir")
	if err != nil {
		return Repository{}, err
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(root, commonDir)
	}
	head, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Repository{}, err
	}
	return Repository{
		Root:      root,
		CommonDir: filepath.Clean(commonDir),
		Head:      head,
	}, nil
}

func Add(ctx context.Context, repository, destination, baseCommit string) error {
	_, err := gitOutput(
		ctx,
		repository,
		"worktree",
		"add",
		"--detach",
		destination,
		baseCommit,
	)
	return err
}

func ResolveCommit(ctx context.Context, repository, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	return gitOutput(ctx, repository, "rev-parse", "--verify", ref+"^{commit}")
}

func Remove(ctx context.Context, repository, worktree string) error {
	status, err := gitOutput(ctx, worktree, "status", "--porcelain")
	if err != nil {
		return err
	}
	if status != "" {
		return ErrDirty
	}
	_, err = gitOutput(ctx, repository, "worktree", "remove", worktree)
	return err
}

func Head(ctx context.Context, worktree string) (string, error) {
	return gitOutput(ctx, worktree, "rev-parse", "HEAD")
}

func ValidateResultCommit(
	ctx context.Context,
	worktree string,
	baseCommit string,
	resultCommit string,
) error {
	line, err := gitOutput(
		ctx,
		worktree,
		"rev-list",
		"--parents",
		"-n",
		"1",
		resultCommit,
	)
	if err != nil {
		return err
	}
	commits := strings.Fields(line)
	if len(commits) != 2 || commits[0] != resultCommit || commits[1] != baseCommit {
		return errors.New("result commit must be a single non-merge commit directly based on base")
	}
	return nil
}

func ChangedPaths(
	ctx context.Context,
	worktree string,
	baseCommit string,
	resultCommit string,
) ([]string, error) {
	output, err := gitRaw(
		ctx,
		worktree,
		"diff",
		"--name-only",
		"-z",
		baseCommit+".."+resultCommit,
	)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(output, "\x00")
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			paths = append(paths, part)
		}
	}
	return paths, nil
}

func Diff(
	ctx context.Context,
	worktree string,
	baseCommit string,
	resultCommit string,
) ([]byte, error) {
	output, err := gitRaw(
		ctx,
		worktree,
		"diff",
		"--binary",
		baseCommit+".."+resultCommit,
	)
	if err != nil {
		return nil, err
	}
	return []byte(output), nil
}

func ValidateAllowedPaths(changed, allowed []string) error {
	for _, path := range changed {
		clean := filepath.ToSlash(filepath.Clean(path))
		permitted := false
		for _, rule := range allowed {
			rule = filepath.ToSlash(rule)
			if strings.HasSuffix(rule, "/") {
				permitted = strings.HasPrefix(clean, rule)
			} else {
				permitted = clean == filepath.ToSlash(filepath.Clean(rule))
			}
			if permitted {
				break
			}
		}
		if !permitted {
			return fmt.Errorf("changed path %q is outside the task allowlist", path)
		}
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	output, err := gitRaw(ctx, dir, args...)
	return strings.TrimSpace(output), err
}

func gitRaw(ctx context.Context, dir string, args ...string) (string, error) {
	result, err := process.Run(ctx, process.Spec{
		Path:    "git",
		Args:    args,
		Dir:     dir,
		Env:     gitEnvironment(),
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, result.Stderr)
	}
	return result.Stdout, nil
}

func gitEnvironment() map[string]string {
	result := map[string]string{
		"LC_ALL": "C",
	}
	for _, key := range []string{"PATH", "HOME", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			result[key] = value
		}
	}
	return result
}
