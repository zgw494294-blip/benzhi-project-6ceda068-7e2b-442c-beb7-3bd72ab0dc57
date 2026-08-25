package archive

import "benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"

type TaskDetail struct {
	Task       observatory.ArchiveTask         `json:"task"`
	Revisions  []observatory.DatasetRevision   `json:"revisions"`
	Findings   []observatory.ValidationFinding `json:"findings"`
	Manifest   *observatory.FrozenManifest     `json:"manifest,omitempty"`
	Credential *observatory.ReleaseCredential  `json:"credential,omitempty"`
}

type MutationResult struct {
	Task   observatory.ArchiveTask `json:"task"`
	Replay bool                    `json:"idempotentReplay"`
}

type RevisionResult struct {
	Task     observatory.ArchiveTask     `json:"task"`
	Revision observatory.DatasetRevision `json:"revision"`
	Replay   bool                        `json:"idempotentReplay"`
}

type ValidationResult struct {
	Task     observatory.ArchiveTask         `json:"task"`
	Findings []observatory.ValidationFinding `json:"findings"`
	Replay   bool                            `json:"idempotentReplay"`
}

type ResolutionResult struct {
	Task    observatory.ArchiveTask       `json:"task"`
	Finding observatory.ValidationFinding `json:"finding"`
	Replay  bool                          `json:"idempotentReplay"`
}

type ManifestResult struct {
	Task     observatory.ArchiveTask    `json:"task"`
	Manifest observatory.FrozenManifest `json:"manifest"`
	Replay   bool                       `json:"idempotentReplay"`
}

type CredentialResult struct {
	Task       observatory.ArchiveTask       `json:"task"`
	Credential observatory.ReleaseCredential `json:"credential"`
	Replay     bool                          `json:"idempotentReplay"`
}

type TimelineResult struct {
	TaskID string                   `json:"taskId"`
	Events []observatory.AuditEvent `json:"events"`
}
