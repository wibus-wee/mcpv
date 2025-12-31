//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"

	"mcpd/internal/domain"
)

var CatalogProviderSet = wire.NewSet(
	NewDynamicCatalogProvider,
	wire.Bind(new(domain.CatalogProvider), new(*DynamicCatalogProvider)),
)

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

var ReloadableAppSet = wire.NewSet(
	CatalogProviderSet,
	NewCatalogState,
	NewScheduler,
	NewServerInitializationManager,
	NewProfileRuntimes,
	NewControlPlaneState,
	newCallerRegistry,
	newDiscoveryService,
	newObservabilityService,
	newAutomationService,
	NewControlPlane,
	NewRPCServer,
	NewReloadManager,
	wire.Bind(new(domain.ControlPlane), new(*ControlPlane)),
)

var AppSet = wire.NewSet(
	CoreInfraSet,
	ReloadableAppSet,
	NewApplication,
)
