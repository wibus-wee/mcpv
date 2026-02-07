package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/app/controlplane/registry"
	"mcpv/internal/domain"
	"mcpv/internal/infra/aggregator"
	"mcpv/internal/infra/telemetry"
)

const (
	runtimeStatusRefreshInterval = 500 * time.Millisecond
	serverInitRefreshInterval    = time.Second
)

type Service struct {
	state    State
	registry *registry.ClientRegistry
	logs     *telemetry.LogBroadcaster

	runtimeStatusIdx           *aggregator.RuntimeStatusIndex
	serverInitIdx              *aggregator.ServerInitIndex
	runtimeStatusWorkerStarted atomic.Bool
	serverInitWorkerStarted    atomic.Bool
}

func NewObservabilityService(state State, registry *registry.ClientRegistry, logs *telemetry.LogBroadcaster) *Service {
	return &Service{
		state:    state,
		registry: registry,
		logs:     logs,
	}
}

// StreamLogs streams logs for a caller.
func (o *Service) StreamLogs(ctx context.Context, client string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	if _, err := o.registry.ResolveVisibleSpecKeys(client); err != nil {
		return closedLogEntryChannel(), err
	}
	return o.streamLogs(ctx, minLevel)
}

// StreamLogsAllServers streams logs across all servers.
func (o *Service) StreamLogsAllServers(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return o.streamLogs(ctx, minLevel)
}

func (o *Service) streamLogs(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
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

// GetPoolStatus returns current pool status.
func (o *Service) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	scheduler := o.state.Scheduler()
	if scheduler == nil {
		return nil, nil
	}
	return scheduler.GetPoolStatus(ctx)
}

// GetServerInitStatus returns current server init statuses.
func (o *Service) GetServerInitStatus(ctx context.Context) ([]domain.ServerInitStatus, error) {
	initManager := o.state.InitManager()
	if initManager == nil {
		return nil, nil
	}
	statuses := initManager.Statuses(ctx)

	// Check bootstrap errors and mark failed servers
	bootstrapManager := o.state.BootstrapManager()
	if bootstrapManager != nil {
		progress := bootstrapManager.GetProgress()
		if progress.State == domain.BootstrapFailed || len(progress.Errors) > 0 {
			for i := range statuses {
				if err, exists := progress.Errors[statuses[i].SpecKey]; exists && err != "" {
					statuses[i].State = domain.ServerInitFailed
					statuses[i].LastError = err
				}
			}
		}
	}

	return statuses, nil
}

// WatchRuntimeStatus streams runtime status for a caller.
func (o *Service) WatchRuntimeStatus(ctx context.Context, client string) (<-chan domain.RuntimeStatusSnapshot, error) {
	if _, err := o.registry.ResolveVisibleSpecKeys(client); err != nil {
		return closedRuntimeStatusChannel(), err
	}
	if o.runtimeStatusIdx == nil {
		return closedRuntimeStatusChannel(), nil
	}

	output := make(chan domain.RuntimeStatusSnapshot, 1)
	updates := o.runtimeStatusIdx.Subscribe(ctx)
	changes := o.registry.WatchClientChanges(ctx)

	go func() {
		defer close(output)
		var last domain.RuntimeStatusSnapshot
		last = o.runtimeStatusIdx.Current()
		o.sendFilteredRuntimeStatus(output, client, last)
		for {
			select {
			case <-ctx.Done():
				return
			case snapshot, ok := <-updates:
				if !ok {
					return
				}
				last = snapshot
				o.sendFilteredRuntimeStatus(output, client, snapshot)
			case event, ok := <-changes:
				if !ok {
					return
				}
				if event.Client == client {
					o.sendFilteredRuntimeStatus(output, client, last)
				}
			}
		}
	}()

	return output, nil
}

// WatchRuntimeStatusAllServers streams runtime status across all servers.
func (o *Service) WatchRuntimeStatusAllServers(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	if o.runtimeStatusIdx == nil {
		return closedRuntimeStatusChannel(), nil
	}
	return o.runtimeStatusIdx.Subscribe(ctx), nil
}

// WatchServerInitStatus streams server init status for a caller.
func (o *Service) WatchServerInitStatus(ctx context.Context, client string) (<-chan domain.ServerInitStatusSnapshot, error) {
	if _, err := o.registry.ResolveVisibleSpecKeys(client); err != nil {
		return closedServerInitStatusChannel(), err
	}
	if o.serverInitIdx == nil {
		return closedServerInitStatusChannel(), nil
	}

	output := make(chan domain.ServerInitStatusSnapshot, 1)
	updates := o.serverInitIdx.Subscribe(ctx)
	changes := o.registry.WatchClientChanges(ctx)

	go func() {
		defer close(output)
		var last domain.ServerInitStatusSnapshot
		last = o.serverInitIdx.Current()
		o.sendFilteredServerInitStatus(output, client, last)
		for {
			select {
			case <-ctx.Done():
				return
			case snapshot, ok := <-updates:
				if !ok {
					return
				}
				last = snapshot
				o.sendFilteredServerInitStatus(output, client, snapshot)
			case event, ok := <-changes:
				if !ok {
					return
				}
				if event.Client == client {
					o.sendFilteredServerInitStatus(output, client, last)
				}
			}
		}
	}()

	return output, nil
}

