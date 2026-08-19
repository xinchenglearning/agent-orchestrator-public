package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type DelegationLevel string

const (
	DelegationSuggest           DelegationLevel = "suggest"
	DelegationPrepare           DelegationLevel = "prepare"
	DelegationExecuteReversible DelegationLevel = "execute_reversible"
	DelegationApprovalRequired  DelegationLevel = "approval_required"
)

var ErrUnsupportedDelegation = errors.New("unsupported delegation level")

var (
	taskIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

const maxExecutionSeconds = 24 * 60 * 60

type CommandSpec struct {
	Argv           []string `json:"argv"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

type Budget struct {
	MaxAttempts int `json:"maxAttempts"`
	MaxSeconds  int `json:"maxSeconds"`
}

type RepositorySpec struct {
	Root    string `json:"root"`
	BaseRef string `json:"baseRef"`
}

type Task struct {
	ID           string          `json:"id"`
	Repository   RepositorySpec  `json:"repository"`
	Objective    string          `json:"objective"`
	Rationale    string          `json:"rationale"`
	Deliverables []string        `json:"deliverables"`
	NonGoals     []string        `json:"nonGoals"`
	AllowedPaths []string        `json:"allowedPaths"`
	Constraints  []string        `json:"constraints"`
	Acceptance   []CommandSpec   `json:"acceptance"`
	Delegation   DelegationLevel `json:"delegation"`
	Budget       Budget          `json:"budget"`
}

func ValidateTask(task Task) error {
	if !taskIDPattern.MatchString(task.ID) {
		return errors.New("task ID must match [a-zA-Z0-9._-]+")
	}
	if !filepath.IsAbs(task.Repository.Root) ||
		filepath.Clean(task.Repository.Root) != task.Repository.Root {
		return errors.New("repository root must be an absolute clean path")
	}
	if !commitPattern.MatchString(task.Repository.BaseRef) {
		return errors.New("repository base ref must be a lowercase 40-character commit")
	}
	if strings.TrimSpace(task.Objective) == "" {
		return errors.New("objective is required")
	}
	if strings.TrimSpace(task.Rationale) == "" {
		return errors.New("rationale is required")
	}
	if err := validateRequiredStrings("deliverables", task.Deliverables); err != nil {
		return err
	}
	if err := validateAllowedPaths(task.AllowedPaths); err != nil {
		return err
	}
	if len(task.Acceptance) == 0 {
		return errors.New("at least one acceptance command is required")
	}
	for i, command := range task.Acceptance {
		if err := validateRequiredStrings(
			fmt.Sprintf("acceptance command %d argv", i),
			command.Argv,
		); err != nil {
			return err
		}
		if command.TimeoutSeconds <= 0 {
			return fmt.Errorf("acceptance command %d timeout must be positive", i)
		}
		if command.TimeoutSeconds > maxExecutionSeconds {
			return fmt.Errorf("acceptance command %d timeout exceeds 24 hours", i)
		}
	}
	switch task.Delegation {
	case DelegationExecuteReversible:
	case DelegationSuggest, DelegationPrepare, DelegationApprovalRequired:
		return fmt.Errorf("%w: %s", ErrUnsupportedDelegation, task.Delegation)
	default:
		return fmt.Errorf("unknown delegation level %q", task.Delegation)
	}
	if task.Budget.MaxAttempts != 1 {
		return errors.New("M1 single strategy requires exactly one attempt")
	}
	if task.Budget.MaxSeconds <= 0 {
		return errors.New("max seconds must be positive")
	}
	if task.Budget.MaxSeconds > maxExecutionSeconds {
		return errors.New("max seconds exceeds 24 hours")
	}
	return nil
}

func DigestTask(task Task) (string, error) {
	if err := ValidateTask(task); err != nil {
		return "", err
	}
	data, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("marshal canonical task: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func validateRequiredStrings(name string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must not be empty", name)
	}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s item %d must not be empty", name, i)
		}
	}
	return nil
}

func validateAllowedPaths(paths []string) error {
	if err := validateRequiredStrings("allowed paths", paths); err != nil {
		return err
	}
	for _, path := range paths {
		clean := filepath.Clean(path)
		if filepath.IsAbs(path) || clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("allowed path %q escapes the repository", path)
		}
	}
	return nil
}
