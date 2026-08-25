package persistence

import "benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"

func observatoryClone(value observatory.Aggregate) observatory.Aggregate {
	return observatory.CloneAggregate(value)
}

func cloneSnapshotPayload(source snapshotPayload) snapshotPayload {
	clone := snapshotPayload{
		SchemaVersion: source.SchemaVersion, LastSequence: source.LastSequence, LastHash: source.LastHash,
		Aggregates:  make(map[string]observatory.Aggregate, len(source.Aggregates)),
		Audits:      make(map[string][]observatory.AuditEvent, len(source.Audits)),
		Idempotency: make(map[string]idempotencyRecord, len(source.Idempotency)),
	}
	for id, aggregate := range source.Aggregates {
		clone.Aggregates[id] = observatory.CloneAggregate(aggregate)
	}
	for id, events := range source.Audits {
		clone.Audits[id] = append([]observatory.AuditEvent(nil), events...)
	}
	for key, record := range source.Idempotency {
		record.Result = append([]byte(nil), record.Result...)
		record.Aggregate = observatory.CloneAggregate(record.Aggregate)
		clone.Idempotency[key] = record
	}
	return clone
}
