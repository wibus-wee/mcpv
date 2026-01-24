package ui

import (
	"errors"

	"mcpd/internal/infra/catalog"
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
			return NewUIErrorWithDetails(ErrCodeInvalidRequest, editorErr.Message, detail)
		case catalog.EditorErrorInvalidConfig:
			return NewUIErrorWithDetails(ErrCodeInvalidConfig, editorErr.Message, detail)
		default:
			return NewUIErrorWithDetails(ErrCodeInvalidConfig, editorErr.Message, detail)
		}
	}
	return NewUIErrorWithDetails(ErrCodeInvalidConfig, "Failed to update configuration", err.Error())
}
