package persistence

import "fmt"

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func errf(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ErrorCode(err error) string {
	if typed, ok := err.(*Error); ok {
		return typed.Code
	}
	return "PERSISTENCE_ERROR"
}
