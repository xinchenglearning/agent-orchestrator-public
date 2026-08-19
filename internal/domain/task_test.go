package domain_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
)

func validTask(t *testing.T) domain.Task {
	t.Helper()

	return domain.Task{
		ID: "task-1",
		Repository: domain.RepositorySpec{
			Root:    t.TempDir(),
			BaseRef: "0123456789abcdef0123456789abcdef01234567",
		},
		Objective:    "Fix the failing parser test",
		Rationale:    "Restore valid configuration loading",
		Deliverables: []string{"one Git commit", "passing tests"},
		NonGoals:     []string{"redesign the parser"},
		AllowedPaths: []string{"parser/", "parser_test.go"},
		Constraints:  []string{"do not modify public APIs"},
		Acceptance: []domain.CommandSpec{{
			Argv:           []string{"go", "test", "./..."},
			TimeoutSeconds: 120,
		}},
		Delegation: domain.DelegationExecuteReversible,
		Budget: domain.Budget{
			MaxAttempts: 1,
			MaxSeconds:  900,
		},
	}
}

func TestValidateTaskRequiresHumanOutcome(t *testing.T) {
	err := domain.ValidateTask(domain.Task{ID: "task-1"})
	if err == nil {
		t.Fatal("expected invalid task")
	}
}

func TestValidateTaskAcceptsReversibleRepositoryWork(t *testing.T) {
	if err := domain.ValidateTask(validTask(t)); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTaskRejectsInvalidBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.Task)
	}{
		{"invalid ID", func(task *domain.Task) { task.ID = "task 1" }},
		{"relative repository", func(task *domain.Task) { task.Repository.Root = "repo" }},
		{"non commit base ref", func(task *domain.Task) { task.Repository.BaseRef = "main" }},
		{"empty objective", func(task *domain.Task) { task.Objective = "" }},
		{"empty rationale", func(task *domain.Task) { task.Rationale = "" }},
		{"empty deliverables", func(task *domain.Task) { task.Deliverables = nil }},
		{"empty allowed paths", func(task *domain.Task) { task.AllowedPaths = nil }},
		{"absolute allowed path", func(task *domain.Task) { task.AllowedPaths = []string{"/tmp/file"} }},
		{"traversing allowed path", func(task *domain.Task) { task.AllowedPaths = []string{"../file"} }},
		{"empty acceptance", func(task *domain.Task) { task.Acceptance = nil }},
		{"empty acceptance argv", func(task *domain.Task) { task.Acceptance[0].Argv = nil }},
		{"non positive timeout", func(task *domain.Task) { task.Acceptance[0].TimeoutSeconds = 0 }},
		{"excessive timeout", func(task *domain.Task) { task.Acceptance[0].TimeoutSeconds = 86401 }},
		{"unsupported retry", func(task *domain.Task) { task.Budget.MaxAttempts = 2 }},
		{"too many attempts", func(task *domain.Task) { task.Budget.MaxAttempts = 3 }},
		{"non positive budget", func(task *domain.Task) { task.Budget.MaxSeconds = 0 }},
		{"excessive budget", func(task *domain.Task) { task.Budget.MaxSeconds = 86401 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := validTask(t)
			tt.mutate(&task)
			if err := domain.ValidateTask(task); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateTaskRejectsUnsupportedDelegation(t *testing.T) {
	task := validTask(t)
	task.Delegation = domain.DelegationSuggest

	err := domain.ValidateTask(task)
	if !errors.Is(err, domain.ErrUnsupportedDelegation) {
		t.Fatalf("expected unsupported delegation, got %v", err)
	}
}

func TestDigestTaskIsStableAndCoversExecutionBoundaries(t *testing.T) {
	task := validTask(t)
	first, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	second, err := domain.DigestTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("digest is not stable: %q != %q", first, second)
	}

	changed := task
	changed.AllowedPaths = append([]string(nil), task.AllowedPaths...)
	changed.AllowedPaths[0] = filepath.Join("parser", "internal")
	other, err := domain.DigestTask(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first == other {
		t.Fatal("digest did not change with allowed paths")
	}
}
