package verification_cache_staleness_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/httpapi"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

func TestVerificationCacheInvalidatedOnVersionChange(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	aggregate, err := observatory.CreateTask(observatory.NewTask{
		ID: "task_cache", Title: "缓存版本测试", InstrumentCode: "CAM-9",
		ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour),
		Owner: "owner", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Task.Version = 1
	commitAggregate(t, store, aggregate, 0, "create-cache-test", now)

	server := httptest.NewServer(httpapi.New(archive.NewService(store, func() time.Time { return now }, nil)).Handler())
	defer server.Close()

	first := requestVerification(t, server.URL)
	if first.Valid {
		t.Fatal("pre-release verification unexpectedly succeeded")
	}

	entry := observatory.ManifestEntry{
		RevisionID: "rev_1", LogicalPath: "science/frame.fits", ByteSize: 4096,
		MediaType: "application/fits", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	entry.EntryDigest = observatory.ManifestEntryDigest(entry)
	manifest := observatory.FrozenManifest{
		ID: "manifest_1", TaskID: aggregate.Task.ID, TaskVersion: 2,
		Entries: []observatory.ManifestEntry{entry}, FrozenBy: "release-lead", FrozenAt: now,
	}
	manifest.MerkleRoot = observatory.MerkleRoot(manifest.Entries)
	credential := observatory.ReleaseCredential{
		ID: "credential_1", TaskID: aggregate.Task.ID, ManifestID: manifest.ID,
		ManifestRoot: manifest.MerkleRoot, ApprovedBy: "release-lead",
		PurposeScope: "科研公开复核", IssuedAt: now,
	}
	credential.CredentialDigest = observatory.CredentialDigest(credential)
	aggregate.Manifest = &manifest
	aggregate.Credential = &credential
	aggregate.Task.State = observatory.StateReleased
	aggregate.Task.Version = 2
	commitAggregate(t, store, aggregate, 1, "release-cache-test", now)

	second := requestVerification(t, server.URL)
	if !second.Valid {
		t.Fatalf("stale verification cache survived aggregate version change: %#v", second.Reasons)
	}
}

func commitAggregate(t *testing.T, store *persistence.Store, aggregate observatory.Aggregate, expected int64, key string, now time.Time) {
	t.Helper()
	_, err := store.Commit(context.Background(), persistence.CommitRequest{
		TaskID: aggregate.Task.ID, ExpectedVersion: expected,
		IdempotencyKey: key, Operation: key, Aggregate: aggregate,
		Audit: observatory.AuditEvent{
			TaskID: aggregate.Task.ID, TaskVersion: aggregate.Task.Version,
			Action: "CACHE_TEST_STATE_CHANGED", Actor: "tester", Role: archive.RoleReleaseLead,
			Reason: "验证缓存版本隔离", CorrelationID: key, OccurredAt: now,
		},
		Result: json.RawMessage(`{"taskId":"task_cache"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func requestVerification(t *testing.T, baseURL string) observatory.VerificationResult {
	t.Helper()
	response, err := http.Get(baseURL + "/api/v1/archive-tasks/task_cache/release-verification")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("verification returned HTTP %d", response.StatusCode)
	}
	var result observatory.VerificationResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
