package observatory

import "fmt"

type Violation struct {
	Code    string
	Message string
}

func (e *Violation) Error() string { return e.Message }

func invalid(code, format string, args ...any) error {
	return &Violation{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ViolationCode(err error) string {
	if violation, ok := err.(*Violation); ok {
		return violation.Code
	}
	return "DOMAIN_ERROR"
}
