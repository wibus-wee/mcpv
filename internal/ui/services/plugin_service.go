package services

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/domain"
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

		// Determine status
		status := "stopped"
		statusError := ""
		if statusMap != nil {
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
			Enabled:            true, // TODO: track enabled state in catalog or separate store
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

// TogglePlugin enables or disables a plugin by triggering a config reload.
func (s *PluginService) TogglePlugin(_ context.Context, req TogglePluginRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ui.NewError(ui.ErrCodeInvalidRequest, "Plugin name is required")
	}

	cp, err := s.deps.getControlPlane()
	if err != nil {
		return err
	}

	catalog := cp.GetCatalog()
	found := false
	for _, plugin := range catalog.Plugins {
		if plugin.Name == name {
			found = true
			break
		}
	}

	if !found {
		return ui.NewError(ui.ErrCodeNotFound, fmt.Sprintf("Plugin %q not found", name))
	}

	// TODO: Implement plugin enable/disable state tracking
	// For now, we'll need to modify the catalog and trigger reload
	// This requires extending the catalog format to support enabled/disabled state
	// or implementing a separate plugin state store

	s.logger.Info("Plugin toggle requested",
		zap.String("plugin", name),
		zap.Bool("enabled", req.Enabled),
	)

	return ui.NewError(ui.ErrCodeNotImplemented, "Plugin enable/disable not yet implemented")
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
