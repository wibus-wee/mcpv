package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type ServerInitializationManager struct {
	scheduler domain.Scheduler
	specs     map[string]domain.ServerSpec
	logger    *zap.Logger

	mu         sync.Mutex
	statuses   map[string]domain.ServerInitStatus
	targets    map[string]int
	running    map[string]struct{}
	retryBase  time.Duration
	retryMax   time.Duration
	maxRetries int

	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

func NewServerInitializationManager(
	scheduler domain.Scheduler,
	snapshot *CatalogSnapshot,
	logger *zap.Logger,
) *ServerInitializationManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	summary := snapshot.Summary()
	specs := summary.specRegistry
	runtime := summary.defaultRuntime

	retryBaseSeconds := runtime.ServerInitRetryBaseSeconds
	if retryBaseSeconds <= 0 {
		retryBaseSeconds = domain.DefaultServerInitRetryBaseSeconds
	}
	retryMaxSeconds := runtime.ServerInitRetryMaxSeconds
	if retryMaxSeconds <= 0 {
		retryMaxSeconds = domain.DefaultServerInitRetryMaxSeconds
	}
	if retryMaxSeconds < retryBaseSeconds {
		retryMaxSeconds = retryBaseSeconds
	}
	maxRetries := runtime.ServerInitMaxRetries
	if maxRetries < 0 {
		maxRetries = domain.DefaultServerInitMaxRetries
	}

	return &ServerInitializationManager{
		scheduler:  scheduler,
		specs:      specs,
		logger:     logger.Named("server_init"),
		statuses:   make(map[string]domain.ServerInitStatus),
		targets:    make(map[string]int),
		running:    make(map[string]struct{}),
		retryBase:  time.Duration(retryBaseSeconds) * time.Second,
		retryMax:   time.Duration(retryMaxSeconds) * time.Second,
		maxRetries: maxRetries,
	}
}

func (m *ServerInitializationManager) Start(ctx context.Context) {
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
		minReady := normalizeMinReady(spec.MinReady)
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

func (m *ServerInitializationManager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *ServerInitializationManager) SetMinReady(specKey string, minReady int) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return errors.New("server initialization manager not started")
	}
	if _, ok := m.specs[specKey]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown spec key %q", specKey)
	}
	switch {
	case minReady < 0:
		minReady = 0
	case minReady > 0:
		minReady = normalizeMinReady(minReady)
	}

	targetChanged := m.targets[specKey] != minReady
	m.targets[specKey] = minReady
	status := m.statuses[specKey]
	status.MinReady = minReady
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

func (m *ServerInitializationManager) RetrySpec(specKey string) error {
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
	status.UpdatedAt = time.Now()
	m.statuses[specKey] = status
	target := m.targets[specKey]
	m.mu.Unlock()

	if target > 0 {
		m.ensureWorker(specKey)
	}
	return nil
}

func (m *ServerInitializationManager) Statuses() []domain.ServerInitStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]domain.ServerInitStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		result = append(result, status)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].ServerName == result[j].ServerName {
			return result[i].SpecKey < result[j].SpecKey
		}
		return result[i].ServerName < result[j].ServerName
	})
	return result
}

func (m *ServerInitializationManager) ensureWorker(specKey string) {
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

func (m *ServerInitializationManager) runSpec(ctx context.Context, specKey string) {
	defer func() {
		m.mu.Lock()
		delete(m.running, specKey)
		m.mu.Unlock()
	}()

	timer := time.NewTimer(m.retryBase)
	defer timer.Stop()

	for {
		target := m.target(specKey)
		if target == 0 {
			m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
				status.State = domain.ServerInitPending
				status.MinReady = 0
				status.Ready = 0
				status.Failed = 0
				status.LastError = ""
				status.RetryCount = 0
				status.NextRetryAt = time.Time{}
				status.UpdatedAt = time.Now()
			})
			return
		}

		if m.scheduler == nil {
			m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
				status.State = domain.ServerInitFailed
				status.LastError = "scheduler unavailable"
				status.RetryCount = 0
				status.NextRetryAt = time.Time{}
				status.UpdatedAt = time.Now()
			})
			return
		}

		prevStatus, _ := m.getStatus(specKey)

		m.updateStatus(specKey, func(status *domain.ServerInitStatus) {
			status.State = domain.ServerInitStarting
			status.MinReady = target
			status.UpdatedAt = time.Now()
		})

		err := m.scheduler.SetDesiredMinReady(ctx, specKey, target)
		ready, failed := m.snapshot(ctx, specKey)
		m.applyResult(specKey, target, ready, failed, err)

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
				status.State = domain.ServerInitFailed
				status.LastError = ctx.Err().Error()
				status.NextRetryAt = time.Time{}
				status.UpdatedAt = time.Now()
			})
			return
		case <-timer.C:
		}
	}
}

func (m *ServerInitializationManager) applyResult(specKey string, target, ready, failed int, err error) {
	state := domain.ServerInitStarting
	switch {
	case ready >= target:
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

func (m *ServerInitializationManager) snapshot(ctx context.Context, specKey string) (int, int) {
	m.mu.Lock()
	scheduler := m.scheduler
	m.mu.Unlock()

	if scheduler == nil {
		return 0, 0
	}

	if ctx == nil {
		ctx = context.Background()
	}

	pools, err := scheduler.GetPoolStatus(ctx)
	if err != nil {
		return 0, 0
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
			}
		}
		return ready, failed
	}
	return 0, 0
}

func (m *ServerInitializationManager) updateStatus(specKey string, mutate func(*domain.ServerInitStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.statuses[specKey]
	if !ok {
		return
	}
	mutate(&status)
	m.statuses[specKey] = status
}

func (m *ServerInitializationManager) getStatus(specKey string) (domain.ServerInitStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.statuses[specKey]
	return status, ok
}

func (m *ServerInitializationManager) target(specKey string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.targets[specKey]
}

func (m *ServerInitializationManager) nextRetryCount(prev domain.ServerInitStatus, ready, failed int, err error) int {
	if ready > prev.Ready {
		return 0
	}
	retryCount := prev.RetryCount
	if err != nil || failed > prev.Failed {
		retryCount++
	}
	return retryCount
}

func (m *ServerInitializationManager) nextRetryDelay(retryCount int) time.Duration {
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

func normalizeMinReady(value int) int {
	if value < 1 {
		return 1
	}
	return value
}
