package archive

import (
	"context"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func (s *Service) GetTask(ctx context.Context, taskID string) (TaskDetail, error) {
	aggregate, err := s.repository.Get(ctx, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	return s.detailFromAggregate(aggregate), nil
}

func (s *Service) ListTasks(ctx context.Context) ([]observatory.ArchiveTask, error) {
	aggregates, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	tasks := make([]observatory.ArchiveTask, len(aggregates))
	for index, aggregate := range aggregates {
		tasks[index] = aggregate.Task
	}
	return tasks, nil
}

func (s *Service) Timeline(ctx context.Context, taskID string) (TimelineResult, error) {
	events, err := s.repository.Timeline(ctx, taskID)
	if err != nil {
		return TimelineResult{}, err
	}
	return TimelineResult{TaskID: taskID, Events: events}, nil
}

func (s *Service) Verify(ctx context.Context, taskID string) (observatory.VerificationResult, error) {
	aggregate, err := s.repository.Get(ctx, taskID)
	if err != nil {
		return observatory.VerificationResult{}, err
	}
	return observatory.VerifyRelease(aggregate), nil
}
