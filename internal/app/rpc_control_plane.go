package app

import (
	"context"
	"strings"

	"mcpv/internal/app/controlplane"
	"mcpv/internal/domain"
	pluginmanager "mcpv/internal/infra/plugin/manager"
)

// RPCControlPlane adapts core services for the RPC control surface.
type RPCControlPlane struct {
	*controlplane.ControlPlane
	configPath    string
	reloadManager *controlplane.ReloadManager
	pluginManager *pluginmanager.Manager
}

func NewRPCControlPlane(
	control *controlplane.ControlPlane,
	configPath string,
	reloadManager *controlplane.ReloadManager,
	pluginManager *pluginmanager.Manager,
) *RPCControlPlane {
	return &RPCControlPlane{
		ControlPlane:  control,
		configPath:    strings.TrimSpace(configPath),
		reloadManager: reloadManager,
		pluginManager: pluginManager,
	}
}

func (c *RPCControlPlane) ConfigPath() string {
	if c == nil {
		return ""
	}
	return c.configPath
}

func (c *RPCControlPlane) ReloadConfig(ctx context.Context) error {
	if c == nil || c.reloadManager == nil {
		return domain.E(domain.CodeFailedPrecond, "reload config", "reload manager unavailable", nil)
	}
	return c.reloadManager.Reload(ctx)
}

func (c *RPCControlPlane) PluginStatus(specs []domain.PluginSpec) []pluginmanager.Status {
	if c == nil || c.pluginManager == nil {
		return nil
	}
	return c.pluginManager.GetStatus(specs)
}
