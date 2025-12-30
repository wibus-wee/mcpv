package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
	"mcpd/internal/infra/telemetry"
)

const (
	runtimeStatusRefreshInterval = 500 * time.Millisecond
	serverInitRefreshInterval    = time.Second
)

type observabilityService struct {
	state    *controlPlaneState
	registry *callerRegistry
	logs     *telemetry.LogBroadcaster

	runtimeStatusIdx *aggregator.RuntimeStatusIndex
	serverInitIdx    *aggregator.ServerInitIndex
}

func newObservabilityService(state *controlPlaneState, registry *callerRegistry, logs *telemetry.LogBroadcaster) *observabilityService {
	return &observabilityService{
		state:    state,
		registry: registry,
		logs:     logs,
	}
}

func (o *observabilityService) StreamLogs(ctx context.Context, caller string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	if _, err := o.registry.resolveProfile(caller); err != nil {
		return closedLogEntryChannel(), err
	}
	return o.streamLogs(ctx, minLevel)
}

func (o *observabilityService) StreamLogsAllProfiles(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return o.streamLogs(ctx, minLevel)
}

func (o *observabilityService) streamLogs(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	if o.logs == nil {
		return closedLogEntryChannel(), nil
	}
	if minLevel == "" {
		minLevel = domain.LogLevelDebug
	}
	source := o.logs.Subscribe(ctx)
	out := make(chan domain.LogEntry, telemetry.DefaultLogBufferSize)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case entry, ok := <-source:
				if !ok {
					return
				}
				if compareLogLevel(entry.Level, minLevel) < 0 {
					continue
				}
				select {
				case out <- entry:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

func (o *observabilityService) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	if o.state.scheduler == nil {
		return nil, nil
	}
	return o.state.scheduler.GetPoolStatus(ctx)
}

func (o *observabilityService) GetServerInitStatus(ctx context.Context) ([]domain.ServerInitStatus, error) {
	if o.state.initManager == nil {
		return nil, nil
	}
	return o.state.initManager.Statuses(), nil
}

func (o *observabilityService) WatchRuntimeStatus(ctx context.Context, caller string) (<-chan domain.RuntimeStatusSnapshot, error) {
	if _, err := o.registry.resolveProfile(caller); err != nil {
		return closedRuntimeStatusChannel(), err
	}
	if o.runtimeStatusIdx == nil {
		return closedRuntimeStatusChannel(), nil
	}
	return o.runtimeStatusIdx.Subscribe(ctx), nil
}

func (o *observabilityService) WatchRuntimeStatusAllProfiles(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	if o.runtimeStatusIdx == nil {
		return closedRuntimeStatusChannel(), nil
	}
	return o.runtimeStatusIdx.Subscribe(ctx), nil
}

func (o *observabilityService) WatchServerInitStatus(ctx context.Context, caller string) (<-chan domain.ServerInitStatusSnapshot, error) {
	if _, err := o.registry.resolveProfile(caller); err != nil {
		return closedServerInitStatusChannel(), err
	}
	if o.serverInitIdx == nil {
		return closedServerInitStatusChannel(), nil
	}
	return o.serverInitIdx.Subscribe(ctx), nil
}

func (o *observabilityService) WatchServerInitStatusAllProfiles(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	if o.serverInitIdx == nil {
		return closedServerInitStatusChannel(), nil
	}
	return o.serverInitIdx.Subscribe(ctx), nil
}

func (o *observabilityService) SetRuntimeStatusIndex(idx *aggregator.RuntimeStatusIndex) {
	o.runtimeStatusIdx = idx
	if idx != nil {
		go o.runRuntimeStatusWorker()
	}
}

func (o *observabilityService) SetServerInitIndex(idx *aggregator.ServerInitIndex) {
	o.serverInitIdx = idx
	if idx != nil {
		go o.runServerInitWorker()
	}
}

func (o *observabilityService) runRuntimeStatusWorker() {
	ticker := time.NewTicker(runtimeStatusRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.state.ctx.Done():
			return
		case <-ticker.C:
			if err := o.runtimeStatusIdx.Refresh(o.state.ctx); err != nil {
				o.state.logger.Warn("runtime status refresh failed", zap.Error(err))
			}
		}
	}
}

func (o *observabilityService) runServerInitWorker() {
	ticker := time.NewTicker(serverInitRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.state.ctx.Done():
			return
		case <-ticker.C:
			if err := o.serverInitIdx.Refresh(o.state.ctx); err != nil {
				o.state.logger.Warn("server init status refresh failed", zap.Error(err))
			}
		}
	}
}

func compareLogLevel(a, b domain.LogLevel) int {
	ar := logLevelRank(a)
	br := logLevelRank(b)
	switch {
	case ar < br:
		return -1
	case ar > br:
		return 1
	default:
		return 0
	}
}

func logLevelRank(level domain.LogLevel) int {
	switch level {
	case domain.LogLevelDebug:
		return 0
	case domain.LogLevelInfo:
		return 1
	case domain.LogLevelNotice:
		return 2
	case domain.LogLevelWarning:
		return 3
	case domain.LogLevelError:
		return 4
	case domain.LogLevelCritical:
		return 5
	case domain.LogLevelAlert:
		return 6
	case domain.LogLevelEmergency:
		return 7
	default:
		return 0
	}
}

func closedLogEntryChannel() chan domain.LogEntry {
	ch := make(chan domain.LogEntry)
	close(ch)
	return ch
}

func closedRuntimeStatusChannel() chan domain.RuntimeStatusSnapshot {
	ch := make(chan domain.RuntimeStatusSnapshot)
	close(ch)
	return ch
}

func closedServerInitStatusChannel() chan domain.ServerInitStatusSnapshot {
	ch := make(chan domain.ServerInitStatusSnapshot)
	close(ch)
	return ch
}
