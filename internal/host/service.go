package host

import (
	"context"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/orchestrator"
	"github.com/xinchenglearning/agent-orchestrator/internal/workspace"
)

type Service struct {
	Engine orchestrator.Engine
}

func (s Service) Prepare(
	ctx context.Context,
	task domain.Task,
) (domain.Task, string, error) {
	repository, err := workspace.Resolve(ctx, task.Repository.Root)
	if err != nil {
		return domain.Task{}, "", err
	}
	baseCommit, err := workspace.ResolveCommit(
		ctx,
		repository.Root,
		task.Repository.BaseRef,
	)
	if err != nil {
		return domain.Task{}, "", err
	}
	task.Repository.Root = repository.Root
	task.Repository.BaseRef = baseCommit
	digest, err := domain.DigestTask(task)
	if err != nil {
		return domain.Task{}, "", err
	}
	return task, digest, nil
}

func (s Service) Run(
	ctx context.Context,
	task domain.Task,
	approvedTaskDigest string,
) (orchestrator.Run, error) {
	return s.Engine.Run(ctx, task, approvedTaskDigest)
}

func (s Service) Status(
	ctx context.Context,
	repositoryRoot string,
	runID string,
) (orchestrator.Run, error) {
	return s.Engine.Status(ctx, repositoryRoot, runID)
}

func (s Service) Decide(
	ctx context.Context,
	repositoryRoot string,
	runID string,
	approval domain.Approval,
) (orchestrator.Run, error) {
	return s.Engine.Decide(ctx, repositoryRoot, runID, approval)
}
