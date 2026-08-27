package commit_error_chain_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/httpapi"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

type gatedRepository struct {
	inner    *persistence.Store
	taskID   string
	mu       sync.Mutex
	arrivals int
	ready    chan struct{}
}

func (r *gatedRepository) Commit(ctx context.Context, request persistence.CommitRequest) (persistence.CommitResult, error) {
	return r.inner.Commit(ctx, request)
}

func (r *gatedRepository) IdempotentResult(ctx context.Context, operation, key, taskID string) (persistence.CommitResult, bool, error) {
	return r.inner.IdempotentResult(ctx, operation, key, taskID)
}

func (r *gatedRepository) Get(ctx context.Context, taskID string) (observatory.Aggregate, error) {
	aggregate, err := r.inner.Get(ctx, taskID)
	if err != nil || taskID != r.taskID {
		return aggregate, err
	}
	r.mu.Lock()
	r.arrivals++
	if r.arrivals == 2 {
		close(r.ready)
	}
	ready := r.ready
	r.mu.Unlock()
	select {
	case <-ready:
		return aggregate, nil
	case <-ctx.Done():
		return observatory.Aggregate{}, ctx.Err()
	}
}

func (r *gatedRepository) Timeline(ctx context.Context, taskID string) ([]observatory.AuditEvent, error) {
	return r.inner.Timeline(ctx, taskID)
}

func (r *gatedRepository) List(ctx context.Context) ([]observatory.Aggregate, error) {
	return r.inner.List(ctx)
}

func (r *gatedRepository) Close() error { return r.inner.Close() }

type sequenceIDs struct {
	mu   sync.Mutex
	next int
}

func (s *sequenceIDs) New(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	return fmt.Sprintf("%s_%d", prefix, s.next)
}

type responseResult struct {
	status int
	code   string
}

func TestConcurrentVersionConflictPreservesErrorChain(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository := &gatedRepository{inner: store, ready: make(chan struct{})}
	t.Cleanup(func() { _ = repository.Close() })

	clock := func() time.Time { return time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC) }
	service := archive.NewService(repository, clock, &sequenceIDs{})
	created, err := service.CreateTask(context.Background(), archive.CreateTaskCommand{
		IdempotencyKey: "create-error-chain", ExpectedVersion: 0,
		Actor: "data-admin", Role: archive.RoleAdministrator,
		Reason: "建立并发冲突复现任务", CorrelationID: "corr-create-error-chain",
		Title: "并发错误链任务", InstrumentCode: "CAM-01", Owner: "observatory",
		ObservationStart: clock(), ObservationEnd: clock().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	repository.taskID = created.Task.ID
	handler := httpapi.New(service).Handler()

	start := make(chan struct{})
	results := make(chan responseResult, 2)
	for index := 0; index < 2; index++ {
		index := index
		go func() {
			<-start
			payload, marshalErr := json.Marshal(archive.RegisterRevisionCommand{
				CommandMeta: archive.CommandMeta{
					IdempotencyKey: fmt.Sprintf("revision-error-chain-%d", index), ExpectedVersion: 1,
					Actor: "data-admin", Role: archive.RoleAdministrator,
					Reason: "并发登记修订", CorrelationID: fmt.Sprintf("corr-revision-%d", index),
				},
				LogicalPath: fmt.Sprintf("raw/frame-%d.fits", index), ByteSize: 1024,
				MediaType: "application/fits", SHA256: fmt.Sprintf("%064x", index+1),
			})
			if marshalErr != nil {
				results <- responseResult{status: -1, code: marshalErr.Error()}
				return
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/archive-tasks/"+created.Task.ID+"/revisions", bytes.NewReader(payload))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			_ = json.Unmarshal(response.Body.Bytes(), &envelope)
			results <- responseResult{status: response.Code, code: envelope.Error.Code}
		}()
	}
	close(start)

	first := <-results
	second := <-results
	successes := 0
	conflicts := 0
	for _, result := range []responseResult{first, second} {
		if result.status == http.StatusCreated {
			successes++
		}
		if result.status == http.StatusConflict && result.code == "VERSION_CONFLICT" {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("TestConcurrentVersionConflictPreservesErrorChain: VERSION_CONFLICT error chain was lost; responses=%+v", []responseResult{first, second})
	}
}
