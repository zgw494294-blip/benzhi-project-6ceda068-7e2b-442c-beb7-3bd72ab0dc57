package httpapi

import (
	"context"
	"errors"
	"net/http"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
)

type requestError struct {
	status  int
	code    string
	message string
}

func (e *requestError) Error() string { return e.message }

func mapError(err error) (int, string, string, any) {
	var request *requestError
	if errors.As(err, &request) {
		return request.status, request.code, request.message, nil
	}
	// Surface client cancellation as a recognizable status so observers can
	// distinguish it from internal failures; the request produced no side
	// effects after the idempotency query completed.
	if errors.Is(err, context.Canceled) {
		return 499, "CLIENT_CLOSED", "请求已被客户端取消", nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusRequestTimeout, "CONTEXT_DEADLINE_EXCEEDED", "请求处理超时", nil
	}
	normalized := archive.NormalizeError(err)
	status := http.StatusUnprocessableEntity
	switch normalized.Code {
	case "TASK_NOT_FOUND", "FINDING_NOT_FOUND", "REPLACEMENT_NOT_FOUND", "SUPERSEDED_NOT_FOUND":
		status = http.StatusNotFound
	case "VERSION_CONFLICT", "INVALID_STATE", "IDEMPOTENCY_KEY_REUSED", "REVISION_EXISTS", "ACTIVE_PATH_CONFLICT", "SUPERSEDES_FORK":
		status = http.StatusConflict
	case "ROLE_FORBIDDEN":
		status = http.StatusForbidden
	case "INTERNAL_ERROR", "PERSISTENCE_ERROR", "STORE_CLOSED", "EVENT_APPEND_FAILED", "EVENT_SYNC_FAILED", "SNAPSHOT_WRITE_FAILED", "SNAPSHOT_SYNC_FAILED":
		status = http.StatusInternalServerError
	}
	return status, normalized.Code, normalized.Message, normalized.Details
}
