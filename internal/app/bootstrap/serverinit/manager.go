package serverinit

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap/activation"
	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry/diagnostics"
)

// Manager coordinates async server initialization.
type Manager struct {
	scheduler domain.Scheduler
	specs     map[string]domain.ServerSpec
	runtime   domain.RuntimeConfig
	logger    *zap.Logger
	probe     diagnostics.Probe

	mu         sync.Mutex
	statuses   map[string]domain.ServerInitStatus
	causes     map[string]domain.StartCause
	targets    map[string]int
	running    map[string]struct{}
	retryBase  time.Duration
	retryMax   time.Duration
	maxRetries int

	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

// NewManager constructs a server initialization manager.
func NewManager(
	scheduler domain.Scheduler,
	state *domain.CatalogState,
	logger *zap.Logger,
	probe diagnostics.Probe,
) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	summary := state.Summary
	specs := summary.SpecRegistry
	runtime := summary.Runtime

	retryBase, retryMax, maxRetries := resolveServerInitRetry(runtime)

	return &Manager{
		scheduler:  scheduler,
		specs:      specs,
		runtime:    runtime,
		logger:     logger.Named("server_init"),
		probe:      probe,
		statuses:   make(map[string]domain.ServerInitStatus),
		causes:     make(map[string]domain.StartCause),
		targets:    make(map[string]int),
		running:    make(map[string]struct{}),
		retryBase:  retryBase,
		retryMax:   retryMax,
		maxRetries: maxRetries,
	}
}

// ApplyCatalogState updates the manager with a new catalog state.
func (m *Manager) ApplyCatalogState(state *domain.CatalogState) {
	summary := state.Summary
	specs := summary.SpecRegistry
	runtime := summary.Runtime
	retryBase, retryMax, maxRetries := resolveServerInitRetry(runtime)

	var added []string
	var activated []string

	m.mu.Lock()
	m.specs = specs
	m.runtime = runtime
	m.retryBase = retryBase
	m.retryMax = retryMax
	m.maxRetries = maxRetries

	for specKey, spec := range specs {
		minReady := activation.BaselineMinReady(runtime, spec)
		status, ok := m.statuses[specKey]
		if !ok {
			added = append(added, specKey)
			status = domain.ServerInitStatus{
				SpecKey:     specKey,
				ServerName:  spec.Name,
				MinReady:    minReady,
				State:       domain.ServerInitPending,
				RetryCount:  0,
				NextRetryAt: time.Time{},
				UpdatedAt:   time.Now(),
			}
		} else {
			status.ServerName = spec.Name
			status.MinReady = minReady
			if minReady == 0 {
				status.State = domain.ServerInitPending
				status.Ready = 0
				status.Failed = 0
				status.LastError = ""
				status.RetryCount = 0
				status.NextRetryAt = time.Time{}
				status.AttemptStartedAt = time.Time{}
				status.AttemptEndedAt = time.Time{}
				status.AttemptStep = ""
				status.AttemptError = ""
				status.AttemptReady = 0
				status.AttemptFailed = 0
				status.AttemptTarget = 0
			}
			status.UpdatedAt = time.Now()
		}
		m.statuses[specKey] = status
		if m.targets[specKey] != minReady && minReady > 0 {
			activated = append(activated, specKey)
		}
		m.targets[specKey] = minReady
		if minReady > 0 {
			m.causes[specKey] = activation.PolicyStartCause(runtime, spec, minReady)
		} else {
			delete(m.causes, specKey)
		}
	}

	for specKey := range m.statuses {
		if _, ok := specs[specKey]; !ok {
			delete(m.statuses, specKey)
			delete(m.targets, specKey)
			delete(m.causes, specKey)
		}
	}
	started := m.started
	m.mu.Unlock()

	if !started {
		return
	}
	for _, specKey := range added {
		m.ensureWorker(specKey)
	}
	for _, specKey := range activated {
		m.ensureWorker(specKey)
	}
}

func resolveServerInitRetry(runtime domain.RuntimeConfig) (time.Duration, time.Duration, int) {
	retryBase := runtime.ServerInitRetryBaseDuration()
	retryMax := runtime.ServerInitRetryMaxDuration()
	if retryMax < retryBase {
		retryMax = retryBase
	}
	maxRetries := runtime.ServerInitMaxRetries
	if maxRetries < 0 {
		maxRetries = domain.DefaultServerInitMaxRetries
	}
	return retryBase, retryMax, maxRetries
}

