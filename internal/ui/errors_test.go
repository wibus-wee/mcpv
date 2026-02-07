package ui

import (
	"errors"
	"testing"

	"mcpv/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestErrorStringFormatsDetails(t *testing.T) {
	uiErr := &Error{Code: "CODE", Message: "message", Details: "details"}
	if got := uiErr.Error(); got != "CODE: message (details)" {
		t.Fatalf("unexpected error string: %s", got)
	}

	uiErr = &Error{Code: "CODE", Message: "message"}
	if got := uiErr.Error(); got != "CODE: message" {
		t.Fatalf("unexpected error string without details: %s", got)
	}
}

func TestMapDomainError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "tool", err: domain.ErrToolNotFound, code: ErrCodeToolNotFound},
		{name: "resource", err: domain.ErrResourceNotFound, code: ErrCodeResourceNotFound},
		{name: "prompt", err: domain.ErrPromptNotFound, code: ErrCodePromptNotFound},
		{name: "cursor", err: domain.ErrInvalidCursor, code: ErrCodeInvalidCursor},
		{name: "client", err: domain.ErrClientNotRegistered, code: ErrCodeInternal},
		{name: "default", err: errors.New("boom"), code: ErrCodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uiErr := MapDomainError(tt.err)
			require.NotNil(t, uiErr)
			if uiErr.Code != tt.code {
				t.Fatalf("expected code %s, got %s", tt.code, uiErr.Code)
			}
		})
	}
}
