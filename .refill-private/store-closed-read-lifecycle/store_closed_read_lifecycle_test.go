package store_closed_read_lifecycle_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

func TestClosedStoreRejectsEveryReadOperation(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	aggregate, err := observatory.CreateTask(observatory.NewTask{ID: "task-private", Title: "关闭生命周期测试",
		InstrumentCode: "CAM-1", ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour), Owner: "owner", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Task.Version = 1
	_, err = store.Commit(context.Background(), persistence.CommitRequest{TaskID: aggregate.Task.ID, ExpectedVersion: 0,
		IdempotencyKey: "create-private", Operation: "create-task", Aggregate: aggregate,
		Audit: observatory.AuditEvent{TaskID: aggregate.Task.ID, TaskVersion: 1, Action: "CREATED"},
		Result: json.RawMessage(`{"taskId":"task-private"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	_, getErr := store.Get(context.Background(), aggregate.Task.ID)
	_, timelineErr := store.Timeline(context.Background(), aggregate.Task.ID)
	_, listErr := store.List(context.Background())
	if persistence.ErrorCode(getErr) != "STORE_CLOSED" || persistence.ErrorCode(timelineErr) != "STORE_CLOSED" || persistence.ErrorCode(listErr) != "STORE_CLOSED" {
		t.Fatalf("closed store reads remained usable: Get=%v Timeline=%v List=%v", getErr, timelineErr, listErr)
	}
}