// Start begins background initialization work.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	now := time.Now()
	for specKey, spec := range m.specs {
		minReady := activation.BaselineMinReady(m.runtime, spec)
		m.targets[specKey] = minReady
		m.statuses[specKey] = domain.ServerInitStatus{
			SpecKey:     specKey,
			ServerName:  spec.Name,
			MinReady:    minReady,
			State:       domain.ServerInitPending,
			RetryCount:  0,
			NextRetryAt: time.Time{},
			UpdatedAt:   now,
		}
	}
	m.started = true
	m.mu.Unlock()

	for specKey := range m.specs {
		m.ensureWorker(specKey)
	}
}

// Stop stops background initialization work.
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// SetMinReady updates min-ready settings for a spec.
func (m *Manager) SetMinReady(specKey string, minReady int, cause domain.StartCause) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return errors.New("server initialization manager not started")
	}
	if _, ok := m.specs[specKey]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown spec key %q", specKey)
	}
	if minReady < 0 {
		minReady = 0
	}

	targetChanged := m.targets[specKey] != minReady
	m.targets[specKey] = minReady
	if minReady > 0 {
		if cause.Reason == "" {
			if spec, ok := m.specs[specKey]; ok {
				cause = activation.PolicyStartCause(m.runtime, spec, minReady)
			}
		}
		m.causes[specKey] = cause
	} else {
		delete(m.causes, specKey)
	}
	status := m.statuses[specKey]
	status.MinReady = minReady
	if minReady == 0 {
		status.State = domain.ServerInitPending
		status.Ready = 0
		status.Failed = 0
		status.LastError = ""
		status.RetryCount = 0
		status.NextRetryAt = time.Time{}
		status.AttemptStartedAt = time.Time{}
		status.AttemptEndedAt = time.Time{}
		status.AttemptStep = ""
		status.AttemptError = ""
		status.AttemptReady = 0
		status.AttemptFailed = 0
		status.AttemptTarget = 0
	}
	status.UpdatedAt = time.Now()
	m.statuses[specKey] = status
	m.mu.Unlock()

	if minReady == 0 {
		return nil
	}
	if targetChanged {
		m.ensureWorker(specKey)
	}
	return nil
}

// RetrySpec requests a retry for a spec initialization.
func (m *Manager) RetrySpec(specKey string) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return errors.New("server initialization manager not started")
	}
	if _, ok := m.specs[specKey]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown spec key %q", specKey)
	}
	status := m.statuses[specKey]
	status.State = domain.ServerInitPending
	status.LastError = ""
	status.RetryCount = 0
	status.NextRetryAt = time.Time{}
	status.AttemptStartedAt = time.Time{}
	status.AttemptEndedAt = time.Time{}
	status.AttemptStep = ""
	status.AttemptError = ""
	status.AttemptReady = 0
	status.AttemptFailed = 0
	status.AttemptTarget = 0
	status.UpdatedAt = time.Now()
	m.statuses[specKey] = status
	target := m.targets[specKey]
	m.mu.Unlock()

	if target > 0 {
		m.ensureWorker(specKey)
	}
	return nil
}

// Statuses returns the current init status snapshot.
func (m *Manager) Statuses(ctx context.Context) []domain.ServerInitStatus {
	m.mu.Lock()
	scheduler := m.scheduler
	result := make([]domain.ServerInitStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		result = append(result, status)
	}
	m.mu.Unlock()

	if scheduler != nil {
		if ctx == nil {
			ctx = context.Background()
		}
		pools, err := scheduler.GetPoolStatus(ctx)
		if err == nil {
			readyBySpec := make(map[string]int, len(pools))
			failedBySpec := make(map[string]int, len(pools))
			minReadyBySpec := make(map[string]int, len(pools))
			for _, pool := range pools {
				ready := 0
				failed := 0
				for _, inst := range pool.Instances {
					switch inst.State {
					case domain.InstanceStateReady, domain.InstanceStateBusy:
						ready++
					case domain.InstanceStateFailed:
						failed++
					case domain.InstanceStateStarting,
						domain.InstanceStateInitializing,
						domain.InstanceStateHandshaking,
						domain.InstanceStateDraining,
						domain.InstanceStateStopped:
					}
				}
				readyBySpec[pool.SpecKey] = ready
				failedBySpec[pool.SpecKey] = failed
				minReadyBySpec[pool.SpecKey] = pool.MinReady
			}

			for i := range result {
				specKey := result[i].SpecKey
				if minReady, ok := minReadyBySpec[specKey]; ok {
					result[i].MinReady = minReady
				}
				ready := readyBySpec[specKey]
				failed := failedBySpec[specKey]
				state := deriveInitState(result[i], ready, failed)
				result[i].Ready = ready
				result[i].Failed = failed
				result[i].State = state
				if state == domain.ServerInitReady {
					result[i].LastError = ""
					result[i].RetryCount = 0
					result[i].NextRetryAt = time.Time{}
				}
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].ServerName == result[j].ServerName {
			return result[i].SpecKey < result[j].SpecKey
		}
		return result[i].ServerName < result[j].ServerName
	})
	return result
}

