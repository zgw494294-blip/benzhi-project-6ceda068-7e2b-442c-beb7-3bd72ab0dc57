package snapshot_log_projection_mismatch_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

func TestSnapshotProjectionMustMatchEventLog(t *testing.T) {
	directory := t.TempDir()
	store, err := persistence.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	aggregate, err := observatory.CreateTask(observatory.NewTask{
		ID: "task-private", Title: "原始恢复标题", InstrumentCode: "CAM-1",
		ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour), Owner: "owner", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Task.Version = 1
	_, err = store.Commit(context.Background(), persistence.CommitRequest{
		TaskID: aggregate.Task.ID, ExpectedVersion: 0, IdempotencyKey: "create-private", Operation: "create-task",
		Aggregate: aggregate, Audit: observatory.AuditEvent{TaskID: aggregate.Task.ID, TaskVersion: 1,
			Action: "CREATED", Actor: "admin", Role: "DATA_ADMIN", Reason: "建立恢复测试", CorrelationID: "private", OccurredAt: now},
		Result: json.RawMessage(`{"taskId":"task-private"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(directory, "snapshot.json")
	encoded, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Payload  json.RawMessage `json:"payload"`
		Checksum string          `json:"checksum"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(envelope.Payload, []byte("原始恢复标题"), []byte("篡改恢复标题"), 1)
	if bytes.Equal(tampered, envelope.Payload) {
		t.Fatal("未能定位快照中的原始标题")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, tampered); err != nil {
		t.Fatal(err)
	}
	envelope.Payload = json.RawMessage(compact.Bytes())
	sum := sha256.Sum256(compact.Bytes())
	envelope.Checksum = hex.EncodeToString(sum[:])
	encoded, err = json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, encoded, 0o640); err != nil {
		t.Fatal(err)
	}

	recovered, err := persistence.Open(directory)
	if err == nil {
		_ = recovered.Close()
		t.Fatal("snapshot projection diverged from event log without rejection")
	}
}
