package archive

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

const (
	RoleAdministrator = "DATA_ADMIN"
	RoleReviewer      = "QUALITY_REVIEWER"
	RoleReleaseLead   = "RELEASE_LEAD"
)

type Service struct {
	repository          persistence.Repository
	now                 func() time.Time
	ids                 IDGenerator
	revisionViewScratch []observatory.DatasetRevision
	findingViewScratch  []observatory.ValidationFinding
}

func NewService(repository persistence.Repository, clock func() time.Time, ids IDGenerator) *Service {
	if clock == nil {
		clock = time.Now
	}
	if ids == nil {
		ids = &RandomIDGenerator{}
	}
	return &Service{repository: repository, now: clock, ids: ids}
}

func validateMeta(meta CommandMeta, roles ...string) error {
	meta.IdempotencyKey = strings.TrimSpace(meta.IdempotencyKey)
	meta.Actor = strings.TrimSpace(meta.Actor)
	meta.Role = strings.TrimSpace(meta.Role)
	meta.Reason = strings.TrimSpace(meta.Reason)
	meta.CorrelationID = strings.TrimSpace(meta.CorrelationID)
	if meta.IdempotencyKey == "" || len(meta.IdempotencyKey) > 160 {
		return fail("INVALID_IDEMPOTENCY_KEY", "idempotencyKey 不能为空且不能超过 160 个字符")
	}
	if meta.ExpectedVersion < 0 {
		return fail("INVALID_EXPECTED_VERSION", "expectedVersion 不能为负数")
	}
	if meta.Actor == "" {
		return fail("ACTOR_REQUIRED", "actor 不能为空")
	}
	allowed := false
	for _, role := range roles {
		if meta.Role == role {
			allowed = true
			break
		}
	}
	if !allowed {
		return fail("ROLE_FORBIDDEN", "角色 %s 无权执行该操作", meta.Role)
	}
	if len([]rune(meta.Reason)) < 2 || len([]rune(meta.Reason)) > 500 {
		return fail("INVALID_REASON", "reason 长度必须为 2 到 500 个字符")
	}
	if meta.CorrelationID == "" || len(meta.CorrelationID) > 160 {
		return fail("INVALID_CORRELATION_ID", "correlationId 不能为空且不能超过 160 个字符")
	}
	return nil
}

func (s *Service) commit(ctx context.Context, operation string, meta CommandMeta, aggregate observatory.Aggregate, action string, result any) (persistence.CommitResult, error) {
	aggregate.Task.Version = meta.ExpectedVersion + 1
	aggregate.Task.UpdatedAt = s.now().UTC()
	encoded, err := json.Marshal(result)
	if err != nil {
		return persistence.CommitResult{}, fail("RESULT_ENCODE_FAILED", "编码操作结果失败")
	}
	audit := observatory.AuditEvent{
		TaskID: aggregate.Task.ID, TaskVersion: aggregate.Task.Version, Action: action,
		Actor: strings.TrimSpace(meta.Actor), Role: strings.TrimSpace(meta.Role),
		Reason: strings.TrimSpace(meta.Reason), CorrelationID: strings.TrimSpace(meta.CorrelationID),
		OccurredAt: s.now().UTC(),
	}
	return s.repository.Commit(ctx, persistence.CommitRequest{
		TaskID: aggregate.Task.ID, ExpectedVersion: meta.ExpectedVersion,
		IdempotencyKey: strings.TrimSpace(meta.IdempotencyKey), Operation: operation,
		Aggregate: aggregate, Audit: audit, Result: encoded,
	})
}

func (s *Service) replay(ctx context.Context, operation, taskID, key string) (persistence.CommitResult, bool, error) {
	return s.repository.IdempotentResult(ctx, operation, strings.TrimSpace(key), taskID)
}

func (s *Service) detailFromAggregate(aggregate observatory.Aggregate) TaskDetail {
	s.revisionViewScratch = s.revisionViewScratch[:0]
	for _, revision := range aggregate.Revisions {
		s.revisionViewScratch = append(s.revisionViewScratch, revision)
	}
	revisions := s.revisionViewScratch
	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].LogicalPath != revisions[j].LogicalPath {
			return revisions[i].LogicalPath < revisions[j].LogicalPath
		}
		return revisions[i].SubmittedAt.Before(revisions[j].SubmittedAt)
	})
	s.findingViewScratch = s.findingViewScratch[:0]
	for _, finding := range aggregate.Findings {
		s.findingViewScratch = append(s.findingViewScratch, finding)
	}
	findings := s.findingViewScratch
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].RevisionID != findings[j].RevisionID {
			return findings[i].RevisionID < findings[j].RevisionID
		}
		if findings[i].RuleCode != findings[j].RuleCode {
			return findings[i].RuleCode < findings[j].RuleCode
		}
		return findings[i].ID < findings[j].ID
	})
	return TaskDetail{Task: aggregate.Task, Revisions: revisions, Findings: findings, Manifest: aggregate.Manifest, Credential: aggregate.Credential}
}
