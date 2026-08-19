package verification_test

import (
	"context"
	"os"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/verification"
)

func TestVerifierRunsOnlyDeclaredArgv(t *testing.T) {
	output := t.TempDir() + "/ran"
	commands := []domain.CommandSpec{{
		Argv:           []string{"/usr/bin/touch", output},
		TimeoutSeconds: 10,
	}}

	results, err := verification.Run(context.Background(), t.TempDir(), commands, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("unexpected results: %+v", results)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
}

func TestVerifierStopsOnFailedCommand(t *testing.T) {
	commands := []domain.CommandSpec{
		{Argv: []string{"/usr/bin/false"}, TimeoutSeconds: 10},
		{Argv: []string{"/usr/bin/true"}, TimeoutSeconds: 10},
	}
	results, err := verification.Run(context.Background(), t.TempDir(), commands, nil)
	if err == nil {
		t.Fatal("expected verification failure")
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
}
