package persistence

import "os"

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
	key := idempotencyMapKey(record.Operation, record.IdempotencyKey)
	s.idempotency[key] = idempotencyRecord{
		TaskID: record.TaskID, Operation: record.Operation,
		Result: append([]byte(nil), record.Result...), Aggregate: observatoryClone(record.Aggregate),
	}
	s.lastSequence = record.Sequence
	s.lastHash = record.Hash
}
