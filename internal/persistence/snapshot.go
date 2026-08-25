package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

func snapshotChecksum(payload snapshotPayload) string {
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func readSnapshot(path string) (snapshotPayload, bool, error) {
	encoded, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return snapshotPayload{}, false, nil
	}
	if err != nil {
		return snapshotPayload{}, false, errf("SNAPSHOT_READ_FAILED", "读取投影快照失败：%v", err)
	}
	var envelope snapshotEnvelope
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		return snapshotPayload{}, false, errf("SNAPSHOT_CORRUPT", "投影快照不是有效 JSON：%v", err)
	}
	if envelope.Payload.SchemaVersion != schemaVersion {
		return snapshotPayload{}, false, errf("SNAPSHOT_SCHEMA_UNSUPPORTED", "不支持快照 schemaVersion %d", envelope.Payload.SchemaVersion)
	}
	if snapshotChecksum(envelope.Payload) != envelope.Checksum {
		return snapshotPayload{}, false, errf("SNAPSHOT_CHECKSUM_INVALID", "投影快照校验和不匹配")
	}
	if envelope.Payload.Aggregates == nil {
		envelope.Payload.Aggregates = map[string]observatory.Aggregate{}
	}
	if envelope.Payload.Audits == nil {
		envelope.Payload.Audits = map[string][]observatory.AuditEvent{}
	}
	if envelope.Payload.Idempotency == nil {
		envelope.Payload.Idempotency = map[string]idempotencyRecord{}
	}
	return envelope.Payload, true, nil
}

func (s *Store) writeSnapshotLocked() error {
	payload := cloneSnapshotPayload(snapshotPayload{
		SchemaVersion: schemaVersion, LastSequence: s.lastSequence, LastHash: s.lastHash,
		Aggregates: s.aggregates, Audits: s.audits, Idempotency: s.idempotency,
	})
	envelope := snapshotEnvelope{Payload: payload, Checksum: snapshotChecksum(payload)}
	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return errf("SNAPSHOT_ENCODE_FAILED", "编码投影快照失败：%v", err)
	}
	temporary, err := os.CreateTemp(s.directory, ".snapshot-*.tmp")
	if err != nil {
		return errf("SNAPSHOT_TEMP_FAILED", "创建快照临时文件失败：%v", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		temporary.Close()
		os.Remove(temporaryPath)
	}
	if err := temporary.Chmod(0o640); err != nil {
		cleanup()
		return errf("SNAPSHOT_WRITE_FAILED", "设置快照权限失败：%v", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		cleanup()
		return errf("SNAPSHOT_WRITE_FAILED", "写入快照失败：%v", err)
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return errf("SNAPSHOT_SYNC_FAILED", "同步快照失败：%v", err)
	}
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryPath)
		return errf("SNAPSHOT_CLOSE_FAILED", "关闭快照失败：%v", err)
	}
	if err := os.Rename(temporaryPath, s.snapshotPath); err != nil {
		os.Remove(temporaryPath)
		return errf("SNAPSHOT_RENAME_FAILED", "替换快照失败：%v", err)
	}
	directory, err := os.Open(filepath.Dir(s.snapshotPath))
	if err != nil {
		return errf("DIRECTORY_SYNC_FAILED", "打开快照目录失败：%v", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return errf("DIRECTORY_SYNC_FAILED", "同步快照目录失败：%v", err)
	}
	return nil
}
