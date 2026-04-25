package errs

import (
	"errors"
	"strings"
)

const (
	CodeInvalidArgument    = "invalid_argument"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeFailedPrecondition = "failed_precondition"
	CodeInternal           = "internal"
)

type CodedError struct {
	code    string
	message string
	cause   error
}

func (e *CodedError) Error() string {
	if strings.TrimSpace(e.message) != "" {
		return e.message
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.code
}

func (e *CodedError) Unwrap() error {
	return e.cause
}

func (e *CodedError) Code() string {
	return e.code
}

func New(code, message string) error {
	return &CodedError{
		code:    strings.TrimSpace(code),
		message: strings.TrimSpace(message),
	}
}

func Wrap(code, message string, cause error) error {
	return &CodedError{
		code:    strings.TrimSpace(code),
		message: strings.TrimSpace(message),
		cause:   cause,
	}
}

func InvalidArgument(message string) error {
	return New(CodeInvalidArgument, message)
}

func NotFound(message string) error {
	return New(CodeNotFound, message)
}

func Conflict(message string) error {
	return New(CodeConflict, message)
}

func FailedPrecondition(message string) error {
	return New(CodeFailedPrecondition, message)
}

func Internal(message string) error {
	return New(CodeInternal, message)
}

func Required(field string) error {
	field = strings.TrimSpace(field)
	if field == "" {
		return InvalidArgument("value is required")
	}
	return InvalidArgument(field + " is required")
}

func JoinInvalid(messages []string) error {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}
		parts = append(parts, message)
	}
	if len(parts) == 0 {
		return nil
	}
	return InvalidArgument(strings.Join(parts, "; "))
}

func Code(err error) string {
	if err == nil {
		return ""
	}

	var coded interface{ Code() string }
	if errors.As(err, &coded) {
		return strings.TrimSpace(coded.Code())
	}

	return ""
}

func HasCode(err error, code string) bool {
	return Code(err) == strings.TrimSpace(code)
}
