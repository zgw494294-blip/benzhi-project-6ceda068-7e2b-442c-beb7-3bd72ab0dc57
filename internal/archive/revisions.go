package archive

import (
	"context"
	"encoding/json"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func (s *Service) RegisterRevision(ctx context.Context, taskID string, command RegisterRevisionCommand) (RevisionResult, error) {
	if err := validateMeta(command.CommandMeta, RoleAdministrator); err != nil {
		return RevisionResult{}, err
	}
	operation := "register-revision:" + taskID
	if previous, found, err := s.replay(ctx, operation, taskID, command.IdempotencyKey); err != nil {
		return RevisionResult{}, err
	} else if found {
		var stored struct {
			RevisionID string `json:"revisionId"`
		}
		_ = json.Unmarshal(previous.Result, &stored)
		return RevisionResult{Task: previous.Aggregate.Task, Revision: previous.Aggregate.Revisions[stored.RevisionID], Replay: true}, nil
	}
	aggregate, err := s.repository.Get(ctx, taskID)
	if err != nil {
		return RevisionResult{}, err
	}
	revision, err := observatory.RegisterRevision(aggregate, observatory.NewRevision{
		ID: s.ids.New("rev"), LogicalPath: command.LogicalPath, ByteSize: command.ByteSize,
		MediaType: command.MediaType, SHA256: command.SHA256,
		SupersedesRevisionID: command.SupersedesRevisionID,
		SubmittedBy:          command.Actor, Now: s.now(),
	})
	if err != nil {
		return RevisionResult{}, err
	}
	aggregate.Revisions[revision.ID] = revision
	if aggregate.Task.State == observatory.StateDraft {
		aggregate.Task.State = observatory.StateCollecting
	}
	result, err := s.commit(ctx, operation, command.CommandMeta, aggregate, "DATASET_REVISION_REGISTERED", map[string]string{"revisionId": revision.ID})
	if err != nil {
		return RevisionResult{}, err
	}
	if result.Replay {
		var stored struct {
			RevisionID string `json:"revisionId"`
		}
		if json.Unmarshal(result.Result, &stored) == nil {
			revision = result.Aggregate.Revisions[stored.RevisionID]
		}
	}
	return RevisionResult{Task: result.Aggregate.Task, Revision: revision, Replay: result.Replay}, nil
}