func (m *Manager) ensureWorker(specKey string) {
	m.mu.Lock()
	if _, ok := m.running[specKey]; ok {
		m.mu.Unlock()
		return
	}
	m.running[specKey] = struct{}{}
	ctx := m.ctx
	m.mu.Unlock()

	go m.runSpec(ctx, specKey)
}

func (m *Manager) runSpec(ctx context.Context, specKey string) {
	defer func() {
		m.mu.Lock()
		delete(m.running, specKey)
		m.mu.Unlock()
	}()

	if ctx == nil {
		ctx = context.Background()
	}

	timer := time.NewTimer(m.retryBase)
	defer timer.Stop()

	for {
		target := m.target(specKey)
		serverName := m.specName(specKey)
		if target == 0 {
			m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
				status.State = domain.ServerInitPending
				status.MinReady = 0
				status.Ready = 0
				status.Failed = 0
				status.LastError = ""
				status.RetryCount = 0
				status.NextRetryAt = time.Time{}
				status.AttemptStartedAt = time.Time{}
				status.AttemptEndedAt = time.Time{}
				status.AttemptStep = ""
				status.AttemptError = ""
				status.AttemptReady = 0
				status.AttemptFailed = 0
				status.AttemptTarget = 0
				status.UpdatedAt = time.Now()
			})
			return
		}

		if m.scheduler == nil {
			m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
				now := time.Now()
				status.State = domain.ServerInitFailed
				status.LastError = "scheduler unavailable"
				status.RetryCount = 0
				status.NextRetryAt = time.Time{}
				status.AttemptStartedAt = now
				status.AttemptEndedAt = now
				status.AttemptStep = "scheduler_unavailable"
				status.AttemptError = "scheduler unavailable"
				status.AttemptReady = 0
				status.AttemptFailed = 0
				status.AttemptTarget = target
				status.UpdatedAt = now
			})
			return
		}

		prevStatus, _ := m.getStatus(specKey)

		attemptStarted := time.Now()
		attemptID := diagnostics.NewAttemptID(specKey, attemptStarted)
		attemptCtx := diagnostics.WithAttemptID(ctx, attemptID)
		m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
			status.State = domain.ServerInitStarting
			status.MinReady = target
			status.AttemptStartedAt = attemptStarted
			status.AttemptEndedAt = time.Time{}
			status.AttemptStep = "set_min_ready"
			status.AttemptError = ""
			status.AttemptReady = 0
			status.AttemptFailed = 0
			status.AttemptTarget = target
			status.UpdatedAt = attemptStarted
		})

		causeCtx := ctx
		if cause, ok := m.startCause(specKey); ok {
			causeCtx = domain.WithStartCause(ctx, cause)
		}
		startSetMinReady := time.Now()
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: serverName,
			AttemptID:  attemptID,
			Step:       diagnostics.StepSetMinReady,
			Phase:      diagnostics.PhaseEnter,
			Timestamp:  startSetMinReady,
			Attributes: map[string]string{"target": strconv.Itoa(target)},
		})
		err := m.scheduler.SetDesiredMinReady(diagnostics.WithAttemptID(causeCtx, attemptID), specKey, target)
		setPhase := diagnostics.PhaseExit
		if err != nil {
			setPhase = diagnostics.PhaseError
		}
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: serverName,
			AttemptID:  attemptID,
			Step:       diagnostics.StepSetMinReady,
			Phase:      setPhase,
			Timestamp:  time.Now(),
			Duration:   time.Since(startSetMinReady),
			Error:      errString(err),
			Attributes: map[string]string{"target": strconv.Itoa(target)},
		})
		m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
			status.AttemptStep = "set_min_ready_done"
			if err != nil {
				status.AttemptError = err.Error()
			}
			status.UpdatedAt = time.Now()
		})
		snapshotStarted := time.Now()
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: serverName,
			AttemptID:  attemptID,
			Step:       diagnostics.StepSnapshotDone,
			Phase:      diagnostics.PhaseEnter,
			Timestamp:  snapshotStarted,
		})
		ready, failed, snapshotErr := m.snapshot(attemptCtx, specKey)
		if snapshotErr != nil {
			m.logger.Warn("server init snapshot failed",
				zap.String("specKey", specKey),
				zap.Error(snapshotErr),
			)
			if err == nil {
				err = snapshotErr
			}
		}
		snapshotPhase := diagnostics.PhaseExit
		if snapshotErr != nil {
			snapshotPhase = diagnostics.PhaseError
		}
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: serverName,
			AttemptID:  attemptID,
			Step:       diagnostics.StepSnapshotDone,
			Phase:      snapshotPhase,
			Timestamp:  time.Now(),
			Duration:   time.Since(snapshotStarted),
			Error:      errString(snapshotErr),
			Attributes: map[string]string{
				"ready":  strconv.Itoa(ready),
				"failed": strconv.Itoa(failed),
			},
		})
		m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
			status.AttemptReady = ready
			status.AttemptFailed = failed
			status.AttemptStep = "snapshot_done"
			if err != nil {
				status.AttemptError = err.Error()
			}
			status.UpdatedAt = time.Now()
		})
		m.applyResult(specKey, target, ready, failed, err)
		m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
			status.AttemptEndedAt = time.Now()
			status.AttemptStep = "apply_result"
			if err != nil {
				status.AttemptError = err.Error()
			}
			status.UpdatedAt = time.Now()
		})

		if ready >= target {
			m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
				status.RetryCount = 0
				status.NextRetryAt = time.Time{}
				status.UpdatedAt = time.Now()
			})
			return
		}

		retryCount := m.nextRetryCount(prevStatus, ready, failed, err)
		if err != nil && isFatalInitError(err) {
			m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
				status.State = domain.ServerInitSuspended
				status.LastError = err.Error()
				status.RetryCount = retryCount
				status.NextRetryAt = time.Time{}
				status.UpdatedAt = time.Now()
			})
			return
		}

		if m.maxRetries > 0 && retryCount >= m.maxRetries {
			m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
				status.State = domain.ServerInitSuspended
				status.LastError = retryLimitMessage(err, m.maxRetries, failed)
				status.RetryCount = retryCount
				status.NextRetryAt = time.Time{}
				status.UpdatedAt = time.Now()
			})
			return
		}

		delay := m.nextRetryDelay(retryCount)
		nextRetryAt := time.Now().Add(delay)
		m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
			status.RetryCount = retryCount
			status.NextRetryAt = nextRetryAt
			status.UpdatedAt = time.Now()
		})

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(delay)

		select {
		case <-ctx.Done():
			m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
				now := time.Now()
				status.State = domain.ServerInitFailed
				status.LastError = ctx.Err().Error()
				status.NextRetryAt = time.Time{}
				status.AttemptEndedAt = now
				status.AttemptStep = "context_done"
				status.AttemptError = ctx.Err().Error()
				status.UpdatedAt = now
			})
			return
		case <-timer.C:
		}
	}
}

