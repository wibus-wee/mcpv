package ui

import (
	"errors"
	"fmt"

	"mcpd/internal/domain"
)

// UIError represents a frontend-friendly error with a code
type UIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *UIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Error codes for frontend handling
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
	ErrCodeInternal           = "INTERNAL_ERROR"
)

// MapDomainError converts domain errors to UIError
func MapDomainError(err error) *UIError {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrToolNotFound):
		return NewUIError(ErrCodeToolNotFound, "Tool not found")
	case errors.Is(err, domain.ErrResourceNotFound):
		return NewUIError(ErrCodeResourceNotFound, "Resource not found")
	case errors.Is(err, domain.ErrPromptNotFound):
		return NewUIError(ErrCodePromptNotFound, "Prompt not found")
	case errors.Is(err, domain.ErrInvalidCursor):
		return NewUIError(ErrCodeInvalidCursor, "Invalid pagination cursor")
	case errors.Is(err, domain.ErrClientNotRegistered):
		// This shouldn't happen in UI layer, but handle it gracefully
		return NewUIError(ErrCodeInternal, "Internal error: client not registered")
	default:
		return NewUIErrorWithDetails(ErrCodeInternal, "Internal error", err.Error())
	}
}

// NewUIError creates a new UIError with code and message
func NewUIError(code, message string) *UIError {
	return &UIError{
		Code:    code,
		Message: message,
	}
}

// NewUIErrorWithDetails creates a new UIError with code, message, and details
func NewUIErrorWithDetails(code, message, details string) *UIError {
	return &UIError{
		Code:    code,
		Message: message,
		Details: details,
	}
}
