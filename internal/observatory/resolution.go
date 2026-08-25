package observatory

import (
	"strings"
	"time"
)

func ProposeResolution(aggregate Aggregate, findingID, replacementID, note string) (ValidationFinding, error) {
	if err := EnsureState(aggregate.Task, StateQuarantined); err != nil {
		return ValidationFinding{}, err
	}
	finding, ok := aggregate.Findings[findingID]
	if !ok {
		return ValidationFinding{}, invalid("FINDING_NOT_FOUND", "校验问题不存在")
	}
	if finding.Severity != SeverityBlocking {
		return ValidationFinding{}, invalid("FINDING_NOT_BLOCKING", "仅阻断问题需要提交处置")
	}
	if finding.Status == FindingAccepted {
		return ValidationFinding{}, invalid("FINDING_ALREADY_ACCEPTED", "校验问题已审核接受")
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) < 4 || len([]rune(note)) > 1000 {
		return ValidationFinding{}, invalid("INVALID_RESOLUTION_NOTE", "处置说明长度必须为 4 到 1000 个字符")
	}
	replacementID = strings.TrimSpace(replacementID)
	if replacementID == "" {
		return ValidationFinding{}, invalid("REPLACEMENT_REQUIRED", "阻断问题处置必须提供替代修订")
	}
	replacement, exists := aggregate.Revisions[replacementID]
	if !exists {
		return ValidationFinding{}, invalid("REPLACEMENT_NOT_FOUND", "替代修订不存在")
	}
	if replacement.SupersedesRevisionID != finding.RevisionID {
		return ValidationFinding{}, invalid("INVALID_REPLACEMENT", "替代修订没有直接替代问题修订")
	}
	finding.Status = FindingProposed
	finding.ResolutionNote = note
	finding.ReplacementID = replacementID
	finding.ReviewedBy = ""
	finding.ReviewedAt = nil
	return finding, nil
}

func ReviewResolution(aggregate Aggregate, findingID, reviewer string, accepted bool, note string, now time.Time) (ValidationFinding, error) {
	if err := EnsureState(aggregate.Task, StateQuarantined); err != nil {
		return ValidationFinding{}, err
	}
	finding, ok := aggregate.Findings[findingID]
	if !ok {
		return ValidationFinding{}, invalid("FINDING_NOT_FOUND", "校验问题不存在")
	}
	if finding.Status != FindingProposed {
		return ValidationFinding{}, invalid("RESOLUTION_NOT_PROPOSED", "问题尚无待审核处置")
	}
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" {
		return ValidationFinding{}, invalid("INVALID_REVIEWER", "审核员不能为空")
	}
	if !accepted && len([]rune(strings.TrimSpace(note))) < 4 {
		return ValidationFinding{}, invalid("RETURN_NOTE_REQUIRED", "退回时必须提供至少 4 个字符的理由")
	}
	if accepted {
		validation, err := ValidateCandidates(aggregate, func(ruleCode, revisionID string) string { return ruleCode + ":" + revisionID })
		if err != nil {
			return ValidationFinding{}, err
		}
		for _, candidate := range validation {
			if candidate.RevisionID == finding.ReplacementID && candidate.Severity == SeverityBlocking {
				return ValidationFinding{}, invalid("REPLACEMENT_VALIDATION_FAILED", "替代修订仍触发阻断规则 %s", candidate.RuleCode)
			}
		}
		finding.Status = FindingAccepted
	} else {
		finding.Status = FindingReturned
		finding.ResolutionNote = finding.ResolutionNote + "\n审核退回：" + strings.TrimSpace(note)
	}
	finding.ReviewedBy = reviewer
	reviewedAt := now.UTC()
	finding.ReviewedAt = &reviewedAt
	return finding, nil
}
