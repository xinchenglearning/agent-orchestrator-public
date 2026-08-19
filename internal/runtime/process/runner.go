package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"syscall"
	"time"
)

type Spec struct {
	Path    string
	Args    []string
	Dir     string
	Env     map[string]string
	Timeout time.Duration
}

type Result struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
	TimedOut   bool
}

func Run(ctx context.Context, spec Spec) (Result, error) {
	if spec.Path == "" {
		return Result{}, errors.New("process path is required")
	}
	runCtx := ctx
	cancel := func() {}
	if spec.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, spec.Timeout)
	}
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = environment(spec.Env)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	result := Result{ExitCode: -1, StartedAt: time.Now()}
	if err := cmd.Start(); err != nil {
		result.FinishedAt = time.Now()
		return result, fmt.Errorf("start process: %w", err)
	}

	wait := make(chan error, 1)
	go func() {
		wait <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-wait:
	case <-runCtx.Done():
		killErr := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if killErr != nil && !errors.Is(killErr, syscall.ESRCH) {
			return result, fmt.Errorf("terminate process group: %w", killErr)
		}
		waitErr = <-wait
		result.TimedOut = errors.Is(runCtx.Err(), context.DeadlineExceeded)
	}

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.FinishedAt = time.Now()
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runCtx.Err() != nil {
		return result, runCtx.Err()
	}
	if waitErr != nil {
		return result, fmt.Errorf("wait for process: %w", waitErr)
	}
	return result, nil
}

func environment(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}
