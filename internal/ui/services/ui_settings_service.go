package services

import (
	"context"
	"encoding/json"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/ui"
	"mcpv/internal/ui/uiconfig"
)

// UISettingsService exposes UI settings APIs for the frontend.
type UISettingsService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewUISettingsService(deps *ServiceDeps) *UISettingsService {
	return &UISettingsService{
		deps:   deps,
		logger: deps.loggerNamed("ui-settings-service"),
	}
}

// GetUISettings returns UI settings for the requested scope.
func (s *UISettingsService) GetUISettings(ctx context.Context, req UISettingsScopeRequest) (UISettingsSnapshot, error) {
	_ = ctx
	store, err := s.deps.uiSettingsStore()
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	scope, err := parseScope(req.Scope)
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if scope == uiconfig.ScopeWorkspace && workspaceID == "" {
		workspaceID, _, err = s.resolveWorkspaceID()
		if err != nil {
			return UISettingsSnapshot{}, err
		}
	}
	snapshot, err := store.Get(scope, workspaceID)
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	return mapSnapshot(snapshot), nil
}

// GetEffectiveUISettings returns merged UI settings for global + workspace scope.
func (s *UISettingsService) GetEffectiveUISettings(ctx context.Context, req UISettingsEffectiveRequest) (UISettingsSnapshot, error) {
	_ = ctx
	store, err := s.deps.uiSettingsStore()
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		workspaceID, _, err = s.resolveWorkspaceID()
		if err != nil {
			return UISettingsSnapshot{}, err
		}
	}
	snapshot, err := store.GetEffective(workspaceID)
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	return mapSnapshot(snapshot), nil
}

// UpdateUISettings applies partial updates to UI settings.
func (s *UISettingsService) UpdateUISettings(ctx context.Context, req UpdateUISettingsRequest) (UISettingsSnapshot, error) {
	_ = ctx
	store, err := s.deps.uiSettingsStore()
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	scope, err := parseScope(req.Scope)
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if scope == uiconfig.ScopeWorkspace && workspaceID == "" {
		workspaceID, _, err = s.resolveWorkspaceID()
		if err != nil {
			return UISettingsSnapshot{}, err
		}
	}
	updates := normalizeUpdates(req.Updates)
	snapshot, err := store.Update(scope, workspaceID, updates, req.Removes)
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	return mapSnapshot(snapshot), nil
}

// ResetUISettings clears UI settings for the requested scope.
func (s *UISettingsService) ResetUISettings(ctx context.Context, req ResetUISettingsRequest) (UISettingsSnapshot, error) {
	_ = ctx
	store, err := s.deps.uiSettingsStore()
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	scope, err := parseScope(req.Scope)
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if scope == uiconfig.ScopeWorkspace && workspaceID == "" {
		workspaceID, _, err = s.resolveWorkspaceID()
		if err != nil {
			return UISettingsSnapshot{}, err
		}
	}
	snapshot, err := store.Reset(scope, workspaceID)
	if err != nil {
		return UISettingsSnapshot{}, err
	}
	return mapSnapshot(snapshot), nil
}

// GetWorkspaceID returns the workspace identifier derived from the config path.
func (s *UISettingsService) GetWorkspaceID(ctx context.Context) (UISettingsWorkspaceIDResponse, error) {
	_ = ctx
	workspaceID, configPath, err := s.resolveWorkspaceID()
	if err != nil {
		return UISettingsWorkspaceIDResponse{}, err
	}
	return UISettingsWorkspaceIDResponse{
		WorkspaceID: workspaceID,
		ConfigPath:  configPath,
	}, nil
}

func (s *UISettingsService) resolveWorkspaceID() (string, string, error) {
	manager := s.deps.manager()
	if manager == nil {
		return "", "", ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	configPath := strings.TrimSpace(manager.GetConfigPath())
	if configPath == "" {
		return "", "", ui.NewError(ui.ErrCodeInvalidConfig, "Configuration path is not available")
	}
	return uiconfig.WorkspaceIDForPath(configPath), configPath, nil
}

func parseScope(value string) (uiconfig.Scope, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return uiconfig.ScopeGlobal, nil
	}
	switch normalized {
	case string(uiconfig.ScopeGlobal):
		return uiconfig.ScopeGlobal, nil
	case string(uiconfig.ScopeWorkspace):
		return uiconfig.ScopeWorkspace, nil
	default:
		return "", ui.NewError(ui.ErrCodeInvalidRequest, "Invalid scope")
	}
}

func normalizeUpdates(raw map[string]json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return map[string]json.RawMessage{}
	}
	out := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		out[key] = append([]byte(nil), value...)
	}
	return out
}

func mapSnapshot(snapshot uiconfig.Snapshot) UISettingsSnapshot {
	sections := make(map[string]json.RawMessage, len(snapshot.Sections))
	for key, value := range snapshot.Sections {
		sections[key] = append([]byte(nil), value...)
	}
	return UISettingsSnapshot{
		Scope:       string(snapshot.Scope),
		WorkspaceID: snapshot.WorkspaceID,
		Version:     snapshot.Version,
		UpdatedAt:   snapshot.UpdatedAt,
		Sections:    sections,
	}
}
