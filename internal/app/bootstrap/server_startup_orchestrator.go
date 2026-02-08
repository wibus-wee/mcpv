package bootstrap

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap/metadata"
	"mcpv/internal/app/bootstrap/serverinit"
	"mcpv/internal/domain"
)

// ServerStartupOrchestrator coordinates bootstrap metadata fetches and server init.
type ServerStartupOrchestrator struct {
	initManager      *serverinit.Manager
	bootstrapManager *metadata.Manager
	logger           *zap.Logger
}

// NewServerStartupOrchestrator constructs a startup orchestrator.
func NewServerStartupOrchestrator(
	initManager *serverinit.Manager,
	bootstrapManager *metadata.Manager,
	logger *zap.Logger,
) *ServerStartupOrchestrator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ServerStartupOrchestrator{
		initManager:      initManager,
		bootstrapManager: bootstrapManager,
		logger:           logger.Named("startup"),
	}
}

// Bootstrap starts the async bootstrap process if configured.
func (o *ServerStartupOrchestrator) Bootstrap(ctx context.Context) {
	if o == nil || o.bootstrapManager == nil {
		return
	}
	o.bootstrapManager.Bootstrap(ctx)
}

// StartInit begins background initialization work if configured.
func (o *ServerStartupOrchestrator) StartInit(ctx context.Context) {
	if o == nil || o.initManager == nil {
		return
	}
	o.initManager.Start(ctx)
}

// StopInit stops background initialization work.
func (o *ServerStartupOrchestrator) StopInit() {
	if o == nil || o.initManager == nil {
		return
	}
	o.initManager.Stop()
}

// ApplyCatalogState updates the init manager with a new catalog state.
func (o *ServerStartupOrchestrator) ApplyCatalogState(state *domain.CatalogState) {
	if o == nil || o.initManager == nil {
		return
	}
	o.initManager.ApplyCatalogState(state)
}

// SetMinReady updates min-ready settings for a spec via init manager.
func (o *ServerStartupOrchestrator) SetMinReady(specKey string, minReady int, cause domain.StartCause) error {
	if o == nil || o.initManager == nil {
		return errors.New("server initialization manager not configured")
	}
	return o.initManager.SetMinReady(specKey, minReady, cause)
}

// RetryInit requests a retry for a spec initialization.
func (o *ServerStartupOrchestrator) RetryInit(specKey string) error {
	if o == nil || o.initManager == nil {
		return errors.New("server initialization manager not configured")
	}
	return o.initManager.RetrySpec(specKey)
}

// InitStatuses returns the current init status snapshot.
func (o *ServerStartupOrchestrator) InitStatuses(ctx context.Context) []domain.ServerInitStatus {
	if o == nil || o.initManager == nil {
		return nil
	}
	return o.initManager.Statuses(ctx)
}

// BootstrapProgress returns the current bootstrap progress.
func (o *ServerStartupOrchestrator) BootstrapProgress() domain.BootstrapProgress {
	if o == nil || o.bootstrapManager == nil {
		return domain.BootstrapProgress{State: domain.BootstrapCompleted}
	}
	return o.bootstrapManager.GetProgress()
}

// WaitForBootstrap blocks until bootstrap completes or context cancels.
func (o *ServerStartupOrchestrator) WaitForBootstrap(ctx context.Context) error {
	if o == nil || o.bootstrapManager == nil {
		return nil
	}
	return o.bootstrapManager.WaitForCompletion(ctx)
}

// HasBootstrap reports whether bootstrap is configured.
func (o *ServerStartupOrchestrator) HasBootstrap() bool {
	if o == nil {
		return false
	}
	return o.bootstrapManager != nil
}
