package services

import (
	"os"
	"testing"

	"go.uber.org/zap"

	"mcpv/internal/ui"
)

func TestServiceDepsLoggerNamed(t *testing.T) {
	logger := zap.NewNop()
	deps := NewServiceDeps(nil, logger)

	base := deps.loggerNamed(" ")
	if base != logger {
		t.Fatal("expected blank name to return base logger")
	}

	named := deps.loggerNamed("child")
	if named == logger {
		t.Fatal("expected named logger to be a different instance")
	}
}

func TestServiceDepsGetControlPlaneWithoutManager(t *testing.T) {
	deps := NewServiceDeps(nil, zap.NewNop())

	_, err := deps.getControlPlane()
	if err == nil {
		t.Fatal("expected error when manager is nil")
	}
	uiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if uiErr.Code != ui.ErrCodeInternal {
		t.Fatalf("expected %s, got %s", ui.ErrCodeInternal, uiErr.Code)
	}
}

func TestServiceDepsCatalogEditorWithConfigPath(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("failed to close temp config file: %v", err)
	}

	manager := ui.NewManager(nil, nil, tempFile.Name())
	deps := NewServiceDeps(nil, zap.NewNop())
	deps.setManager(manager)

	editor, err := deps.catalogEditor()
	if err != nil {
		t.Fatalf("expected catalog editor, got error: %v", err)
	}
	if editor == nil {
		t.Fatal("expected non-nil catalog editor")
	}
}
