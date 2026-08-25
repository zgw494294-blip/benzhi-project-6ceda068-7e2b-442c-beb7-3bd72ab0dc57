package persistence

import (
	"context"
	"encoding/json"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

type CommitRequest struct {
	TaskID          string
	ExpectedVersion int64
	IdempotencyKey  string
	Operation       string
	Aggregate       observatory.Aggregate
	Audit           observatory.AuditEvent
	Result          json.RawMessage
}

type CommitResult struct {
	Aggregate observatory.Aggregate
	Result    json.RawMessage
	Replay    bool
}

type Repository interface {
	Commit(context.Context, CommitRequest) (CommitResult, error)
	IdempotentResult(context.Context, string, string, string) (CommitResult, bool, error)
	Get(context.Context, string) (observatory.Aggregate, error)
	Timeline(context.Context, string) ([]observatory.AuditEvent, error)
	List(context.Context) ([]observatory.Aggregate, error)
	Close() error
}
