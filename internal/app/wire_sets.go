//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/app/bootstrap/serverinit"
	appCatalog "mcpv/internal/app/catalog"
	"mcpv/internal/app/controlplane"
	"mcpv/internal/domain"
	"mcpv/internal/infra/rpc"
)

// CatalogProviderSet wires catalog providers for dependency injection.
var CatalogProviderSet = wire.NewSet(
	ConfigPath,
	appCatalog.NewDynamicCatalogProvider,
	wire.Bind(new(domain.CatalogProvider), new(*appCatalog.DynamicCatalogProvider)),
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
	NewSamplingHandler,
	NewElicitationHandler,
	NewPluginManager,
	NewMCPTransport,
	NewLifecycleManager,
	NewPingProbe,
)

// ReloadableAppSet wires reloadable application dependencies.
var ReloadableAppSet = wire.NewSet(
	CatalogProviderSet,
	appCatalog.NewCatalogState,
	NewScheduler,
	domain.NewMetadataCache,
	NewBootstrapManagerProvider,
	serverinit.NewManager,
	bootstrap.NewServerStartupOrchestrator,
	newRuntimeState,
	provideControlPlaneState,
	NewPipelineEngine,
	NewGovernanceExecutor,
	controlplane.NewClientRegistry,
	controlplane.NewToolDiscoveryService,
	controlplane.NewResourceDiscoveryService,
	controlplane.NewPromptDiscoveryService,
	controlplane.NewObservabilityService,
	controlplane.NewAutomationService,
	controlplane.NewControlPlane,
	NewRPCServer,
	controlplane.NewReloadManager,
	wire.Bind(new(controlplane.API), new(*controlplane.ControlPlane)),
	wire.Bind(new(rpc.ControlPlaneAPI), new(*controlplane.ControlPlane)),
)

// AppSet wires the full application dependency set.
var AppSet = wire.NewSet(
	CoreInfraSet,
	ReloadableAppSet,
	wire.Struct(new(ApplicationOptions), "*"),
	NewApplication,
)
