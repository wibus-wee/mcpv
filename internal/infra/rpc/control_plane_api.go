package rpc

import (
	"context"

	"mcpv/internal/domain"
	pluginmanager "mcpv/internal/infra/plugin/manager"
)

type ControlPlaneAPI interface {
	domain.ControlPlaneCoreAPI
	domain.StoreAPI
	domain.AutomationAPI
	domain.TasksAPI
	ConfigPathAPI
	ReloadConfigAPI
	PluginStatusAPI
}

type ConfigPathAPI interface {
	ConfigPath() string
}

type ReloadConfigAPI interface {
	ReloadConfig(ctx context.Context) error
}

type PluginStatusAPI interface {
	PluginStatus(specs []domain.PluginSpec) []pluginmanager.Status
}
