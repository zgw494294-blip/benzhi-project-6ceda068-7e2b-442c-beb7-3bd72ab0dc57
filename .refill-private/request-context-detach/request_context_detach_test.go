package request_context_detach_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

const taskID = "task_context_lifecycle"

var fixedTime = time.Date(2026, 8, 25, 9, 30, 0, 0, time.UTC)

type fixedIDs struct{}

func (fixedIDs) New(prefix string) string { return prefix + "_context_lifecycle" }

type cancelAfterReplayRepository struct {
	aggregate observatory.Aggregate
	cancel    context.CancelFunc
	commits   int
}

func (r *cancelAfterReplayRepository) Commit(ctx context.Context, request persistence.CommitRequest) (persistence.CommitResult, error) {
	if err := ctx.Err(); err != nil {
		return persistence.CommitResult{}, err
	}
	r.commits++
	return persistence.CommitResult{Aggregate: observatory.CloneAggregate(request.Aggregate), Result: append([]byte(nil), request.Result...)}, nil
}

func (r *cancelAfterReplayRepository) IdempotentResult(ctx context.Context, operation, key, requestedTaskID string) (persistence.CommitResult, bool, error) {
	if err := ctx.Err(); err != nil {
		return persistence.CommitResult{}, false, err
	}
	r.cancel()
	return persistence.CommitResult{}, false, nil
}

func (r *cancelAfterReplayRepository) Get(ctx context.Context, requestedTaskID string) (observatory.Aggregate, error) {
	if err := ctx.Err(); err != nil {
		return observatory.Aggregate{}, err
	}
	return observatory.CloneAggregate(r.aggregate), nil
}

func (r *cancelAfterReplayRepository) Timeline(context.Context, string) ([]observatory.AuditEvent, error) {
	return nil, nil
}

func (r *cancelAfterReplayRepository) List(context.Context) ([]observatory.Aggregate, error) {
	return nil, nil
}

func (r *cancelAfterReplayRepository) Close() error { return nil }

type mutationScenario struct {
	name      string
	aggregate observatory.Aggregate
	invoke    func(*archive.Service, context.Context) error
}

func TestCanceledContextStopsAllMutationPhases(t *testing.T) {
	scenarios := []mutationScenario{
		{
			name:      "register revision",
			aggregate: aggregateFor(observatory.StateCollecting, nil),
			invoke: func(service *archive.Service, ctx context.Context) error {
				_, err := service.RegisterRevision(ctx, taskID, archive.RegisterRevisionCommand{
					CommandMeta: commandMeta(archive.RoleAdministrator, "cancel-register"),
					LogicalPath: "raw/context.fits", ByteSize: 2880, MediaType: "application/fits",
					SHA256: strings.Repeat("a", 64),
				})
				return err
			},
		},
		{
			name:      "validate",
			aggregate: aggregateFor(observatory.StateCollecting, validRevisions()),
			invoke: func(service *archive.Service, ctx context.Context) error {
				_, err := service.Validate(ctx, taskID, archive.ValidateCommand{CommandMeta: commandMeta(archive.RoleReviewer, "cancel-validate")})
				return err
			},
		},
		{
			name:      "propose resolution",
			aggregate: quarantinedAggregate(observatory.FindingOpen),
			invoke: func(service *archive.Service, ctx context.Context) error {
				_, err := service.ProposeResolution(ctx, taskID, archive.ProposeResolutionCommand{
					CommandMeta: commandMeta(archive.RoleAdministrator, "cancel-propose"), FindingID: "finding_blocking",
					ReplacementRevisionID: "rev_good", ResolutionNote: "替代修订已经完成校验",
				})
				return err
			},
		},
		{
			name:      "review resolution",
			aggregate: quarantinedAggregate(observatory.FindingProposed),
			invoke: func(service *archive.Service, ctx context.Context) error {
				_, err := service.ReviewResolution(ctx, taskID, archive.ReviewResolutionCommand{
					CommandMeta: commandMeta(archive.RoleReviewer, "cancel-review"), FindingID: "finding_blocking", Accepted: true,
				})
				return err
			},
		},
		{
			name:      "freeze",
			aggregate: aggregateFor(observatory.StateReviewPending, validRevisions()),
			invoke: func(service *archive.Service, ctx context.Context) error {
				_, err := service.Freeze(ctx, taskID, archive.FreezeCommand{CommandMeta: commandMeta(archive.RoleReleaseLead, "cancel-freeze")})
				return err
			},
		},
		{
			name:      "issue credential",
			aggregate: frozenAggregate(),
			invoke: func(service *archive.Service, ctx context.Context) error {
				_, err := service.IssueCredential(ctx, taskID, archive.IssueCredentialCommand{
					CommandMeta: commandMeta(archive.RoleReleaseLead, "cancel-credential"), PurposeScope: "公开科研数据用途",
				})
				return err
			},
		},
	}

	violations := 0
	for _, scenario := range scenarios {
		ctx, cancel := context.WithCancel(context.Background())
		repository := &cancelAfterReplayRepository{aggregate: scenario.aggregate, cancel: cancel}
		service := archive.NewService(repository, func() time.Time { return fixedTime }, fixedIDs{})
		err := scenario.invoke(service, ctx)
		cancel()
		if !errors.Is(err, context.Canceled) || repository.commits != 0 {
			violations++
			t.Logf("%s: err=%v commits=%d", scenario.name, err, repository.commits)
		}
	}
	if violations != 0 {
		t.Fatalf("%d canceled mutations committed after request cancellation", violations)
	}
}

