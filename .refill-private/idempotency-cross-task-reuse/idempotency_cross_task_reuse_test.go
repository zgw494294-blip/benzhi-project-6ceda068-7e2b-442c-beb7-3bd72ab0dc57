package idempotency_cross_task_reuse_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

type sequenceIDs struct{ next atomic.Uint64 }

func (g *sequenceIDs) New(prefix string) string {
	return prefix + "-private-" + time.Unix(int64(g.next.Add(1)), 0).UTC().Format("150405")
}

func TestIdempotencyKeyCannotBeReusedAcrossTasks(t *testing.T) {
	repository, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	service := archive.NewService(repository, func() time.Time { return now }, &sequenceIDs{})
	create := func(key, title string) string {
		result, createErr := service.CreateTask(context.Background(), archive.CreateTaskCommand{
			IdempotencyKey: key, ExpectedVersion: 0, Actor: "admin", Role: archive.RoleAdministrator,
			Reason: "建立隔离测试任务", CorrelationID: "private-" + key, Title: title,
			InstrumentCode: "CAM-1", ObservationStart: now.Add(-2 * time.Hour),
			ObservationEnd: now.Add(-time.Hour), Owner: "owner",
		})
		if createErr != nil {
			t.Fatalf("创建任务失败：%v", createErr)
		}
		return result.Task.ID
	}
	taskA := create("create-a", "幂等隔离任务甲")
	taskB := create("create-b", "幂等隔离任务乙")

	register := func(taskID, path, digest string) error {
		_, registerErr := service.RegisterRevision(context.Background(), taskID, archive.RegisterRevisionCommand{
			CommandMeta: archive.CommandMeta{IdempotencyKey: "shared-revision-key", ExpectedVersion: 1,
				Actor: "admin", Role: archive.RoleAdministrator, Reason: "登记观测修订", CorrelationID: "private-register"},
			LogicalPath: path, ByteSize: 5760, MediaType: "application/fits", SHA256: strings.Repeat(digest, 64),
		})
		return registerErr
	}
	if err := register(taskA, "science/a.fits", "a"); err != nil {
		t.Fatalf("首次使用幂等键失败：%v", err)
	}
	if err := register(taskB, "science/b.fits", "b"); err == nil || archive.NormalizeError(err).Code != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("cross-task idempotency reuse was accepted: %v", err)
	}
}
