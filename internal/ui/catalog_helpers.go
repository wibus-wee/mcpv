package ui

import (
	"errors"

	"mcpv/internal/infra/catalog"
)

func mapCatalogError(err error) error {
	if err == nil {
		return nil
	}
	var editorErr *catalog.EditorError
	if errors.As(err, &editorErr) {
		detail := ""
		if editorErr.Err != nil {
			detail = editorErr.Err.Error()
		}
		switch editorErr.Kind {
		case catalog.EditorErrorInvalidRequest:
			return NewErrorWithDetails(ErrCodeInvalidRequest, editorErr.Message, detail)
		case catalog.EditorErrorInvalidConfig:
			return NewErrorWithDetails(ErrCodeInvalidConfig, editorErr.Message, detail)
		default:
			return NewErrorWithDetails(ErrCodeInvalidConfig, editorErr.Message, detail)
		}
	}
	return NewErrorWithDetails(ErrCodeInvalidConfig, "Failed to update configuration", err.Error())
}
