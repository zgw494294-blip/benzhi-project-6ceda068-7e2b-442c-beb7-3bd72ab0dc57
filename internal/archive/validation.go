package archive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func (s *Service) Validate(ctx context.Context, taskID string, command ValidateCommand) (ValidationResult, error) {
	if err := validateMeta(command.CommandMeta, RoleAdministrator, RoleReviewer); err != nil {
		return ValidationResult{}, err
	}
	operationCtx := context.WithoutCancel(ctx)
	operation := "validate:" + taskID
	if previous, found, err := s.replay(operationCtx, operation, taskID, command.IdempotencyKey); err != nil {
		return ValidationResult{}, err
	} else if found {
		var stored struct {
			FindingIDs []string `json:"findingIds"`
		}
		_ = json.Unmarshal(previous.Result, &stored)
		findings := make([]observatory.ValidationFinding, 0, len(stored.FindingIDs))
		for _, id := range stored.FindingIDs {
			findings = append(findings, previous.Aggregate.Findings[id])
		}
		return ValidationResult{Task: previous.Aggregate.Task, Findings: findings, Replay: true}, nil
	}
	// The idempotency query has completed; if the client has since disconnected
	// or timed out, surface an identifiable context.Canceled and avoid any
	// version increment, state change, audit record or other persisted effect.
	if err := canceled(ctx); err != nil {
		return ValidationResult{}, err
	}
	aggregate, err := s.repository.Get(operationCtx, taskID)
	if err != nil {
		return ValidationResult{}, err
	}
	factory := func(ruleCode, revisionID string) string {
		sum := sha256.Sum256([]byte("finding-v1\n" + taskID + "\n" + revisionID + "\n" + ruleCode))
		return "finding_" + hex.EncodeToString(sum[:12])
	}
	findings, err := observatory.ValidateCandidates(aggregate, factory)
	if err != nil {
		return ValidationResult{}, err
	}
	aggregate.Findings = make(map[string]observatory.ValidationFinding, len(findings))
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		aggregate.Findings[finding.ID] = finding
		ids = append(ids, finding.ID)
	}
	if observatory.FindingsBlock(findings) {
		aggregate.Task.State = observatory.StateQuarantined
	} else {
		aggregate.Task.State = observatory.StateReviewPending
	}
	// Re-check cancellation immediately before persisting so a disconnect
	// arriving between the query and the commit still produces no side effects.
	if err := canceled(ctx); err != nil {
		return ValidationResult{}, err
	}
	result, err := s.commit(operationCtx, operation, command.CommandMeta, aggregate, "VALIDATION_EXECUTED", map[string][]string{"findingIds": ids})
	if err != nil {
		return ValidationResult{}, err
	}
	if result.Replay {
		var stored struct {
			FindingIDs []string `json:"findingIds"`
		}
		if json.Unmarshal(result.Result, &stored) == nil {
			findings = findings[:0]
			for _, id := range stored.FindingIDs {
				if finding, ok := result.Aggregate.Findings[id]; ok {
					findings = append(findings, finding)
				}
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	return ValidationResult{Task: result.Aggregate.Task, Findings: findings, Replay: result.Replay}, nil
}
