package transfer

import (
	"errors"
	"strings"

	"mcpv/internal/domain"
)

// Source identifies a transfer configuration source.
type Source string

const (
	// SourceClaude identifies the Claude config source.
	SourceClaude Source = "claude"
	// SourceCodex identifies the Codex config source.
	SourceCodex Source = "codex"
	// SourceGemini identifies the Gemini config source.
	SourceGemini Source = "gemini"
)

const (
	// IssueInvalid indicates an invalid entry in the source config.
	IssueInvalid = "invalid"
	// IssueDuplicate indicates a duplicate entry in the source config.
	IssueDuplicate = "duplicate"
)

var (
	// ErrNotFound indicates the source config file is missing.
	ErrNotFound = errors.New("transfer source config not found")
	// ErrUnknownSource indicates the source string is not supported.
	ErrUnknownSource = errors.New("unknown transfer source")
)

// Issue describes a parsing or validation issue.
type Issue struct {
	Name    string
	Kind    string
	Message string
}

// Result holds parsed servers and issues for a transfer source.
type Result struct {
	Source  Source
	Path    string
	Servers []domain.ServerSpec
	Issues  []Issue
}

// ParseSource converts a raw string into a Source.
func ParseSource(raw string) (Source, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(SourceClaude):
		return SourceClaude, nil
	case string(SourceCodex):
		return SourceCodex, nil
	case string(SourceGemini):
		return SourceGemini, nil
	default:
		return "", ErrUnknownSource
	}
}
