//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"

	"mcpd/internal/domain"
)

// CatalogProviderSet wires catalog providers for dependency injection.
var CatalogProviderSet = wire.NewSet(
	NewDynamicCatalogProvider,
	wire.Bind(new(domain.CatalogProvider), new(*DynamicCatalogProvider)),
)

// CoreInfraSet wires core infrastructure dependencies.
var CoreInfraSet = wire.NewSet(
	NewLogging,
	NewLogger,
	NewLogBroadcaster,
	NewMetricsRegistry,
	NewMetrics,
	NewHealthTracker,
	NewListChangeHub,
	NewCommandLauncher,
	NewMCPTransport,
	NewLifecycleManager,
	NewPingProbe,
)

// ReloadableAppSet wires reloadable application dependencies.
var ReloadableAppSet = wire.NewSet(
	CatalogProviderSet,
	NewCatalogState,
	NewScheduler,
	domain.NewMetadataCache,
	NewBootstrapManagerProvider,
	NewServerInitializationManager,
	NewRuntimeState,
	NewControlPlaneState,
	newClientRegistry,
	newDiscoveryService,
	newObservabilityService,
	newAutomationService,
	NewControlPlane,
	NewRPCServer,
	NewReloadManager,
	wire.Bind(new(domain.ControlPlane), new(*ControlPlane)),
)

// AppSet wires the full application dependency set.
var AppSet = wire.NewSet(
	CoreInfraSet,
	ReloadableAppSet,
	NewApplication,
)
