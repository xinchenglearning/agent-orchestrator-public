package policy

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
)

type ExecutionMode string

const (
	ExecutionTrustedLocal ExecutionMode = "trusted-local"
	ExecutionStrict       ExecutionMode = "strict"
)

var (
	ErrNativeSandboxRequired  = errors.New("strict mode requires a native sandbox")
	ErrWorkspaceWriteRequired = errors.New("writer requires workspace write capability")
)

type Capabilities struct {
	WorkspaceWrite bool
	NativeSandbox  bool
}

type Policy struct {
	AllowedEnv     map[string]struct{}
	AllowWriteRole string
	DeniedActions  map[string]struct{}
}

func (p Policy) FilterEnv(requested []string, source map[string]string) map[string]string {
	result := make(map[string]string)
	for _, key := range requested {
		if _, allowed := p.AllowedEnv[key]; !allowed {
			continue
		}
		if value, ok := source[key]; ok {
			result[key] = value
		}
	}
	return result
}

func (p Policy) ValidateAction(action string) error {
	if _, denied := p.DeniedActions[action]; denied {
		return fmt.Errorf("action %q is denied", action)
	}
	return nil
}

func (p Policy) ValidateCommand(argv []string) error {
	if len(argv) == 0 {
		return errors.New("command argv is required")
	}
	command := filepath.Base(argv[0])
	switch command {
	case "env", "sh", "bash", "dash", "fish", "zsh":
		return fmt.Errorf("command wrapper %q is denied", command)
	case "node", "perl", "python", "python3", "ruby":
		for _, argument := range argv[1:] {
			switch argument {
			case "-c", "-e", "--eval":
				return fmt.Errorf("inline code through %q is denied", command)
			}
			if strings.HasPrefix(argument, "--eval=") {
				return fmt.Errorf("inline code through %q is denied", command)
			}
		}
	}
	if err := p.ValidateAction(command); err != nil {
		return err
	}
	if command != "git" {
		return nil
	}
	for _, argument := range argv[1:] {
		if err := p.ValidateAction(argument); err != nil {
			return err
		}
	}
	return nil
}

func (p Policy) ValidateWriteRole(role string) error {
	if role == "" || role != p.AllowWriteRole {
		return fmt.Errorf("role %q has no write capability", role)
	}
	return nil
}

func (p Policy) ValidateExecution(mode ExecutionMode, capabilities Capabilities) error {
	if !capabilities.WorkspaceWrite {
		return ErrWorkspaceWriteRequired
	}
	switch mode {
	case ExecutionTrustedLocal:
		return nil
	case ExecutionStrict:
		if !capabilities.NativeSandbox {
			return ErrNativeSandboxRequired
		}
		return nil
	default:
		return fmt.Errorf("unknown execution mode %q", mode)
	}
}

func ValidateApprovedTask(task domain.Task, approvedDigest string) error {
	actual, err := domain.DigestTask(task)
	if err != nil {
		return err
	}
	if approvedDigest == "" || actual != approvedDigest {
		return errors.New("task does not match the approved digest")
	}
	return nil
}