func (m *Manager) applyResult(specKey string, target, ready, failed int, err error) {
	state := domain.ServerInitStarting
	switch {
	case target == 0 && err == nil && failed == 0:
		// On-demand server with no activation: metadata ready, no instances needed.
		state = domain.ServerInitReady
	case ready >= target && target > 0:
		state = domain.ServerInitReady
	case ready > 0:
		state = domain.ServerInitDegraded
	case err != nil || failed > 0:
		state = domain.ServerInitFailed
	default:
		state = domain.ServerInitStarting
	}

	lastError := ""
	if err != nil {
		lastError = err.Error()
	}

	m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
		status.State = state
		status.MinReady = target
		status.Ready = ready
		status.Failed = failed
		status.LastError = lastError
		status.UpdatedAt = time.Now()
	})
}

func deriveInitState(status domain.ServerInitStatus, ready, failed int) domain.ServerInitState {
	target := status.MinReady
	if target <= 0 {
		// On-demand server with no activation: metadata ready, no instances needed.
		// Return ready regardless of instance count (0 is valid).
		if failed == 0 {
			return domain.ServerInitReady
		}
		return domain.ServerInitFailed
	}

	if status.State == domain.ServerInitSuspended && ready < target {
		return domain.ServerInitSuspended
	}
	if status.State == domain.ServerInitFailed && ready < target {
		return domain.ServerInitFailed
	}

	switch {
	case ready >= target:
		return domain.ServerInitReady
	case ready > 0:
		return domain.ServerInitDegraded
	case failed > 0:
		return domain.ServerInitFailed
	default:
		return domain.ServerInitStarting
	}
}

