//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"

	"mcpd/internal/domain"
)

var CatalogAccessorSet = wire.NewSet(
	NewStaticCatalogAccessor,
	wire.Bind(new(domain.CatalogAccessor), new(*StaticCatalogAccessor)),
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
	CatalogAccessorSet,
	NewCatalogSnapshot,
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
	wire.Bind(new(domain.ControlPlane), new(*ControlPlane)),
)

var AppSet = wire.NewSet(
	CoreInfraSet,
	ReloadableAppSet,
	NewApplication,
)
