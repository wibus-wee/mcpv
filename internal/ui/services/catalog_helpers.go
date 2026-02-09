package services

import (
	"errors"

	catalogeditor "mcpv/internal/infra/catalog/editor"
	"mcpv/internal/ui"
)

func mapCatalogError(err error) error {
	if err == nil {
		return nil
	}
	var editorErr *catalogeditor.Error
	if errors.As(err, &editorErr) {
		detail := ""
		if editorErr.Err != nil {
			detail = editorErr.Err.Error()
		}
		switch editorErr.Kind {
		case catalogeditor.ErrorInvalidRequest:
			return ui.NewErrorWithDetails(ui.ErrCodeInvalidRequest, editorErr.Message, detail)
		case catalogeditor.ErrorInvalidConfig:
			return ui.NewErrorWithDetails(ui.ErrCodeInvalidConfig, editorErr.Message, detail)
		default:
			return ui.NewErrorWithDetails(ui.ErrCodeInvalidConfig, editorErr.Message, detail)
		}
	}
	return ui.NewErrorWithDetails(ui.ErrCodeInvalidConfig, "Failed to update configuration", err.Error())
}
