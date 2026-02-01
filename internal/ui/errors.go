package ui

import (
	"errors"
	"fmt"

	"mcpv/internal/domain"
)

// Error represents a frontend-friendly error with a code.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Error codes for frontend handling.
const (
	ErrCodeCoreNotRunning     = "CORE_NOT_RUNNING"
	ErrCodeCoreAlreadyRunning = "CORE_ALREADY_RUNNING"
	ErrCodeCoreFailed         = "CORE_FAILED"
	ErrCodeInvalidState       = "INVALID_STATE"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeToolNotFound       = "TOOL_NOT_FOUND"
	ErrCodeResourceNotFound   = "RESOURCE_NOT_FOUND"
	ErrCodePromptNotFound     = "PROMPT_NOT_FOUND"
	ErrCodeInvalidCursor      = "INVALID_CURSOR"
	ErrCodeInvalidConfig      = "INVALID_CONFIG"
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
	ErrCodeOperationCancelled = "OPERATION_CANCELLED"
	ErrCodeNotImplemented     = "NOT_IMPLEMENTED"
	ErrCodeInternal           = "INTERNAL_ERROR"
)

// MapDomainError converts domain errors to Error.
func MapDomainError(err error) *Error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrToolNotFound):
		return NewError(ErrCodeToolNotFound, "Tool not found")
	case errors.Is(err, domain.ErrResourceNotFound):
		return NewError(ErrCodeResourceNotFound, "Resource not found")
	case errors.Is(err, domain.ErrPromptNotFound):
		return NewError(ErrCodePromptNotFound, "Prompt not found")
	case errors.Is(err, domain.ErrInvalidCursor):
		return NewError(ErrCodeInvalidCursor, "Invalid pagination cursor")
	case errors.Is(err, domain.ErrClientNotRegistered):
		// This shouldn't happen in UI layer, but handle it gracefully.
		return NewError(ErrCodeInternal, "Internal error: client not registered")
	default:
		return NewErrorWithDetails(ErrCodeInternal, "Internal error", err.Error())
	}
}

// NewError creates a new Error with code and message.
func NewError(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithDetails creates a new Error with code, message, and details.
func NewErrorWithDetails(code, message, details string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: details,
	}
}
