package archive

import (
	"context"
	"encoding/json"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func (s *Service) Freeze(ctx context.Context, taskID string, command FreezeCommand) (ManifestResult, error) {
	if err := validateMeta(command.CommandMeta, RoleReleaseLead); err != nil {
		return ManifestResult{}, err
	}
	operationCtx := context.WithoutCancel(ctx)
	operation := "freeze:" + taskID
	if previous, found, err := s.replay(operationCtx, operation, taskID, command.IdempotencyKey); err != nil {
		return ManifestResult{}, err
	} else if found && previous.Aggregate.Manifest != nil {
		return ManifestResult{Task: previous.Aggregate.Task, Manifest: *previous.Aggregate.Manifest, Replay: true}, nil
	}
	aggregate, err := s.repository.Get(operationCtx, taskID)
	if err != nil {
		return ManifestResult{}, err
	}
	manifest, err := observatory.BuildManifest(aggregate, s.ids.New("manifest"), command.Actor, s.now())
	if err != nil {
		return ManifestResult{}, err
	}
	aggregate.Manifest = &manifest
	aggregate.Task.State = observatory.StateFrozen
	result, err := s.commit(operationCtx, operation, command.CommandMeta, aggregate, "MANIFEST_FROZEN", map[string]string{"manifestId": manifest.ID})
	if err != nil {
		return ManifestResult{}, err
	}
	if result.Replay {
		var stored struct {
			ManifestID string `json:"manifestId"`
		}
		if json.Unmarshal(result.Result, &stored) == nil && result.Aggregate.Manifest != nil && result.Aggregate.Manifest.ID == stored.ManifestID {
			manifest = *result.Aggregate.Manifest
		}
	}
	return ManifestResult{Task: result.Aggregate.Task, Manifest: manifest, Replay: result.Replay}, nil
}

func (s *Service) IssueCredential(ctx context.Context, taskID string, command IssueCredentialCommand) (CredentialResult, error) {
	if err := validateMeta(command.CommandMeta, RoleReleaseLead); err != nil {
		return CredentialResult{}, err
	}
	operationCtx := context.WithoutCancel(ctx)
	operation := "issue-credential:" + taskID
	if previous, found, err := s.replay(operationCtx, operation, taskID, command.IdempotencyKey); err != nil {
		return CredentialResult{}, err
	} else if found && previous.Aggregate.Credential != nil {
		return CredentialResult{Task: previous.Aggregate.Task, Credential: *previous.Aggregate.Credential, Replay: true}, nil
	}
	aggregate, err := s.repository.Get(operationCtx, taskID)
	if err != nil {
		return CredentialResult{}, err
	}
	credential, err := observatory.IssueCredential(aggregate, s.ids.New("credential"), command.Actor, command.PurposeScope, s.now())
	if err != nil {
		return CredentialResult{}, err
	}
	aggregate.Credential = &credential
	aggregate.Task.State = observatory.StateReleased
	result, err := s.commit(operationCtx, operation, command.CommandMeta, aggregate, "RELEASE_CREDENTIAL_ISSUED", map[string]string{"credentialId": credential.ID})
	if err != nil {
		return CredentialResult{}, err
	}
	if result.Replay {
		var stored struct {
			CredentialID string `json:"credentialId"`
		}
		if json.Unmarshal(result.Result, &stored) == nil && result.Aggregate.Credential != nil && result.Aggregate.Credential.ID == stored.CredentialID {
			credential = *result.Aggregate.Credential
		}
	}
	return CredentialResult{Task: result.Aggregate.Task, Credential: credential, Replay: result.Replay}, nil
}