// WatchServerInitStatusAllServers streams server init status across all servers.
func (o *Service) WatchServerInitStatusAllServers(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	if o.serverInitIdx == nil {
		return closedServerInitStatusChannel(), nil
	}
	return o.serverInitIdx.Subscribe(ctx), nil
}

// SetRuntimeStatusIndex updates the runtime status index.
func (o *Service) SetRuntimeStatusIndex(idx *aggregator.RuntimeStatusIndex) {
	o.runtimeStatusIdx = idx
	if idx == nil {
		return
	}
	if !o.runtimeStatusWorkerStarted.CompareAndSwap(false, true) {
		return
	}
	go o.runRuntimeStatusWorker()
}

// SetServerInitIndex updates the server init status index.
func (o *Service) SetServerInitIndex(idx *aggregator.ServerInitIndex) {
	o.serverInitIdx = idx
	if idx == nil {
		return
	}
	if !o.serverInitWorkerStarted.CompareAndSwap(false, true) {
		return
	}
	go o.runServerInitWorker()
}

func (o *Service) runRuntimeStatusWorker() {
	ticker := time.NewTicker(runtimeStatusRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.state.Context().Done():
			return
		case <-ticker.C:
			ctx := o.state.Context()
			if err := o.runtimeStatusIdx.Refresh(ctx); err != nil {
				o.state.Logger().Warn("runtime status refresh failed", zap.Error(err))
			}
		}
	}
}

func (o *Service) runServerInitWorker() {
	ticker := time.NewTicker(serverInitRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.state.Context().Done():
			return
		case <-ticker.C:
			ctx := o.state.Context()
			if err := o.serverInitIdx.Refresh(ctx); err != nil {
				o.state.Logger().Warn("server init status refresh failed", zap.Error(err))
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

func (o *Service) sendFilteredRuntimeStatus(ch chan<- domain.RuntimeStatusSnapshot, client string, snapshot domain.RuntimeStatusSnapshot) {
	visibleSpecKeys, err := o.registry.ResolveVisibleSpecKeys(client)
	if err != nil {
		return
	}
	filtered := filterRuntimeStatusSnapshot(snapshot, toSpecKeySet(visibleSpecKeys))
	select {
	case ch <- filtered:
	default:
	}
}

func (o *Service) sendFilteredServerInitStatus(ch chan<- domain.ServerInitStatusSnapshot, client string, snapshot domain.ServerInitStatusSnapshot) {
	visibleSpecKeys, err := o.registry.ResolveVisibleSpecKeys(client)
	if err != nil {
		return
	}
	filtered := filterServerInitStatusSnapshot(snapshot, toSpecKeySet(visibleSpecKeys))
	select {
	case ch <- filtered:
	default:
	}
}

func filterRuntimeStatusSnapshot(snapshot domain.RuntimeStatusSnapshot, visibleSpecKeys map[string]struct{}) domain.RuntimeStatusSnapshot {
	if len(snapshot.Statuses) == 0 || len(visibleSpecKeys) == 0 {
		return domain.RuntimeStatusSnapshot{
			ETag:        "",
			Statuses:    []domain.ServerRuntimeStatus{},
			GeneratedAt: snapshot.GeneratedAt,
		}
	}
	statuses := make([]domain.ServerRuntimeStatus, 0, len(snapshot.Statuses))
	for _, status := range snapshot.Statuses {
		if _, ok := visibleSpecKeys[status.SpecKey]; ok {
			statuses = append(statuses, status)
		}
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].SpecKey < statuses[j].SpecKey
	})
	return domain.RuntimeStatusSnapshot{
		ETag:        runtimeStatusETag(statuses),
		Statuses:    statuses,
		GeneratedAt: snapshot.GeneratedAt,
	}
}

func filterServerInitStatusSnapshot(snapshot domain.ServerInitStatusSnapshot, visibleSpecKeys map[string]struct{}) domain.ServerInitStatusSnapshot {
	if len(snapshot.Statuses) == 0 || len(visibleSpecKeys) == 0 {
		return domain.ServerInitStatusSnapshot{
			Statuses:    []domain.ServerInitStatus{},
			GeneratedAt: snapshot.GeneratedAt,
		}
	}
	statuses := make([]domain.ServerInitStatus, 0, len(snapshot.Statuses))
	for _, status := range snapshot.Statuses {
		if _, ok := visibleSpecKeys[status.SpecKey]; ok {
			statuses = append(statuses, status)
		}
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].SpecKey < statuses[j].SpecKey
	})
	return domain.ServerInitStatusSnapshot{
		Statuses:    statuses,
		GeneratedAt: snapshot.GeneratedAt,
	}
}

func runtimeStatusETag(statuses []domain.ServerRuntimeStatus) string {
	if len(statuses) == 0 {
		return ""
	}
	data, err := json.Marshal(statuses)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
