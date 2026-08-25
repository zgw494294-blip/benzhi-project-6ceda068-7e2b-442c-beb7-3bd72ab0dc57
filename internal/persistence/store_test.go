package persistence

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func TestCommitReplayAndRecovery(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	aggregate, _ := observatory.CreateTask(observatory.NewTask{ID: "task_test", Title: "恢复测试任务", InstrumentCode: "CAM-1", ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour), Owner: "owner", Now: now})
	aggregate.Task.Version = 1
	resultJSON := json.RawMessage(`{"taskId":"task_test"}`)
	request := CommitRequest{
		TaskID: "task_test", ExpectedVersion: 0, IdempotencyKey: "create-key", Operation: "create-task",
		Aggregate: aggregate, Audit: observatory.AuditEvent{TaskID: "task_test", TaskVersion: 1, Action: "CREATED", Actor: "admin", Role: "DATA_ADMIN", Reason: "测试创建", CorrelationID: "test", OccurredAt: now}, Result: resultJSON,
	}
	first, err := store.Commit(context.Background(), request)
	if err != nil || first.Replay {
		t.Fatalf("首次提交失败：%v", err)
	}
	replayed, err := store.Commit(context.Background(), request)
	if err != nil || !replayed.Replay || replayed.Aggregate.Task.Version != 1 {
		t.Fatalf("幂等重放失败：%v %#v", err, replayed)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	loaded, err := recovered.Get(context.Background(), "task_test")
	if err != nil || loaded.Task.Version != 1 {
		t.Fatalf("恢复聚合失败：%v %#v", err, loaded.Task)
	}
	timeline, err := recovered.Timeline(context.Background(), "task_test")
	if err != nil || len(timeline) != 1 || timeline[0].Sequence != 1 {
		t.Fatalf("恢复时间线失败：%v %#v", err, timeline)
	}
}

func TestExpectedVersionConflict(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	aggregate, _ := observatory.CreateTask(observatory.NewTask{ID: "task", Title: "并发测试", InstrumentCode: "CAM-1", ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour), Owner: "owner", Now: now})
	aggregate.Task.Version = 2
	_, err = store.Commit(context.Background(), CommitRequest{TaskID: "task", ExpectedVersion: 1, IdempotencyKey: "wrong", Operation: "test", Aggregate: aggregate, Audit: observatory.AuditEvent{TaskID: "task", TaskVersion: 2}})
	if ErrorCode(err) != "VERSION_CONFLICT" {
		t.Fatalf("应返回 VERSION_CONFLICT，实际为 %v", err)
	}
}
