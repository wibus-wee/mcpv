package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry"
)

var (
	// ErrUnknownSpecKey indicates the spec key does not exist.
	ErrUnknownSpecKey = domain.ErrUnknownSpecKey
	// ErrNoCapacity indicates no instance capacity is available.
	ErrNoCapacity = errors.New("no capacity available")
	// ErrStickyBusy indicates a sticky instance is busy.
	ErrStickyBusy = errors.New("sticky instance at capacity")
	// ErrNotImplemented indicates the scheduler is not available.
	ErrNotImplemented = errors.New("scheduler not implemented")
)

// Options configures the basic scheduler.
type Options struct {
	Probe        domain.HealthProbe
	PingInterval time.Duration
	Logger       *zap.Logger
	Metrics      domain.Metrics
	Health       *telemetry.HealthTracker
}

// BasicScheduler orchestrates instance lifecycle and routing policies.
type BasicScheduler struct {
	lifecycle domain.Lifecycle
	specsMu   sync.RWMutex
	specs     map[string]domain.ServerSpec

	poolsMu sync.RWMutex
	pools   map[string]*poolState

	probe   domain.HealthProbe
	logger  *zap.Logger
	metrics domain.Metrics
	health  *telemetry.HealthTracker

	mu         sync.Mutex
	idleTicker *time.Ticker
	stopIdle   chan struct{}
	pingTicker *time.Ticker
	stopPing   chan struct{}

	idleBeat *telemetry.Heartbeat
	pingBeat *telemetry.Heartbeat
}

type trackedInstance struct {
	instance       *domain.Instance
	drainOnce      sync.Once
	drainDone      chan struct{}
	drainCloseOnce sync.Once
}

func (t *trackedInstance) closeDrainDone() {
	if t == nil || t.drainDone == nil {
		return
	}
	t.drainCloseOnce.Do(func() {
		close(t.drainDone)
	})
}

type stickyBinding struct {
	inst       *trackedInstance
	lastAccess time.Time
}

type poolState struct {
	mu            sync.Mutex
	spec          domain.ServerSpec
	specKey       string
	minReady      int
	starting      int
	startCount    int
	stopCount     int
	startInFlight bool
	startCancel   context.CancelFunc
	generation    uint64
	signalSeq     uint64
	instances     []*trackedInstance
	draining      []*trackedInstance
	sticky        map[string]*stickyBinding
	rrIndex       int
	waitCond      *sync.Cond
	waiters       int
}

type stopCandidate struct {
	specKey string
	state   *poolState
	inst    *trackedInstance
	reason  string
}

type poolEntry struct {
	specKey string
	state   *poolState
}

// NewBasicScheduler constructs a BasicScheduler using the provided options.
func NewBasicScheduler(lifecycle domain.Lifecycle, specs map[string]domain.ServerSpec, opts Options) (*BasicScheduler, error) {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BasicScheduler{
		lifecycle: lifecycle,
		specs:     cloneSpecRegistry(specs),
		pools:     make(map[string]*poolState),
		probe:     opts.Probe,
		logger:    logger.Named("scheduler"),
		metrics:   opts.Metrics,
		health:    opts.Health,
		stopIdle:  make(chan struct{}),
		stopPing:  make(chan struct{}),
	}, nil
}
