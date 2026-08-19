package process_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	process "github.com/xinchenglearning/agent-orchestrator/internal/runtime/process"
)

func TestRunnerUsesExplicitEnvironment(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := process.Run(context.Background(), process.Spec{
		Path: executable,
		Args: []string{"-test.run=TestHelperProcess", "--", "print-env"},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "1",
			"ALLOWED_VALUE":          "visible",
		},
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit %d: %s", result.ExitCode, result.Stderr)
	}
	if strings.TrimSpace(result.Stdout) != "visible|" {
		t.Fatalf("unexpected environment: %q", result.Stdout)
	}
}

func TestRunnerCancelsTimedOutProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	result, err := process.Run(context.Background(), process.Spec{
		Path: executable,
		Args: []string{"-test.run=TestHelperProcess", "--", "sleep"},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "1",
		},
		Timeout: 50 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !result.TimedOut {
		t.Fatalf("expected timed out result: %+v", result)
	}
	if time.Since(start) > time.Second {
		t.Fatal("timed out process was not terminated promptly")
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(2)
	}
	switch args[1] {
	case "print-env":
		fmt.Printf("%s|%s", os.Getenv("ALLOWED_VALUE"), os.Getenv("BLOCKED_VALUE"))
	case "sleep":
		time.Sleep(10 * time.Second)
	default:
		os.Exit(2)
	}
	os.Exit(0)
}
