package controlplane

import (
	"mcpv/internal/app/controlplane/automation"
	"mcpv/internal/app/controlplane/discovery"
	"mcpv/internal/app/controlplane/observability"
	"mcpv/internal/app/controlplane/registry"
	"mcpv/internal/infra/telemetry"
)

type ClientRegistry = registry.ClientRegistry

type ToolDiscoveryService = discovery.ToolDiscoveryService

type ResourceDiscoveryService = discovery.ResourceDiscoveryService

type PromptDiscoveryService = discovery.PromptDiscoveryService

type ObservabilityService = observability.Service

type AutomationService = automation.Service

func NewClientRegistry(state *State) *ClientRegistry {
	return registry.NewClientRegistry(state)
}

func NewToolDiscoveryService(state *State, registry *ClientRegistry) *ToolDiscoveryService {
	return discovery.NewToolDiscoveryService(state, registry)
}

func NewResourceDiscoveryService(state *State, registry *ClientRegistry) *ResourceDiscoveryService {
	return discovery.NewResourceDiscoveryService(state, registry)
}

func NewPromptDiscoveryService(state *State, registry *ClientRegistry) *PromptDiscoveryService {
	return discovery.NewPromptDiscoveryService(state, registry)
}

func NewObservabilityService(state *State, registry *ClientRegistry, logs *telemetry.LogBroadcaster) *ObservabilityService {
	return observability.NewObservabilityService(state, registry, logs)
}

func NewAutomationService(state *State, registry *ClientRegistry, tools *ToolDiscoveryService) *AutomationService {
	return automation.NewAutomationService(state, registry, tools)
}
