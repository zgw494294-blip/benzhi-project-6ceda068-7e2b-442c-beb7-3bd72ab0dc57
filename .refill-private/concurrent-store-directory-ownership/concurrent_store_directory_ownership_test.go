package concurrent_store_directory_ownership_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

func TestConcurrentStoresCannotCommitSameSequence(t *testing.T) {
	directory := t.TempDir()
	bootstrap, err := persistence.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	aggregate, err := observatory.CreateTask(observatory.NewTask{ID: "task-private", Title: "目录所有权测试",
		InstrumentCode: "CAM-1", ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour), Owner: "owner", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Task.Version = 1
	_, err = bootstrap.Commit(context.Background(), persistence.CommitRequest{TaskID: aggregate.Task.ID, ExpectedVersion: 0,
		IdempotencyKey: "bootstrap", Operation: "create-task", Aggregate: aggregate,
		Audit: observatory.AuditEvent{TaskID: aggregate.Task.ID, TaskVersion: 1, Action: "CREATED"},
		Result: json.RawMessage(`{"taskId":"task-private"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.Close(); err != nil {
		t.Fatal(err)
	}

	first, err := persistence.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := persistence.Open(directory)
	if err != nil {
		return
	}
	defer second.Close()

	start := make(chan struct{})
	results := make(chan error, 2)
	commit := func(store *persistence.Store, key, title string) {
		<-start
		candidate := observatory.CloneAggregate(aggregate)
		candidate.Task.Version = 2
		candidate.Task.Title = title
		_, commitErr := store.Commit(context.Background(), persistence.CommitRequest{TaskID: candidate.Task.ID, ExpectedVersion: 1,
			IdempotencyKey: key, Operation: "rename-task", Aggregate: candidate,
			Audit: observatory.AuditEvent{TaskID: candidate.Task.ID, TaskVersion: 2, Action: "RENAMED"},
			Result: json.RawMessage(`{"ok":true}`)})
		results <- commitErr
	}
	go commit(first, "first", "并发标题甲")
	go commit(second, "second", "并发标题乙")
	close(start)
	firstErr := <-results
	secondErr := <-results
	if firstErr == nil && secondErr == nil {
		if err := first.Close(); err != nil {
			t.Fatal(err)
		}
		if err := second.Close(); err != nil {
			t.Fatal(err)
		}
		recovered, recoveryErr := persistence.Open(directory)
		if recoveryErr == nil {
			_ = recovered.Close()
			t.Fatal("concurrent stores committed the same event sequence but recovery unexpectedly succeeded")
		}
		t.Fatalf("concurrent stores committed the same event sequence: recovery=%v", recoveryErr)
	}
}
