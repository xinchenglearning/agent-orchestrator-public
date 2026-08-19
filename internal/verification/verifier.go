package verification

import (
	"context"
	"fmt"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	process "github.com/xinchenglearning/agent-orchestrator/internal/runtime/process"
)

type CommandResult struct {
	Argv       []string  `json:"argv"`
	ExitCode   int       `json:"exitCode"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

func Run(
	ctx context.Context,
	workspaceDir string,
	commands []domain.CommandSpec,
	env map[string]string,
) ([]CommandResult, error) {
	results := make([]CommandResult, 0, len(commands))
	for _, command := range commands {
		if len(command.Argv) == 0 {
			return results, fmt.Errorf("verification command argv is empty")
		}
		result, err := process.Run(ctx, process.Spec{
			Path:    command.Argv[0],
			Args:    command.Argv[1:],
			Dir:     workspaceDir,
			Env:     env,
			Timeout: time.Duration(command.TimeoutSeconds) * time.Second,
		})
		results = append(results, CommandResult{
			Argv:       append([]string(nil), command.Argv...),
			ExitCode:   result.ExitCode,
			Stdout:     result.Stdout,
			Stderr:     result.Stderr,
			StartedAt:  result.StartedAt,
			FinishedAt: result.FinishedAt,
		})
		if err != nil {
			return results, fmt.Errorf("verification command %q failed: %w", command.Argv[0], err)
		}
	}
	return results, nil
}
