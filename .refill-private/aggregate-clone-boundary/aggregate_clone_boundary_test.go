package aggregatecloneboundary_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

func TestCommitResultMutationDoesNotPolluteStoredAggregate(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	originalReviewTime := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	reviewedAt := originalReviewTime
	aggregate := observatory.Aggregate{
		Task: observatory.ArchiveTask{ID: "task_clone", Version: 1},
		Revisions: map[string]observatory.DatasetRevision{
			"revision_1": {ID: "revision_1", TaskID: "task_clone"},
		},
		Findings: map[string]observatory.ValidationFinding{
			"finding_1": {ID: "finding_1", TaskID: "task_clone", ReviewedAt: &reviewedAt},
		},
		Manifest: &observatory.FrozenManifest{
			ID: "manifest_1", TaskID: "task_clone",
			Entries:    []observatory.ManifestEntry{{RevisionID: "revision_1", SHA256: "original-sha"}},
			MerkleRoot: "original-root",
		},
		Credential: &observatory.ReleaseCredential{
			ID: "credential_1", TaskID: "task_clone", CredentialDigest: "original-digest",
		},
	}
	result, err := store.Commit(context.Background(), persistence.CommitRequest{
		TaskID: "task_clone", ExpectedVersion: 0, IdempotencyKey: "clone-boundary-key",
		Operation: "create", Aggregate: aggregate,
		Audit:  observatory.AuditEvent{TaskID: "task_clone", TaskVersion: 1},
		Result: json.RawMessage(`{"taskId":"task_clone"}`),
	})
	if err != nil {
		t.Fatalf("commit aggregate: %v", err)
	}

	mutatedReviewTime := originalReviewTime.Add(48 * time.Hour)
	*result.Aggregate.Findings["finding_1"].ReviewedAt = mutatedReviewTime
	result.Aggregate.Manifest.Entries[0].SHA256 = "mutated-sha"
	result.Aggregate.Manifest.MerkleRoot = "mutated-root"
	result.Aggregate.Credential.CredentialDigest = "mutated-digest"

	stored, err := store.Get(context.Background(), "task_clone")
	if err != nil {
		t.Fatalf("get aggregate: %v", err)
	}
	finding := stored.Findings["finding_1"]
	if finding.ReviewedAt == nil || !finding.ReviewedAt.Equal(originalReviewTime) ||
		stored.Manifest == nil || stored.Manifest.Entries[0].SHA256 != "original-sha" ||
		stored.Manifest.MerkleRoot != "original-root" || stored.Credential == nil ||
		stored.Credential.CredentialDigest != "original-digest" {
		t.Fatal("commit result mutation polluted stored aggregate")
	}
}
