package persistence

import (
	"encoding/json"
	"os"
	"reflect"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func (s *Store) recover() error {
	snapshot, snapshotExists, err := readSnapshot(s.snapshotPath)
	if err != nil {
		return err
	}
	if snapshotExists {
		s.aggregates = snapshot.Aggregates
		s.audits = snapshot.Audits
		s.idempotency = snapshot.Idempotency
		s.lastSequence = snapshot.LastSequence
		s.lastHash = snapshot.LastHash
	}
	verifiedSequence := int64(0)
	verifiedHash := ""
	derived := newDerivedProjection()
	err = readEventLog(s.logPath, func(record eventRecord) error {
		if record.SchemaVersion != schemaVersion {
			return errf("EVENT_SCHEMA_UNSUPPORTED", "不支持事件 schemaVersion %d", record.SchemaVersion)
		}
		if record.Sequence != verifiedSequence+1 {
			return errf("EVENT_SEQUENCE_BROKEN", "事件序号不连续：收到 %d", record.Sequence)
		}
		if record.PreviousHash != verifiedHash {
			return errf("EVENT_CHAIN_BROKEN", "事件 %d 的前序哈希不匹配", record.Sequence)
		}
		if !validHexDigest(record.Hash) || calculateRecordHash(record) != record.Hash {
			return errf("EVENT_HASH_INVALID", "事件 %d 的内容哈希无效", record.Sequence)
		}
		verifiedSequence = record.Sequence
		verifiedHash = record.Hash
		derived.apply(record)
		if snapshotExists && record.Sequence == snapshot.LastSequence {
			if err := derived.compareWithSnapshot(snapshot); err != nil {
				return err
			}
		}
		if record.Sequence > s.lastSequence {
			s.applyRecord(record)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if snapshotExists && snapshot.LastSequence > verifiedSequence {
		return errf("EVENT_LOG_TRUNCATED", "事件日志短于快照记录的提交位置")
	}
	if s.lastSequence != verifiedSequence || s.lastHash != verifiedHash {
		if verifiedSequence == 0 && !snapshotExists {
			return nil
		}
		return errf("RECOVERY_POSITION_MISMATCH", "恢复位置与事件日志不一致")
	}
	if _, err := os.Stat(s.logPath); err != nil && !os.IsNotExist(err) {
		return errf("EVENT_LOG_STAT_FAILED", "检查事件日志失败：%v", err)
	}
	return nil
}

func (s *Store) applyRecord(record eventRecord) {
	aggregate := observatoryClone(record.Aggregate)
	s.aggregates[record.TaskID] = aggregate
	s.audits[record.TaskID] = append(s.audits[record.TaskID], record.Audit)
	key := record.Operation + "\x00" + record.IdempotencyKey
	s.idempotency[key] = idempotencyRecord{
		TaskID: record.TaskID, Operation: record.Operation,
		Result: append([]byte(nil), record.Result...), Aggregate: observatoryClone(record.Aggregate),
	}
	s.lastSequence = record.Sequence
	s.lastHash = record.Hash
}

type derivedProjection struct {
	aggregates  map[string]observatory.Aggregate
	audits      map[string][]observatory.AuditEvent
	idempotency map[string]idempotencyRecord
}

func newDerivedProjection() *derivedProjection {
	return &derivedProjection{
		aggregates:  map[string]observatory.Aggregate{},
		audits:      map[string][]observatory.AuditEvent{},
		idempotency: map[string]idempotencyRecord{},
	}
}

func (d *derivedProjection) apply(record eventRecord) {
	d.aggregates[record.TaskID] = observatoryClone(record.Aggregate)
	d.audits[record.TaskID] = append(d.audits[record.TaskID], record.Audit)
	key := record.Operation + "\x00" + record.IdempotencyKey
	d.idempotency[key] = idempotencyRecord{
		TaskID: record.TaskID, Operation: record.Operation,
		Result: append([]byte(nil), record.Result...), Aggregate: observatoryClone(record.Aggregate),
	}
}

func (d *derivedProjection) compareWithSnapshot(snapshot snapshotPayload) error {
	if len(d.aggregates) != len(snapshot.Aggregates) {
		return errf("SNAPSHOT_PROJECTION_MISMATCH", "快照聚合投影与事件日志推导结果不一致")
	}
	for id, aggregate := range snapshot.Aggregates {
		derived, ok := d.aggregates[id]
		if !ok || !equalAggregate(derived, aggregate) {
			return errf("SNAPSHOT_PROJECTION_MISMATCH", "快照聚合投影与事件日志推导结果不一致")
		}
	}
	if len(d.audits) != len(snapshot.Audits) {
		return errf("SNAPSHOT_PROJECTION_MISMATCH", "快照审计投影与事件日志推导结果不一致")
	}
	for id, events := range snapshot.Audits {
		derived, ok := d.audits[id]
		if !ok || len(derived) != len(events) {
			return errf("SNAPSHOT_PROJECTION_MISMATCH", "快照审计投影与事件日志推导结果不一致")
		}
		for index := range events {
			if !reflect.DeepEqual(derived[index], events[index]) {
				return errf("SNAPSHOT_PROJECTION_MISMATCH", "快照审计投影与事件日志推导结果不一致")
			}
		}
	}
	if len(d.idempotency) != len(snapshot.Idempotency) {
		return errf("SNAPSHOT_PROJECTION_MISMATCH", "快照幂等投影与事件日志推导结果不一致")
	}
	for key, record := range snapshot.Idempotency {
		derived, ok := d.idempotency[key]
		if !ok || derived.TaskID != record.TaskID || derived.Operation != record.Operation {
			return errf("SNAPSHOT_PROJECTION_MISMATCH", "快照幂等投影与事件日志推导结果不一致")
		}
		if !rawJSONEqual(derived.Result, record.Result) || !equalAggregate(derived.Aggregate, record.Aggregate) {
			return errf("SNAPSHOT_PROJECTION_MISMATCH", "快照幂等投影与事件日志推导结果不一致")
		}
	}
	return nil
}

func rawJSONEqual(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return reflect.DeepEqual(a, b)
	}
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

func equalAggregate(a, b observatory.Aggregate) bool {
	return reflect.DeepEqual(a.Task, b.Task) &&
		reflect.DeepEqual(a.Revisions, b.Revisions) &&
		reflect.DeepEqual(a.Findings, b.Findings) &&
		reflect.DeepEqual(a.Manifest, b.Manifest) &&
		reflect.DeepEqual(a.Credential, b.Credential)
}