func commandMeta(role, key string) archive.CommandMeta {
	return archive.CommandMeta{
		IdempotencyKey: key, ExpectedVersion: 4, Actor: "context-tester", Role: role,
		Reason: "验证请求取消传播", CorrelationID: "correlation-" + key,
	}
}

func aggregateFor(state observatory.TaskState, revisions map[string]observatory.DatasetRevision) observatory.Aggregate {
	if revisions == nil {
		revisions = map[string]observatory.DatasetRevision{}
	}
	return observatory.Aggregate{
		Task:      observatory.ArchiveTask{ID: taskID, State: state, Version: 4, CreatedAt: fixedTime, UpdatedAt: fixedTime},
		Revisions: revisions, Findings: map[string]observatory.ValidationFinding{},
	}
}

func validRevisions() map[string]observatory.DatasetRevision {
	return map[string]observatory.DatasetRevision{
		"rev_good": {
			ID: "rev_good", TaskID: taskID, LogicalPath: "raw/context.fits", ByteSize: 2880,
			MediaType: "application/fits", SHA256: strings.Repeat("b", 64), SubmittedBy: "data-admin", SubmittedAt: fixedTime,
		},
	}
}

func quarantinedAggregate(status observatory.FindingStatus) observatory.Aggregate {
	revisions := validRevisions()
	revisions["rev_bad"] = observatory.DatasetRevision{
		ID: "rev_bad", TaskID: taskID, LogicalPath: "raw/context.fits", ByteSize: 1,
		MediaType: "application/fits", SHA256: strings.Repeat("c", 64), SubmittedBy: "data-admin", SubmittedAt: fixedTime,
	}
	replacement := revisions["rev_good"]
	replacement.SupersedesRevisionID = "rev_bad"
	revisions["rev_good"] = replacement
	aggregate := aggregateFor(observatory.StateQuarantined, revisions)
	aggregate.Findings["finding_blocking"] = observatory.ValidationFinding{
		ID: "finding_blocking", TaskID: taskID, RevisionID: "rev_bad", RuleCode: "MINIMUM_PAYLOAD_SIZE",
		Severity: observatory.SeverityBlocking, Status: status, ReplacementID: "rev_good", ResolutionNote: "替代修订已经完成校验",
	}
	return aggregate
}

func frozenAggregate() observatory.Aggregate {
	aggregate := aggregateFor(observatory.StateFrozen, validRevisions())
	entry := observatory.ManifestEntry{
		RevisionID: "rev_good", LogicalPath: "raw/context.fits", ByteSize: 2880,
		MediaType: "application/fits", SHA256: strings.Repeat("b", 64),
	}
	entry.EntryDigest = observatory.ManifestEntryDigest(entry)
	aggregate.Manifest = &observatory.FrozenManifest{
		ID: "manifest_context", TaskID: taskID, TaskVersion: 4, Entries: []observatory.ManifestEntry{entry},
		MerkleRoot: observatory.MerkleRoot([]observatory.ManifestEntry{entry}), FrozenBy: "release-lead", FrozenAt: fixedTime,
	}
	return aggregate
}
