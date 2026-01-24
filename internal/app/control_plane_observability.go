package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
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
	registry *clientRegistry
	logs     *telemetry.LogBroadcaster

	runtimeStatusIdx *aggregator.RuntimeStatusIndex
	serverInitIdx    *aggregator.ServerInitIndex
}

func newObservabilityService(state *controlPlaneState, registry *clientRegistry, logs *telemetry.LogBroadcaster) *observabilityService {
	return &observabilityService{
		state:    state,
		registry: registry,
		logs:     logs,
	}
}

// StreamLogs streams logs for a caller.
func (o *observabilityService) StreamLogs(ctx context.Context, client string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	if _, err := o.registry.resolveClientTags(client); err != nil {
		return closedLogEntryChannel(), err
	}
	return o.streamLogs(ctx, minLevel)
}

// StreamLogsAllServers streams logs across all servers.
func (o *observabilityService) StreamLogsAllServers(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
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

// GetPoolStatus returns current pool status.
func (o *observabilityService) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	if o.state.scheduler == nil {
		return nil, nil
	}
	return o.state.scheduler.GetPoolStatus(ctx)
}

// GetServerInitStatus returns current server init statuses.
func (o *observabilityService) GetServerInitStatus(ctx context.Context) ([]domain.ServerInitStatus, error) {
	if o.state.initManager == nil {
		return nil, nil
	}
	return o.state.initManager.Statuses(ctx), nil
}

// WatchRuntimeStatus streams runtime status for a caller.
func (o *observabilityService) WatchRuntimeStatus(ctx context.Context, client string) (<-chan domain.RuntimeStatusSnapshot, error) {
	if _, err := o.registry.resolveClientTags(client); err != nil {
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
func (o *observabilityService) WatchRuntimeStatusAllServers(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	if o.runtimeStatusIdx == nil {
		return closedRuntimeStatusChannel(), nil
	}
	return o.runtimeStatusIdx.Subscribe(ctx), nil
}

// WatchServerInitStatus streams server init status for a caller.
func (o *observabilityService) WatchServerInitStatus(ctx context.Context, client string) (<-chan domain.ServerInitStatusSnapshot, error) {
	if _, err := o.registry.resolveClientTags(client); err != nil {
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
func (o *observabilityService) WatchServerInitStatusAllServers(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	if o.serverInitIdx == nil {
		return closedServerInitStatusChannel(), nil
	}
	return o.serverInitIdx.Subscribe(ctx), nil
}

// SetRuntimeStatusIndex updates the runtime status index.
func (o *observabilityService) SetRuntimeStatusIndex(idx *aggregator.RuntimeStatusIndex) {
	o.runtimeStatusIdx = idx
	if idx != nil {
		go o.runRuntimeStatusWorker()
	}
}

// SetServerInitIndex updates the server init status index.
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

func (o *observabilityService) sendFilteredRuntimeStatus(ch chan<- domain.RuntimeStatusSnapshot, client string, snapshot domain.RuntimeStatusSnapshot) {
	tags, err := o.registry.resolveClientTags(client)
	if err != nil {
		return
	}
	filtered := filterRuntimeStatusSnapshot(snapshot, o.visibleSpecKeys(tags))
	select {
	case ch <- filtered:
	default:
	}
}

func (o *observabilityService) sendFilteredServerInitStatus(ch chan<- domain.ServerInitStatusSnapshot, client string, snapshot domain.ServerInitStatusSnapshot) {
	tags, err := o.registry.resolveClientTags(client)
	if err != nil {
		return
	}
	filtered := filterServerInitStatusSnapshot(snapshot, o.visibleSpecKeys(tags))
	select {
	case ch <- filtered:
	default:
	}
}

func (o *observabilityService) visibleSpecKeys(tags []string) map[string]struct{} {
	catalog := o.state.Catalog()
	serverSpecKeys := o.state.ServerSpecKeys()
	visible := make(map[string]struct{})
	for name, specKey := range serverSpecKeys {
		spec, ok := catalog.Specs[name]
		if !ok {
			continue
		}
		if isVisibleToTags(tags, spec.Tags) {
			visible[specKey] = struct{}{}
		}
	}
	return visible
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
