package observatory

import (
	"strings"
	"testing"
	"time"
)

func TestQuarantineResolutionFreezeAndVerify(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	aggregate, err := CreateTask(NewTask{
		ID: "task_test", Title: "系外行星凌星观测", InstrumentCode: "CCD-01",
		ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour), Owner: "owner", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	bad, err := RegisterRevision(aggregate, NewRevision{
		ID: "rev_bad", LogicalPath: "science/frame.fits", ByteSize: 3000,
		MediaType: "application/fits", SHA256: strings.Repeat("a", 64), SubmittedBy: "admin", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Revisions[bad.ID] = bad
	aggregate.Task.State = StateCollecting
	findings, err := ValidateCandidates(aggregate, func(rule, revision string) string { return rule + "_" + revision })
	if err != nil || !FindingsBlock(findings) {
		t.Fatalf("应产生阻断问题：%v, %#v", err, findings)
	}
	for _, finding := range findings {
		aggregate.Findings[finding.ID] = finding
	}
	aggregate.Task.State = StateQuarantined
	replacement, err := RegisterRevision(aggregate, NewRevision{
		ID: "rev_good", LogicalPath: bad.LogicalPath, ByteSize: 5760,
		MediaType: "application/fits", SHA256: strings.Repeat("b", 64),
		SupersedesRevisionID: bad.ID, SubmittedBy: "admin", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Revisions[replacement.ID] = replacement
	var blocking ValidationFinding
	for _, finding := range findings {
		if finding.Severity == SeverityBlocking {
			blocking = finding
			break
		}
	}
	proposed, err := ProposeResolution(aggregate, blocking.ID, replacement.ID, "使用块对齐的重新导出文件替代")
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Findings[proposed.ID] = proposed
	reviewed, err := ReviewResolution(aggregate, proposed.ID, "reviewer", true, "", now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Findings[reviewed.ID] = reviewed
	aggregate.Task.State = StateReviewPending
	aggregate.Task.Version = 5
	manifest, err := BuildManifest(aggregate, "manifest_test", "lead", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 1 || manifest.Entries[0].RevisionID != replacement.ID {
		t.Fatalf("清单未仅包含活动替代修订：%#v", manifest.Entries)
	}
	aggregate.Manifest = &manifest
	aggregate.Task.State = StateFrozen
	credential, err := IssueCredential(aggregate, "credential_test", "lead", "公开科研复核使用", now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Credential = &credential
	aggregate.Task.State = StateReleased
	if result := VerifyRelease(aggregate); !result.Valid {
		t.Fatalf("正常凭据应有效：%v", result.Reasons)
	}
	aggregate.Manifest.Entries[0].ByteSize++
	if result := VerifyRelease(aggregate); result.Valid {
		t.Fatal("篡改清单后验证不应通过")
	}
}

func TestRevisionRejectsForkAndUnsafePath(t *testing.T) {
	now := time.Now().UTC()
	aggregate, _ := CreateTask(NewTask{ID: "task", Title: "测试任务", InstrumentCode: "CAM-1", ObservationStart: now.Add(-2 * time.Hour), ObservationEnd: now.Add(-time.Hour), Owner: "owner", Now: now})
	if _, err := RegisterRevision(aggregate, NewRevision{ID: "bad", LogicalPath: "../escape.fits", ByteSize: 2880, MediaType: "application/fits", SHA256: strings.Repeat("a", 64), SubmittedBy: "admin", Now: now}); err == nil {
		t.Fatal("应拒绝上级目录路径")
	}
}
