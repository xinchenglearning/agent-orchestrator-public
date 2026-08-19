package redaction_test

import (
	"strings"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/redaction"
)

func TestRedactorRemovesConfiguredAndCommonSecrets(t *testing.T) {
	r := redaction.New([]string{"exact-secret"})
	input := strings.Join([]string{
		"token=exact-secret",
		"Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
		"-----BEGIN PRIVATE KEY-----\nprivate\n-----END PRIVATE KEY-----",
	}, "\n")
	output := r.Redact(input)

	for _, secret := range []string{
		"exact-secret",
		"abcdefghijklmnopqrstuvwxyz",
		"private",
	} {
		if strings.Contains(output, secret) {
			t.Fatalf("secret remains in output: %q", secret)
		}
	}
}
