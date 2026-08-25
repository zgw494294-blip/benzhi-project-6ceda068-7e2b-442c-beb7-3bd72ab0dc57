package persistence

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
)

func calculateRecordHash(record eventRecord) string {
	record.Hash = ""
	payload, _ := json.Marshal(record)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (s *Store) appendRecord(record eventRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return errf("EVENT_ENCODE_FAILED", "编码事件失败：%v", err)
	}
	payload = append(payload, '\n')
	if _, err := s.logFile.Write(payload); err != nil {
		return errf("EVENT_APPEND_FAILED", "追加事件失败：%v", err)
	}
	if err := s.logFile.Sync(); err != nil {
		return errf("EVENT_SYNC_FAILED", "同步事件日志失败：%v", err)
	}
	return nil
}

func readEventLog(path string, apply func(eventRecord) error) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errf("EVENT_LOG_OPEN_FAILED", "读取事件日志失败：%v", err)
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 256*1024)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				return errf("EVENT_LOG_TRUNCATED", "事件日志末尾记录不完整")
			}
			var record eventRecord
			if err := json.Unmarshal(line, &record); err != nil {
				return errf("EVENT_LOG_CORRUPT", "事件日志包含无效 JSON：%v", err)
			}
			if err := apply(record); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return errf("EVENT_LOG_READ_FAILED", "读取事件日志失败：%v", readErr)
		}
	}
}

func validHexDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
