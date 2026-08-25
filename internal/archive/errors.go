package archive

import (
	"errors"
	"fmt"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/observatory"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func fail(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func NormalizeError(err error) *Error {
	if err == nil {
		return nil
	}
	var application *Error
	if errors.As(err, &application) {
		return application
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
