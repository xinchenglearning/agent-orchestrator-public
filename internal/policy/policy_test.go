package policy_test

import (
	"errors"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/policy"
)

func TestPolicyFiltersEnvironmentByBothAllowlists(t *testing.T) {
	p := policy.Policy{
		AllowedEnv: map[string]struct{}{
			"PATH":      {},
			"CODEX_KEY": {},
		},
	}
	got := p.FilterEnv(
		[]string{"PATH", "CODEX_KEY", "HOME"},
		map[string]string{
			"PATH":      "/bin",
			"CODEX_KEY": "secret",
			"HOME":      "/tmp/home",
		},
	)
	if len(got) != 2 || got["PATH"] != "/bin" || got["CODEX_KEY"] != "secret" {
		t.Fatalf("unexpected filtered environment: %#v", got)
	}
}

func TestPolicyDeniesProductionActionsAndMultipleWriters(t *testing.T) {
	p := policy.Policy{
		AllowWriteRole: "writer",
		DeniedActions: map[string]struct{}{
			"push":   {},
			"merge":  {},
			"deploy": {},
		},
	}
	if err := p.ValidateAction("push"); err == nil {
		t.Fatal("expected push denial")
	}
	if err := p.ValidateWriteRole("reviewer"); err == nil {
		t.Fatal("expected reviewer write denial")
	}
	if err := p.ValidateWriteRole("writer"); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyFailsStrictModeClosedWithoutNativeSandbox(t *testing.T) {
	p := policy.Policy{}
	if err := p.ValidateExecution(
		policy.ExecutionTrustedLocal,
		policy.Capabilities{},
	); !errors.Is(err, policy.ErrWorkspaceWriteRequired) {
		t.Fatalf("expected workspace write error, got %v", err)
	}
	err := p.ValidateExecution(
		policy.ExecutionStrict,
		policy.Capabilities{WorkspaceWrite: true},
	)
	if !errors.Is(err, policy.ErrNativeSandboxRequired) {
		t.Fatalf("expected native sandbox error, got %v", err)
	}
	if err := p.ValidateExecution(
		policy.ExecutionTrustedLocal,
		policy.Capabilities{WorkspaceWrite: true},
	); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyRejectsDeniedVerificationCommands(t *testing.T) {
	p := policy.Policy{DeniedActions: map[string]struct{}{
		"push":   {},
		"merge":  {},
		"deploy": {},
	}}
	for _, argv := range [][]string{
		{"/usr/bin/git", "push", "origin", "main"},
		{"git", "merge", "feature"},
		{"deploy", "production"},
		{"sh", "-c", "git push origin main"},
		{"bash", "-lc", "deploy production"},
		{"python", "-c", "import os; os.system('git push origin main')"},
		{"node", "-e", "require('child_process').execSync('git push origin main')"},
		{"node", "--eval=require('child_process').execSync('git push origin main')"},
		{"env", "sh", "-c", "git push origin main"},
	} {
		if err := p.ValidateCommand(argv); err == nil {
			t.Fatalf("expected command denial for %v", argv)
		}
	}
	if err := p.ValidateCommand([]string{"go", "test", "./..."}); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyBindsAcceptanceToApprovedTaskDigest(t *testing.T) {
	task := domain.Task{
		ID: "task-1",
		Repository: domain.RepositorySpec{
			Root:    t.TempDir(),
			BaseRef: "0123456789abcdef0123456789abcdef01234567",
		},
		Objective:    "Fix parser",
		Rationale:    "Restore loading",
		Deliverables: []string{"commit"},
		AllowedPaths: []string{"parser.go"},
		Acceptance: []domain.CommandSpec{{
			Argv:           []string{"go", "test", "./..."},
			TimeoutSeconds: 30,
		}},
		Delegation: domain.DelegationExecuteReversible,
		Budget:     domain.Budget{MaxAttempts: 1, MaxSeconds: 60},
	}
	digest, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.ValidateApprovedTask(task, digest); err != nil {
		t.Fatal(err)
	}
	task.Acceptance[0].Argv = []string{"go", "test", "./internal/..."}
	if err := policy.ValidateApprovedTask(task, digest); err == nil {
		t.Fatal("expected changed acceptance command rejection")
	}
}
