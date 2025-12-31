package app

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
)

type controlPlaneState struct {
	info         domain.ControlPlaneInfo
	profiles     map[string]*profileRuntime
	callers      map[string]string
	specRegistry map[string]domain.ServerSpec
	scheduler    domain.Scheduler
	initManager  *ServerInitializationManager
	runtime      domain.RuntimeConfig
	logger       *zap.Logger
	ctx          context.Context
	profileStore domain.ProfileStore
}

func newControlPlaneState(
	ctx context.Context,
	profiles map[string]*profileRuntime,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	store domain.ProfileStore,
	summary profileSummary,
	logger *zap.Logger,
) *controlPlaneState {
	if logger == nil {
		logger = zap.NewNop()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callers := store.Callers
	if callers == nil {
		callers = map[string]string{}
	}

	return &controlPlaneState{
		info:         defaultControlPlaneInfo(),
		profiles:     profiles,
		callers:      callers,
		specRegistry: summary.specRegistry,
		scheduler:    scheduler,
		initManager:  initManager,
		runtime:      summary.defaultRuntime,
		profileStore: store,
		logger:       logger.Named("control_plane"),
		ctx:          ctx,
	}
}

type callerState struct {
	pid           int
	profile       string
	lastHeartbeat time.Time
}

type profileRuntime struct {
	name      string
	specKeys  []string
	tools     *aggregator.ToolIndex
	resources *aggregator.ResourceIndex
	prompts   *aggregator.PromptIndex

	mu     sync.Mutex
	active bool
}

func (p *profileRuntime) Activate(ctx context.Context) {
	p.mu.Lock()
	if p.active {
		p.mu.Unlock()
		return
	}
	p.active = true
	p.mu.Unlock()

	if p.tools != nil {
		p.tools.Start(ctx)
	}
	if p.resources != nil {
		p.resources.Start(ctx)
	}
	if p.prompts != nil {
		p.prompts.Start(ctx)
	}
}

func (p *profileRuntime) Deactivate() {
	p.mu.Lock()
	if !p.active {
		p.mu.Unlock()
		return
	}
	p.active = false
	p.mu.Unlock()

	if p.tools != nil {
		p.tools.Stop()
	}
	if p.resources != nil {
		p.resources.Stop()
	}
	if p.prompts != nil {
		p.prompts.Stop()
	}
}

func defaultControlPlaneInfo() domain.ControlPlaneInfo {
	return domain.ControlPlaneInfo{
		Name:    "mcpd",
		Version: Version,
		Build:   Build,
	}
}
