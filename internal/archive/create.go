package archive

import (
	"context"
	"strings"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func (s *Service) CreateTask(ctx context.Context, command CreateTaskCommand) (MutationResult, error) {
	meta := CommandMeta{
		IdempotencyKey: command.IdempotencyKey, ExpectedVersion: command.ExpectedVersion, Actor: command.Actor,
		Role: command.Role, Reason: command.Reason, CorrelationID: command.CorrelationID,
	}
	if err := validateMeta(meta, RoleAdministrator); err != nil {
		return MutationResult{}, err
	}
	if command.ExpectedVersion != 0 {
		return MutationResult{}, fail("INVALID_EXPECTED_VERSION", "创建任务的 expectedVersion 必须为 0")
	}
	id := stableTaskID(strings.TrimSpace(command.IdempotencyKey))
	aggregate, err := observatory.CreateTask(observatory.NewTask{
		ID: id, Title: command.Title, InstrumentCode: command.InstrumentCode,
		ObservationStart: command.ObservationStart, ObservationEnd: command.ObservationEnd,
		Owner: command.Owner, Now: s.now(),
	})
	if err != nil {
		return MutationResult{}, err
	}
	result, err := s.commit(ctx, "create-task", meta, aggregate, "ARCHIVE_TASK_CREATED", map[string]string{"taskId": id})
	if err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Task: result.Aggregate.Task, Replay: result.Replay}, nil
}
