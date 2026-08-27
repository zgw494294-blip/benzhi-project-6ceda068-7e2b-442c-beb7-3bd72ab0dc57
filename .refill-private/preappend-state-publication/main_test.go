package preappendstatepublication_test

import (
	"context"
	"encoding/json"
	"syscall"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

func TestFailedAppendDoesNotPublishState(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	aggregate, err := observatory.CreateTask(observatory.NewTask{
		ID: "task_append_failure", Title: "日志失败原子性测试", InstrumentCode: "CAM-FAIL",
		ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour),
		Owner: "test-owner", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Task.Version = 1

	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	limited := original
	limited.Cur = 0
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited); err != nil {
		t.Fatal(err)
	}
	restored := false
	defer func() {
		if !restored {
			_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original)
		}
	}()

	_, commitErr := store.Commit(context.Background(), persistence.CommitRequest{
		TaskID: "task_append_failure", ExpectedVersion: 0,
		IdempotencyKey: "append-failure-key", Operation: "create-task",
		Aggregate: aggregate,
		Audit: observatory.AuditEvent{
			TaskID: "task_append_failure", TaskVersion: 1, Action: "ARCHIVE_TASK_CREATED",
			Actor: "test-admin", Role: "DATA_ADMIN", Reason: "验证日志失败原子性",
			CorrelationID: "append-failure-test", OccurredAt: now,
		},
		Result: json.RawMessage(`{"taskId":"task_append_failure"}`),
	})
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	restored = true
	if persistence.ErrorCode(commitErr) != "EVENT_APPEND_FAILED" {
		t.Fatalf("预期 EVENT_APPEND_FAILED，实际为 %v", commitErr)
	}

	_, getErr := store.Get(context.Background(), "task_append_failure")
	if persistence.ErrorCode(getErr) != "TASK_NOT_FOUND" {
		t.Fatalf("TestFailedAppendDoesNotPublishState: 事件日志追加失败后内存投影仍发布了任务，Get 错误为 %v", getErr)
	}
	if _, found, err := store.IdempotentResult(context.Background(), "create-task", "append-failure-key", "task_append_failure"); err != nil || found {
		t.Fatalf("TestFailedAppendDoesNotPublishState: 事件日志追加失败后幂等结果仍被发布，found=%v err=%v", found, err)
	}
}
