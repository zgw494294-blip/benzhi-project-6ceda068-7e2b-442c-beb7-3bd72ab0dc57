package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
)

type Store struct {
	mu           sync.RWMutex
	directory    string
	logPath      string
	snapshotPath string
	logFile      *os.File
	aggregates   map[string]observatory.Aggregate
	audits       map[string][]observatory.AuditEvent
	idempotency  map[string]idempotencyRecord
	lastSequence int64
	lastHash     string
	closed       bool
	now          func() time.Time
}

func Open(directory string) (*Store, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, errf("INVALID_DATA_DIRECTORY", "数据目录不能为空")
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, errf("DATA_DIRECTORY_FAILED", "创建数据目录失败：%v", err)
	}
	store := &Store{
		directory:    directory,
		logPath:      filepath.Join(directory, "events.jsonl"),
		snapshotPath: filepath.Join(directory, "snapshot.json"),
		aggregates:   map[string]observatory.Aggregate{},
		audits:       map[string][]observatory.AuditEvent{},
		idempotency:  map[string]idempotencyRecord{},
		now:          time.Now,
	}
	if err := store.recover(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(store.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return nil, errf("EVENT_LOG_OPEN_FAILED", "打开事件日志失败：%v", err)
	}
	store.logFile = file
	return store, nil
}

func (s *Store) Commit(ctx context.Context, request CommitRequest) (CommitResult, error) {
	if err := ctx.Err(); err != nil {
		return CommitResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return CommitResult{}, errf("STORE_CLOSED", "持久化仓储已关闭")
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return CommitResult{}, errf("IDEMPOTENCY_KEY_REQUIRED", "idempotencyKey 不能为空")
	}
	key := request.Operation + "\x00" + request.IdempotencyKey
	if previous, exists := s.idempotency[key]; exists {
		if previous.TaskID != request.TaskID || previous.Operation != request.Operation {
			return CommitResult{}, errf("IDEMPOTENCY_KEY_REUSED", "idempotencyKey 已被其他资源使用")
		}
		return CommitResult{Aggregate: observatory.CloneAggregate(previous.Aggregate), Result: append(json.RawMessage(nil), previous.Result...), Replay: true}, nil
	}
	current, exists := s.aggregates[request.TaskID]
	currentVersion := int64(0)
	if exists {
		currentVersion = current.Task.Version
	}
	if currentVersion != request.ExpectedVersion {
		return CommitResult{}, errf("VERSION_CONFLICT", "版本冲突：期望 %d，当前 %d", request.ExpectedVersion, currentVersion)
	}
	if request.Aggregate.Task.ID != request.TaskID {
		return CommitResult{}, errf("TASK_ID_MISMATCH", "提交聚合的任务标识不匹配")
	}
	if request.Aggregate.Task.Version != request.ExpectedVersion+1 {
		return CommitResult{}, errf("INVALID_NEXT_VERSION", "提交聚合版本必须恰好递增一次")
	}
	if request.Audit.TaskID != request.TaskID || request.Audit.TaskVersion != request.Aggregate.Task.Version {
		return CommitResult{}, errf("INVALID_AUDIT", "审计事件与提交版本不匹配")
	}
	record := eventRecord{
		SchemaVersion: schemaVersion, Sequence: s.lastSequence + 1, PreviousHash: s.lastHash,
		CommittedAt: s.now().UTC(), TaskID: request.TaskID, ExpectedVersion: request.ExpectedVersion,
		IdempotencyKey: request.IdempotencyKey, Operation: request.Operation,
		Aggregate: observatory.CloneAggregate(request.Aggregate), Audit: request.Audit,
		Result: append(json.RawMessage(nil), request.Result...),
	}
	record.Audit.Sequence = record.Sequence
	record.Hash = calculateRecordHash(record)
	if err := s.appendRecord(record); err != nil {
		return CommitResult{}, err
	}
	s.applyRecord(record)
	if err := s.writeSnapshotLocked(); err != nil {
		return CommitResult{}, err
	}
	return CommitResult{Aggregate: observatory.CloneAggregate(record.Aggregate), Result: append(json.RawMessage(nil), record.Result...)}, nil
}

func (s *Store) IdempotentResult(ctx context.Context, operation, key, taskID string) (CommitResult, bool, error) {
	if err := ctx.Err(); err != nil {
		return CommitResult{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return CommitResult{}, false, errf("STORE_CLOSED", "持久化仓储已关闭")
	}
	record, exists := s.idempotency[operation+"\x00"+strings.TrimSpace(key)]
	if !exists {
		return CommitResult{}, false, nil
	}
	if record.TaskID != taskID || record.Operation != operation {
		return CommitResult{}, false, errf("IDEMPOTENCY_KEY_REUSED", "idempotencyKey 已被其他资源使用")
	}
	return CommitResult{
		Aggregate: observatory.CloneAggregate(record.Aggregate),
		Result:    append(json.RawMessage(nil), record.Result...), Replay: true,
	}, true, nil
}

func (s *Store) Get(ctx context.Context, taskID string) (observatory.Aggregate, error) {
	if err := ctx.Err(); err != nil {
		return observatory.Aggregate{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return observatory.Aggregate{}, errf("STORE_CLOSED", "持久化仓储已关闭")
	}
	aggregate, ok := s.aggregates[taskID]
	if !ok {
		return observatory.Aggregate{}, errf("TASK_NOT_FOUND", "归档任务不存在")
	}
	return observatory.CloneAggregate(aggregate), nil
}

func (s *Store) Timeline(ctx context.Context, taskID string) ([]observatory.AuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errf("STORE_CLOSED", "持久化仓储已关闭")
	}
	if _, ok := s.aggregates[taskID]; !ok {
		return nil, errf("TASK_NOT_FOUND", "归档任务不存在")
	}
	events := append([]observatory.AuditEvent(nil), s.audits[taskID]...)
	sort.Slice(events, func(i, j int) bool { return events[i].Sequence < events[j].Sequence })
	return events, nil
}

func (s *Store) List(ctx context.Context) ([]observatory.Aggregate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errf("STORE_CLOSED", "持久化仓储已关闭")
	}
	items := make([]observatory.Aggregate, 0, len(s.aggregates))
	for _, aggregate := range s.aggregates {
		items = append(items, observatory.CloneAggregate(aggregate))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Task.CreatedAt.Equal(items[j].Task.CreatedAt) {
			return items[i].Task.ID < items[j].Task.ID
		}
		return items[i].Task.CreatedAt.Before(items[j].Task.CreatedAt)
	})
	return items, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.logFile == nil {
		return nil
	}
	return errors.Join(s.logFile.Sync(), s.logFile.Close())
}
