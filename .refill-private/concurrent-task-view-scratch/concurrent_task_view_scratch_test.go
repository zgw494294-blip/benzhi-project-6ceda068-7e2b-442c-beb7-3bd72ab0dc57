package concurrent_task_view_scratch_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

type barrierRepository struct {
	aggregates map[string]observatory.Aggregate
	ready      chan string
	release    chan struct{}
	blocked    atomic.Bool
}

func (r *barrierRepository) Get(ctx context.Context, taskID string) (observatory.Aggregate, error) {
	if r.blocked.Load() {
		select {
		case r.ready <- taskID:
		case <-ctx.Done():
			return observatory.Aggregate{}, ctx.Err()
		}
		select {
		case <-r.release:
		case <-ctx.Done():
			return observatory.Aggregate{}, ctx.Err()
		}
	}
	return observatory.CloneAggregate(r.aggregates[taskID]), nil
}

func (r *barrierRepository) Commit(context.Context, persistence.CommitRequest) (persistence.CommitResult, error) {
	panic("Commit must not be called")
}

func (r *barrierRepository) IdempotentResult(context.Context, string, string, string) (persistence.CommitResult, bool, error) {
	panic("IdempotentResult must not be called")
}

func (r *barrierRepository) Timeline(context.Context, string) ([]observatory.AuditEvent, error) {
	panic("Timeline must not be called")
}

func (r *barrierRepository) List(context.Context) ([]observatory.Aggregate, error) {
	panic("List must not be called")
}

func (r *barrierRepository) Close() error { return nil }

func TestConcurrentTaskViewsShareScratch(t *testing.T) {
	repository := &barrierRepository{
		aggregates: map[string]observatory.Aggregate{
			"task-a": aggregate("task-a", "rev-a", "finding-a"),
			"task-b": aggregate("task-b", "rev-b", "finding-b"),
		},
		ready:   make(chan string, 2),
		release: make(chan struct{}),
	}
	service := archive.NewService(repository, nil, nil)

	if _, err := service.GetTask(context.Background(), "task-a"); err != nil {
		t.Fatalf("prewarm task detail: %v", err)
	}
	repository.blocked.Store(true)

	var results [2]archive.TaskDetail
	var errors [2]error
	var wait sync.WaitGroup
	wait.Add(2)
	for index, taskID := range []string{"task-a", "task-b"} {
		index, taskID := index, taskID
		go func() {
			defer wait.Done()
			results[index], errors[index] = service.GetTask(context.Background(), taskID)
		}()
	}
	<-repository.ready
	<-repository.ready
	close(repository.release)
	wait.Wait()

	for index, expected := range []struct {
		taskID    string
		revision  string
		findingID string
	}{{"task-a", "rev-a", "finding-a"}, {"task-b", "rev-b", "finding-b"}} {
		if errors[index] != nil {
			t.Fatalf("get %s: %v", expected.taskID, errors[index])
		}
		if len(results[index].Revisions) != 1 || results[index].Revisions[0].ID != expected.revision ||
			len(results[index].Findings) != 1 || results[index].Findings[0].ID != expected.findingID {
			encoded, _ := json.Marshal(results[index])
			t.Fatalf("concurrent task detail for %s was contaminated: %s", expected.taskID, encoded)
		}
	}
}

func aggregate(taskID, revisionID, findingID string) observatory.Aggregate {
	return observatory.Aggregate{
		Task: observatory.ArchiveTask{ID: taskID, Version: 1},
		Revisions: map[string]observatory.DatasetRevision{
			revisionID: {ID: revisionID, TaskID: taskID, LogicalPath: taskID + ".fits"},
		},
		Findings: map[string]observatory.ValidationFinding{
			findingID: {ID: findingID, TaskID: taskID, RevisionID: revisionID},
		},
	}
}
