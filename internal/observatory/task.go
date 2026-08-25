package observatory

import (
	"strings"
	"time"
)

type NewTask struct {
	ID               string
	Title            string
	InstrumentCode   string
	ObservationStart time.Time
	ObservationEnd   time.Time
	Owner            string
	Now              time.Time
}

func CreateTask(input NewTask) (Aggregate, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Title = strings.TrimSpace(input.Title)
	input.InstrumentCode = strings.ToUpper(strings.TrimSpace(input.InstrumentCode))
	input.Owner = strings.TrimSpace(input.Owner)
	if input.ID == "" {
		return Aggregate{}, invalid("INVALID_TASK_ID", "任务标识不能为空")
	}
	if len([]rune(input.Title)) < 2 || len([]rune(input.Title)) > 120 {
		return Aggregate{}, invalid("INVALID_TITLE", "任务标题长度必须为 2 到 120 个字符")
	}
	if len(input.InstrumentCode) < 2 || len(input.InstrumentCode) > 32 {
		return Aggregate{}, invalid("INVALID_INSTRUMENT", "仪器代码长度必须为 2 到 32 个字符")
	}
	for _, ch := range input.InstrumentCode {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return Aggregate{}, invalid("INVALID_INSTRUMENT", "仪器代码只能包含大写字母、数字、连字符或下划线")
		}
	}
	if input.Owner == "" {
		return Aggregate{}, invalid("INVALID_OWNER", "负责人不能为空")
	}
	if input.ObservationStart.IsZero() || input.ObservationEnd.IsZero() {
		return Aggregate{}, invalid("INVALID_OBSERVATION_WINDOW", "观测时间窗不能为空")
	}
	if !input.ObservationEnd.After(input.ObservationStart) {
		return Aggregate{}, invalid("INVALID_OBSERVATION_WINDOW", "观测结束时间必须晚于开始时间")
	}
	if input.ObservationEnd.Sub(input.ObservationStart) > 31*24*time.Hour {
		return Aggregate{}, invalid("INVALID_OBSERVATION_WINDOW", "单次任务观测时间窗不能超过 31 天")
	}
	now := input.Now.UTC()
	task := ArchiveTask{
		ID: input.ID, Title: input.Title, InstrumentCode: input.InstrumentCode,
		ObservationStart: input.ObservationStart.UTC(), ObservationEnd: input.ObservationEnd.UTC(),
		Owner: input.Owner, State: StateDraft, Version: 0, CreatedAt: now, UpdatedAt: now,
	}
	return Aggregate{Task: task, Revisions: map[string]DatasetRevision{}, Findings: map[string]ValidationFinding{}}, nil
}

func EnsureState(task ArchiveTask, allowed ...TaskState) error {
	for _, state := range allowed {
		if task.State == state {
			return nil
		}
	}
	return invalid("INVALID_STATE", "任务状态 %s 不允许执行该操作", task.State)
}

func HasOpenBlockingFindings(aggregate Aggregate) bool {
	for _, finding := range aggregate.Findings {
		if finding.Severity == SeverityBlocking && finding.Status != FindingAccepted {
			return true
		}
	}
	return false
}

func AllBlockingAccepted(aggregate Aggregate) bool {
	found := false
	for _, finding := range aggregate.Findings {
		if finding.Severity != SeverityBlocking {
			continue
		}
		found = true
		if finding.Status != FindingAccepted {
			return false
		}
	}
	return found
}
