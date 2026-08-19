package adapters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/adapters"
)

func TestParseStructuredResultAcceptsRawAndFencedJSON(t *testing.T) {
	tests := []string{
		`{"sessionId":"session-1","resultCommit":"0123456789abcdef0123456789abcdef01234567"}`,
		"result:\n```json\n" +
			`{"sessionId":"session-1","resultCommit":"0123456789abcdef0123456789abcdef01234567"}` +
			"\n```",
	}
	for _, input := range tests {
		result, err := adapters.ParseStructuredResult(input)
		if err != nil {
			t.Fatal(err)
		}
		if result.SessionID != "session-1" {
			t.Fatalf("unexpected result: %+v", result)
		}
	}
}

func TestParseStructuredResultRejectsUnknownOrMissingFields(t *testing.T) {
	for _, input := range []string{
		`{"sessionId":"session-1","resultCommit":"","extra":true}`,
		`{"sessionId":"session-1"}`,
		`not json`,
	} {
		if _, err := adapters.ParseStructuredResult(input); !errors.Is(err, adapters.ErrProtocol) {
			t.Fatalf("expected protocol error for %q, got %v", input, err)
		}
	}
}

func TestParseStructuredResultRetriesRepairOnce(t *testing.T) {
	attempts := 0
	result, err := adapters.ParseStructuredResultWithRepair(
		context.Background(),
		`invalid`,
		func(context.Context, string) (string, error) {
			attempts++
			return `{"sessionId":"repaired","resultCommit":"0123456789abcdef0123456789abcdef01234567"}`, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || result.SessionID != "repaired" {
		t.Fatalf("attempts=%d result=%+v", attempts, result)
	}
}
