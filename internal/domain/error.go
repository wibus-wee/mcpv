package domain

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeInvalidArgument  ErrorCode = "INVALID_ARGUMENT"
	CodeNotFound         ErrorCode = "NOT_FOUND"
	CodeUnavailable      ErrorCode = "UNAVAILABLE"
	CodeFailedPrecond    ErrorCode = "FAILED_PRECONDITION"
	CodePermissionDenied ErrorCode = "PERMISSION_DENIED"
	CodeUnauthenticated  ErrorCode = "UNAUTHENTICATED"
	CodeInternal         ErrorCode = "INTERNAL"
	CodeCanceled         ErrorCode = "CANCELED"
	CodeDeadlineExceeded ErrorCode = "DEADLINE_EXCEEDED"
	CodeNotImplemented   ErrorCode = "NOT_IMPLEMENTED"
)

type Error struct {
	Code      ErrorCode
	Op        string
	Message   string
	Cause     error
	Retryable bool
	Meta      map[string]string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	msg := e.Message
	if msg == "" && e.Cause != nil {
		msg = e.Cause.Error()
	}
	if e.Op == "" {
		if msg == "" {
			return string(e.Code)
		}
		return fmt.Sprintf("%s: %s", e.Code, msg)
	}
	if msg == "" {
		return fmt.Sprintf("%s: %s", e.Op, e.Code)
	}
	return fmt.Sprintf("%s: %s: %s", e.Op, e.Code, msg)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func E(code ErrorCode, op, msg string, cause error) *Error {
	if msg == "" && cause != nil {
		msg = cause.Error()
	}
	return &Error{
		Code:    code,
		Op:      op,
		Message: msg,
		Cause:   cause,
	}
}

func Wrap(code ErrorCode, op string, err error) *Error {
	if err == nil {
		return nil
	}
	var existing *Error
	if errors.As(err, &existing) {
		if existing.Op != "" || op == "" {
			return existing
		}
		return &Error{
			Code:      existing.Code,
			Op:        op,
			Message:   existing.Message,
			Cause:     existing.Cause,
			Retryable: existing.Retryable,
			Meta:      existing.Meta,
		}
	}
	return E(code, op, "", err)
}

func CodeFrom(err error) (ErrorCode, bool) {
	if err == nil {
		return "", false
	}
	var domainErr *Error
	if errors.As(err, &domainErr) && domainErr.Code != "" {
		return domainErr.Code, true
	}
	switch {
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, ErrMethodNotAllowed), errors.Is(err, ErrInvalidCursor):
		return CodeInvalidArgument, true
	case errors.Is(err, ErrUnknownSpecKey):
		return CodeInvalidArgument, true
	case errors.Is(err, ErrToolNotFound), errors.Is(err, ErrResourceNotFound), errors.Is(err, ErrPromptNotFound), errors.Is(err, ErrTaskNotFound):
		return CodeNotFound, true
	case errors.Is(err, ErrTasksNotImplemented):
		return CodeNotImplemented, true
	case errors.Is(err, ErrClientNotRegistered):
		return CodeFailedPrecond, true
	case errors.Is(err, ErrNoReadyInstance):
		return CodeUnavailable, true
	case errors.Is(err, ErrConnectionClosed):
		return CodeUnavailable, true
	case errors.Is(err, ErrUnsupportedProtocol), errors.Is(err, ErrInvalidCommand), errors.Is(err, ErrExecutableNotFound):
		return CodeFailedPrecond, true
	case errors.Is(err, ErrPermissionDenied):
		return CodePermissionDenied, true
	default:
		return "", false
	}
}
