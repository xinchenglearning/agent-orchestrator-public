package redaction

import (
	"regexp"
	"sort"
	"strings"
)

const replacement = "[REDACTED]"

var (
	bearerPattern = regexp.MustCompile(
		`(?i)(authorization\s*:\s*bearer\s+)[^\s]+`,
	)
	credentialPattern = regexp.MustCompile(
		`(?i)((?:api[_-]?key|token|secret)\s*[:=]\s*)[^\s]+`,
	)
	privateKeyPattern = regexp.MustCompile(
		`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`,
	)
)

type Redactor struct {
	secrets []string
}

func New(secrets []string) Redactor {
	filtered := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if secret != "" {
			filtered = append(filtered, secret)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return len(filtered[i]) > len(filtered[j])
	})
	return Redactor{secrets: filtered}
}

func (r Redactor) Redact(input string) string {
	output := input
	for _, secret := range r.secrets {
		output = strings.ReplaceAll(output, secret, replacement)
	}
	output = bearerPattern.ReplaceAllString(output, "${1}"+replacement)
	output = credentialPattern.ReplaceAllString(output, "${1}"+replacement)
	output = privateKeyPattern.ReplaceAllString(output, replacement)
	return output
}
