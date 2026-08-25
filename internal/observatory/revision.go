package observatory

import (
	"encoding/hex"
	"mime"
	"path"
	"strings"
	"time"
)

type NewRevision struct {
	ID                   string
	LogicalPath          string
	ByteSize             int64
	MediaType            string
	SHA256               string
	SupersedesRevisionID string
	SubmittedBy          string
	Now                  time.Time
}

func RegisterRevision(aggregate Aggregate, input NewRevision) (DatasetRevision, error) {
	if err := EnsureState(aggregate.Task, StateDraft, StateCollecting, StateQuarantined); err != nil {
		return DatasetRevision{}, err
	}
	input.ID = strings.TrimSpace(input.ID)
	input.LogicalPath = strings.TrimSpace(input.LogicalPath)
	input.MediaType = strings.ToLower(strings.TrimSpace(input.MediaType))
	input.SHA256 = strings.ToLower(strings.TrimSpace(input.SHA256))
	input.SubmittedBy = strings.TrimSpace(input.SubmittedBy)
	input.SupersedesRevisionID = strings.TrimSpace(input.SupersedesRevisionID)
	if input.ID == "" || input.SubmittedBy == "" {
		return DatasetRevision{}, invalid("INVALID_REVISION", "修订标识和提交人不能为空")
	}
	if _, exists := aggregate.Revisions[input.ID]; exists {
		return DatasetRevision{}, invalid("REVISION_EXISTS", "修订 %s 已存在", input.ID)
	}
	if err := validateLogicalPath(input.LogicalPath); err != nil {
		return DatasetRevision{}, err
	}
	if input.ByteSize <= 0 {
		return DatasetRevision{}, invalid("INVALID_BYTE_SIZE", "数据集字节数必须大于零")
	}
	if input.ByteSize > 1<<50 {
		return DatasetRevision{}, invalid("INVALID_BYTE_SIZE", "单个数据集声明大小超过服务上限")
	}
	if input.MediaType == "" || mime.TypeByExtension(path.Ext(input.LogicalPath)) == "" && !strings.Contains(input.MediaType, "/") {
		return DatasetRevision{}, invalid("INVALID_MEDIA_TYPE", "媒体类型必须使用 type/subtype 格式")
	}
	if _, err := hex.DecodeString(input.SHA256); err != nil || len(input.SHA256) != 64 {
		return DatasetRevision{}, invalid("INVALID_SHA256", "SHA-256 摘要必须为 64 位十六进制字符串")
	}
	if input.SupersedesRevisionID == input.ID {
		return DatasetRevision{}, invalid("SELF_SUPERSEDES", "修订不能替代自身")
	}
	if input.SupersedesRevisionID != "" {
		parent, ok := aggregate.Revisions[input.SupersedesRevisionID]
		if !ok {
			return DatasetRevision{}, invalid("SUPERSEDED_NOT_FOUND", "被替代修订不存在")
		}
		if parent.LogicalPath != input.LogicalPath {
			return DatasetRevision{}, invalid("SUPERSEDES_PATH_MISMATCH", "替代修订必须保持相同逻辑路径")
		}
		for _, existing := range aggregate.Revisions {
			if existing.SupersedesRevisionID == input.SupersedesRevisionID {
				return DatasetRevision{}, invalid("SUPERSEDES_FORK", "同一修订不能产生多个替代分支")
			}
		}
	} else {
		for _, existing := range ActiveRevisions(aggregate) {
			if existing.LogicalPath == input.LogicalPath {
				return DatasetRevision{}, invalid("ACTIVE_PATH_CONFLICT", "逻辑路径已有活动修订，必须声明替代关系")
			}
		}
	}
	return DatasetRevision{
		ID: input.ID, TaskID: aggregate.Task.ID, LogicalPath: input.LogicalPath,
		ByteSize: input.ByteSize, MediaType: input.MediaType, SHA256: input.SHA256,
		SupersedesRevisionID: input.SupersedesRevisionID, SubmittedBy: input.SubmittedBy,
		SubmittedAt: input.Now.UTC(),
	}, nil
}

func validateLogicalPath(value string) error {
	if value == "" || len(value) > 512 {
		return invalid("INVALID_LOGICAL_PATH", "逻辑路径不能为空且不能超过 512 字节")
	}
	if strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "\\") {
		return invalid("INVALID_LOGICAL_PATH", "逻辑路径必须是相对 POSIX 文件路径")
	}
	if path.Clean(value) != value || value == "." {
		return invalid("INVALID_LOGICAL_PATH", "逻辑路径不能包含空段、点段或上级目录")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return invalid("INVALID_LOGICAL_PATH", "逻辑路径包含非法分段")
		}
	}
	return nil
}

func ActiveRevisions(aggregate Aggregate) []DatasetRevision {
	superseded := make(map[string]bool)
	for _, revision := range aggregate.Revisions {
		if revision.SupersedesRevisionID != "" {
			superseded[revision.SupersedesRevisionID] = true
		}
	}
	active := make([]DatasetRevision, 0, len(aggregate.Revisions))
	for id, revision := range aggregate.Revisions {
		if !superseded[id] {
			active = append(active, revision)
		}
	}
	return active
}
