package archive

import (
	"context"
	"errors"
	"fmt"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	cause   error
}

func (e *Error) Error() string { return e.Message }

// Unwrap preserves identity for errors.Is/errors.As so that callers can still
// recognize wrapped sentinel errors such as context.Canceled.
func (e *Error) Unwrap() error { return e.cause }

func fail(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// canceled reports the request's cancellation cause when present, so callers
// can return an identifiable context.Canceled / context.DeadlineExceeded.
func canceled(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func NormalizeError(err error) *Error {
	if err == nil {
		return nil
	}
	var application *Error
	if errors.As(err, &application) {
		return application
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Code: "CONTEXT_CANCELED", Message: "请求已被客户端取消", cause: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: "CONTEXT_DEADLINE_EXCEEDED", Message: "请求处理超时", cause: err}
	}
	var domain *observatory.Violation
	if errors.As(err, &domain) {
		return &Error{Code: domain.Code, Message: domain.Message}
	}
	var stored *persistence.Error
	if errors.As(err, &stored) {
		return &Error{Code: stored.Code, Message: stored.Message}
	}
	return &Error{Code: "INTERNAL_ERROR", Message: "服务处理请求时发生内部错误"}
}
