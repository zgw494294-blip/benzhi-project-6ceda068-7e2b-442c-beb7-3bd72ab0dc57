package persistence

import (
	"encoding/json"
	"strings"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

const schemaVersion = 1

type eventRecord struct {
	SchemaVersion   int                    `json:"schemaVersion"`
	Sequence        int64                  `json:"sequence"`
	PreviousHash    string                 `json:"previousHash"`
	CommittedAt     time.Time              `json:"committedAt"`
	TaskID          string                 `json:"taskId"`
	ExpectedVersion int64                  `json:"expectedVersion"`
	IdempotencyKey  string                 `json:"idempotencyKey"`
	Operation       string                 `json:"operation"`
	Aggregate       observatory.Aggregate  `json:"aggregate"`
	Audit           observatory.AuditEvent `json:"audit"`
	Result          json.RawMessage        `json:"result"`
	Hash            string                 `json:"hash"`
}

type idempotencyRecord struct {
	TaskID    string                `json:"taskId"`
	Operation string                `json:"operation"`
	Result    json.RawMessage       `json:"result"`
	Aggregate observatory.Aggregate `json:"aggregate"`
}

type snapshotPayload struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	LastSequence  int64                               `json:"lastSequence"`
	LastHash      string                              `json:"lastHash"`
	Aggregates    map[string]observatory.Aggregate    `json:"aggregates"`
	Audits        map[string][]observatory.AuditEvent `json:"audits"`
	Idempotency   map[string]idempotencyRecord        `json:"idempotency"`
}

type snapshotEnvelope struct {
	Payload  snapshotPayload `json:"payload"`
	Checksum string          `json:"checksum"`
}

// idempotencyMapKey builds the idempotency index key from an operation and an
// idempotency key. The key is scoped by the operation type (the portion of the
// operation before the task identifier separator) so that the same
// idempotency key reused on a different task for the same operation type
// resolves to the same index entry and is rejected as a reuse.
func idempotencyMapKey(operation, idempotencyKey string) string {
	operationType := operation
	if index := strings.Index(operation, ":"); index >= 0 {
		operationType = operation[:index]
	}
	return operationType + "\x00" + idempotencyKey
}
