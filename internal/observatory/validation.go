package observatory

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type FindingIDFactory func(ruleCode, revisionID string) string

func ValidateCandidates(aggregate Aggregate, factory FindingIDFactory) ([]ValidationFinding, error) {
	if err := EnsureState(aggregate.Task, StateCollecting, StateQuarantined, StateReviewPending); err != nil {
		return nil, err
	}
	active := ActiveRevisions(aggregate)
	if len(active) == 0 {
		return nil, invalid("NO_ACTIVE_REVISIONS", "任务没有可校验的活动修订")
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].LogicalPath == active[j].LogicalPath {
			return active[i].ID < active[j].ID
		}
		return active[i].LogicalPath < active[j].LogicalPath
	})
	findings := make([]ValidationFinding, 0)
	seenDigest := map[string]string{}
	for _, revision := range active {
		add := func(code string, severity FindingSeverity, message string) {
			findings = append(findings, ValidationFinding{
				ID: factory(code, revision.ID), TaskID: aggregate.Task.ID, RevisionID: revision.ID,
				RuleCode: code, Severity: severity, Message: message, Status: FindingOpen,
			})
		}
		if revision.ByteSize < 2880 {
			add("MINIMUM_PAYLOAD_SIZE", SeverityBlocking, "观测数据小于最小有效载荷 2880 字节")
		}
		extension := strings.ToLower(path.Ext(revision.LogicalPath))
		switch extension {
		case ".fits", ".fit", ".fts":
			if revision.ByteSize%2880 != 0 {
				add("FITS_BLOCK_ALIGNMENT", SeverityBlocking, "FITS 文件大小不是 2880 字节块的整数倍")
			}
			if revision.MediaType != "application/fits" && revision.MediaType != "image/fits" {
				add("FITS_MEDIA_TYPE", SeverityWarning, "FITS 扩展名建议使用 application/fits 或 image/fits")
			}
		case ".json":
			if revision.MediaType != "application/json" {
				add("JSON_MEDIA_TYPE", SeverityWarning, "JSON 元数据应使用 application/json")
			}
		case ".csv":
			if revision.MediaType != "text/csv" {
				add("CSV_MEDIA_TYPE", SeverityWarning, "CSV 元数据应使用 text/csv")
			}
		default:
			add("UNRECOGNIZED_EXTENSION", SeverityWarning, fmt.Sprintf("未识别的数据扩展名 %s", extension))
		}
		if previous, exists := seenDigest[revision.SHA256]; exists {
			add("DUPLICATE_CONTENT", SeverityWarning, "内容摘要与活动修订 "+previous+" 相同")
		} else {
			seenDigest[revision.SHA256] = revision.ID
		}
	}
	return findings, nil
}

func FindingsBlock(findings []ValidationFinding) bool {
	for _, finding := range findings {
		if finding.Severity == SeverityBlocking {
			return true
		}
	}
	return false
}
