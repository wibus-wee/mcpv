package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/catalog/normalizer"
	"mcpv/internal/ui"
)

// PluginService exposes plugin management APIs.
type PluginService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewPluginService(deps *ServiceDeps) *PluginService {
	return &PluginService{
		deps:   deps,
		logger: deps.loggerNamed("plugin-service"),
	}
}

// GetPluginList returns all configured plugins with their current state and metrics.
func (s *PluginService) GetPluginList(_ context.Context) ([]PluginListEntry, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	catalog := cp.GetCatalog()
	if len(catalog.Plugins) == 0 {
		return []PluginListEntry{}, nil
	}

	// Get plugin runtime status from coreApp
	var statusMap map[string]bool
	coreApp, coreErr := s.deps.getCoreApp()
	if coreErr == nil && coreApp != nil {
		statusList := coreApp.GetPluginStatus()
		statusMap = make(map[string]bool, len(statusList))
		for _, st := range statusList {
			statusMap[st.Name] = st.Running
		}
	}

	plugins := make([]PluginListEntry, 0, len(catalog.Plugins))
	for _, spec := range catalog.Plugins {
		// Get latest metrics from telemetry (placeholder - implement when telemetry exposes plugin metrics)
		metrics := PluginMetrics{
			CallCount:      0,
			RejectionCount: 0,
			AvgLatencyMs:   0.0,
		}

		enabled := !spec.Disabled

		// Determine status
		status := "stopped"
		statusError := ""
		if !enabled {
			status = "stopped"
			statusError = ""
		} else if statusMap != nil {
			if running, ok := statusMap[spec.Name]; ok && running {
				status = "running"
			} else {
				status = "error"
				statusError = "Plugin failed to start or is not running"
			}
		}

		entry := PluginListEntry{
			Name:               spec.Name,
			Category:           string(spec.Category),
			Flows:              mapPluginFlows(spec.Flows),
			Required:           spec.Required,
			Enabled:            enabled,
			Status:             status,
			StatusError:        statusError,
			CommitHash:         spec.CommitHash,
			TimeoutMs:          spec.TimeoutMs,
			HandshakeTimeoutMs: spec.HandshakeTimeoutMs,
			Cmd:                spec.Cmd,
			Env:                spec.Env,
			Cwd:                spec.Cwd,
			ConfigJSON:         string(spec.ConfigJSON), // Convert json.RawMessage to string
			LatestMetrics:      metrics,
		}
		plugins = append(plugins, entry)
	}

	return plugins, nil
}

// CreatePlugin adds a plugin to the config file.
func (s *PluginService) CreatePlugin(ctx context.Context, req CreatePluginRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	spec, err := mapPluginSpecDetailToDomain(req.Spec)
	if err != nil {
		return ui.NewError(ui.ErrCodeInvalidRequest, err.Error())
	}
	if err := editor.CreatePlugin(ctx, spec); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// UpdatePlugin updates an existing plugin in the config file.
func (s *PluginService) UpdatePlugin(ctx context.Context, req UpdatePluginRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	spec, err := mapPluginSpecDetailToDomain(req.Spec)
	if err != nil {
		return ui.NewError(ui.ErrCodeInvalidRequest, err.Error())
	}
	if err := editor.UpdatePlugin(ctx, spec); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// DeletePlugin removes a plugin from the config file.
func (s *PluginService) DeletePlugin(ctx context.Context, req DeletePluginRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ui.NewError(ui.ErrCodeInvalidRequest, "Plugin name is required")
	}
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.DeletePlugin(ctx, name); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// TogglePlugin enables or disables a plugin by triggering a config reload.
func (s *PluginService) TogglePlugin(ctx context.Context, req TogglePluginRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ui.NewError(ui.ErrCodeInvalidRequest, "Plugin name is required")
	}

	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}

	cp, err := s.deps.getControlPlane()
	if err != nil {
		return err
	}
	found := false
	for _, plugin := range cp.GetCatalog().Plugins {
		if plugin.Name == name {
			found = true
			break
		}
	}
	if !found {
		return ui.NewError(ui.ErrCodeNotFound, fmt.Sprintf("Plugin %q not found", name))
	}

	s.logger.Info("Plugin toggle requested",
		zap.String("plugin", name),
		zap.Bool("enabled", req.Enabled),
	)

	if err := editor.SetPluginDisabled(ctx, name, !req.Enabled); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// GetPluginMetrics returns aggregated metrics for all plugins.
func (s *PluginService) GetPluginMetrics(_ context.Context) (map[string]PluginMetrics, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	catalog := cp.GetCatalog()
	metrics := make(map[string]PluginMetrics, len(catalog.Plugins))

	// TODO: Query telemetry for actual plugin metrics
	// For now, return empty metrics
	for _, plugin := range catalog.Plugins {
		metrics[plugin.Name] = PluginMetrics{
			CallCount:      0,
			RejectionCount: 0,
			AvgLatencyMs:   0.0,
		}
	}

	return metrics, nil
}

// mapPluginFlows converts domain.PluginFlow to string slice.
func mapPluginFlows(flows []domain.PluginFlow) []string {
	result := make([]string, len(flows))
	for i, flow := range flows {
		result[i] = string(flow)
	}
	return result
}

func mapPluginSpecDetailToDomain(spec PluginSpecDetail) (domain.PluginSpec, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return domain.PluginSpec{}, fmt.Errorf("plugin name is required")
	}

	category, ok := domain.NormalizePluginCategory(spec.Category)
	if !ok {
		return domain.PluginSpec{}, fmt.Errorf("plugin category is invalid")
	}

	cmd := make([]string, 0, len(spec.Cmd))
	for _, entry := range spec.Cmd {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		cmd = append(cmd, trimmed)
	}
	if len(cmd) == 0 {
		return domain.PluginSpec{}, fmt.Errorf("plugin cmd is required")
	}

	flows, ok := domain.NormalizePluginFlows(spec.Flows)
	if !ok {
		return domain.PluginSpec{}, fmt.Errorf("plugin flows must contain request and/or response")
	}

	var configJSON json.RawMessage
	if strings.TrimSpace(spec.ConfigJSON) != "" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(spec.ConfigJSON), &parsed); err != nil {
			return domain.PluginSpec{}, fmt.Errorf("plugin config must be valid JSON object: %v", err)
		}
		encoded, err := json.Marshal(parsed)
		if err != nil {
			return domain.PluginSpec{}, fmt.Errorf("plugin config must be valid JSON object: %v", err)
		}
		configJSON = encoded
	}

	pluginFlows := make([]domain.PluginFlow, 0, len(flows))
	pluginFlows = append(pluginFlows, flows...)

	return domain.PluginSpec{
		Name:               name,
		Category:           category,
		Required:           spec.Required,
		Disabled:           spec.Disabled,
		Cmd:                cmd,
		Env:                normalizer.NormalizeEnvMap(spec.Env),
		Cwd:                strings.TrimSpace(spec.Cwd),
		CommitHash:         strings.TrimSpace(spec.CommitHash),
		TimeoutMs:          spec.TimeoutMs,
		HandshakeTimeoutMs: spec.HandshakeTimeoutMs,
		ConfigJSON:         configJSON,
		Flows:              pluginFlows,
	}, nil
}
