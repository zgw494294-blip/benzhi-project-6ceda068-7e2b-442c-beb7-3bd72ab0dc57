package archive

import (
	"context"
	"encoding/json"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func (s *Service) ProposeResolution(ctx context.Context, taskID string, command ProposeResolutionCommand) (ResolutionResult, error) {
	if err := validateMeta(command.CommandMeta, RoleAdministrator); err != nil {
		return ResolutionResult{}, err
	}
	operationCtx := context.WithoutCancel(ctx)
	operation := "propose-resolution:" + taskID
	if previous, found, err := s.replay(operationCtx, operation, taskID, command.IdempotencyKey); err != nil {
		return ResolutionResult{}, err
	} else if found {
		var stored struct {
			FindingID string `json:"findingId"`
		}
		_ = json.Unmarshal(previous.Result, &stored)
		return ResolutionResult{Task: previous.Aggregate.Task, Finding: previous.Aggregate.Findings[stored.FindingID], Replay: true}, nil
	}
	// The idempotency query has completed; if the client has since disconnected
	// or timed out, surface an identifiable context.Canceled and avoid any
	// version increment, state change, audit record or other persisted effect.
	if err := canceled(ctx); err != nil {
		return ResolutionResult{}, err
	}
	aggregate, err := s.repository.Get(operationCtx, taskID)
	if err != nil {
		return ResolutionResult{}, err
	}
	finding, err := observatory.ProposeResolution(aggregate, command.FindingID, command.ReplacementRevisionID, command.ResolutionNote)
	if err != nil {
		return ResolutionResult{}, err
	}
	aggregate.Findings[finding.ID] = finding
	// Re-check cancellation immediately before persisting so a disconnect
	// arriving between the query and the commit still produces no side effects.
	if err := canceled(ctx); err != nil {
		return ResolutionResult{}, err
	}
	result, err := s.commit(operationCtx, operation, command.CommandMeta, aggregate, "RESOLUTION_PROPOSED", map[string]string{"findingId": finding.ID})
	if err != nil {
		return ResolutionResult{}, err
	}
	if result.Replay {
		var stored struct {
			FindingID string `json:"findingId"`
		}
		if json.Unmarshal(result.Result, &stored) == nil {
			finding = result.Aggregate.Findings[stored.FindingID]
		}
	}
	return ResolutionResult{Task: result.Aggregate.Task, Finding: finding, Replay: result.Replay}, nil
}

func (s *Service) ReviewResolution(ctx context.Context, taskID string, command ReviewResolutionCommand) (ResolutionResult, error) {
	if err := validateMeta(command.CommandMeta, RoleReviewer); err != nil {
		return ResolutionResult{}, err
	}
	operationCtx := context.WithoutCancel(ctx)
	operation := "review-resolution:" + taskID
	if previous, found, err := s.replay(operationCtx, operation, taskID, command.IdempotencyKey); err != nil {
		return ResolutionResult{}, err
	} else if found {
		var stored struct {
			FindingID string `json:"findingId"`
		}
		_ = json.Unmarshal(previous.Result, &stored)
		return ResolutionResult{Task: previous.Aggregate.Task, Finding: previous.Aggregate.Findings[stored.FindingID], Replay: true}, nil
	}
	// The idempotency query has completed; if the client has since disconnected
	// or timed out, surface an identifiable context.Canceled and avoid any
	// version increment, state change, audit record or other persisted effect.
	if err := canceled(ctx); err != nil {
		return ResolutionResult{}, err
	}
	aggregate, err := s.repository.Get(operationCtx, taskID)
	if err != nil {
		return ResolutionResult{}, err
	}
	finding, err := observatory.ReviewResolution(aggregate, command.FindingID, command.Actor, command.Accepted, command.ReviewNote, s.now())
	if err != nil {
		return ResolutionResult{}, err
	}
	aggregate.Findings[finding.ID] = finding
	if command.Accepted && !observatory.HasOpenBlockingFindings(aggregate) {
		aggregate.Task.State = observatory.StateReviewPending
	}
	// Re-check cancellation immediately before persisting so a disconnect
	// arriving between the query and the commit still produces no side effects.
	if err := canceled(ctx); err != nil {
		return ResolutionResult{}, err
	}
	result, err := s.commit(operationCtx, operation, command.CommandMeta, aggregate, "RESOLUTION_REVIEWED", map[string]string{"findingId": finding.ID})
	if err != nil {
		return ResolutionResult{}, err
	}
	if result.Replay {
		var stored struct {
			FindingID string `json:"findingId"`
		}
		if json.Unmarshal(result.Result, &stored) == nil {
			finding = result.Aggregate.Findings[stored.FindingID]
		}
	}
	return ResolutionResult{Task: result.Aggregate.Task, Finding: finding, Replay: result.Replay}, nil
}