func (m *Manager) snapshot(ctx context.Context, specKey string) (int, int, error) {
	m.mu.Lock()
	scheduler := m.scheduler
	m.mu.Unlock()

	if scheduler == nil {
		return 0, 0, errors.New("scheduler unavailable")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	pools, err := scheduler.GetPoolStatus(ctx)
	if err != nil {
		return 0, 0, err
	}

	for _, pool := range pools {
		if pool.SpecKey != specKey {
			continue
		}
		ready := 0
		failed := 0
		for _, inst := range pool.Instances {
			switch inst.State {
			case domain.InstanceStateReady, domain.InstanceStateBusy:
				ready++
			case domain.InstanceStateFailed:
				failed++
			case domain.InstanceStateStarting,
				domain.InstanceStateInitializing,
				domain.InstanceStateHandshaking,
				domain.InstanceStateDraining,
				domain.InstanceStateStopped:
			}
		}
		return ready, failed, nil
	}
	return 0, 0, nil
}

func (m *Manager) recordEvent(event diagnostics.Event) {
	if m == nil || m.probe == nil {
		return
	}
	m.probe.Record(event)
}

func (m *Manager) specName(specKey string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[specKey]
	if !ok {
		return ""
	}
	return spec.Name
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (m *Manager) updateStatus(specKey string, mutate func(*domain.ServerInitStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.statuses[specKey]
	if !ok {
		return
	}
	mutate(&status)
	m.statuses[specKey] = status
}

func (m *Manager) getStatus(specKey string) (domain.ServerInitStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.statuses[specKey]
	return status, ok
}

func (m *Manager) target(specKey string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.targets[specKey]
}

func (m *Manager) startCause(specKey string) (domain.StartCause, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cause, ok := m.causes[specKey]
	return cause, ok
}

func (m *Manager) nextRetryCount(prev domain.ServerInitStatus, ready, failed int, err error) int {
	if ready > prev.Ready {
		return 0
	}
	retryCount := prev.RetryCount
	if err != nil || failed > prev.Failed {
		retryCount++
	}
	return retryCount
}

func (m *Manager) nextRetryDelay(retryCount int) time.Duration {
	if retryCount < 1 {
		retryCount = 1
	}
	delay := m.retryBase
	for i := 1; i < retryCount; i++ {
		delay *= 2
		if delay >= m.retryMax {
			return m.retryMax
		}
	}
	if delay > m.retryMax {
		return m.retryMax
	}
	return delay
}

func retryLimitMessage(err error, maxRetries int, failed int) string {
	if err != nil {
		return fmt.Sprintf("retry limit reached (%d): %s", maxRetries, err.Error())
	}
	if failed > 0 {
		return fmt.Sprintf("retry limit reached (%d) with %d failed instances", maxRetries, failed)
	}
	return fmt.Sprintf("retry limit reached (%d)", maxRetries)
}

func isFatalInitError(err error) bool {
	return errors.Is(err, domain.ErrInvalidCommand) ||
		errors.Is(err, domain.ErrExecutableNotFound) ||
		errors.Is(err, domain.ErrPermissionDenied) ||
		errors.Is(err, domain.ErrUnsupportedProtocol) ||
		errors.Is(err, domain.ErrUnknownSpecKey)
}
