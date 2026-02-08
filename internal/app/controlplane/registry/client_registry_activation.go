package registry

import (
	"context"
	"errors"
	"sort"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap/activation"
	"mcpv/internal/domain"
)

func (r *ClientRegistry) activateSpecs(ctx context.Context, specKeys []string, client string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	runtime := r.state.Runtime()
	registry := r.state.SpecRegistry()

	type activationTask struct {
		specKey  string
		minReady int
		cause    domain.StartCause
	}

	// filter the specs that need to be started (reference count from 0 to 1)
	r.mu.Lock()
	specsToStart := make([]string, 0, len(order))
	for _, specKey := range order {
		// only start if going from 0 to 1
		if r.specCounts[specKey] == 1 {
			specsToStart = append(specsToStart, specKey)
		}
	}
	r.mu.Unlock()

	// Prepare activation tasks outside the lock to avoid lock inversion.
	tasks := make([]activationTask, 0, len(specsToStart))
	for _, specKey := range specsToStart {
		spec, ok := registry[specKey]
		if !ok {
			return errors.New("unknown spec key " + specKey)
		}
		minReady := activation.ActiveMinReady(spec)
		cause := activation.ClientStartCause(runtime, spec, client, minReady)
		tasks = append(tasks, activationTask{
			specKey:  specKey,
			minReady: minReady,
			cause:    cause,
		})
	}

	// Perform the actual start operations outside the lock.
	for _, task := range tasks {
		causeCtx := domain.WithStartCause(ctx, task.cause)
		startup := r.state.Startup()
		if startup != nil {
			err := startup.SetMinReady(task.specKey, task.minReady, task.cause)
			if err == nil {
				continue
			}
			r.state.Logger().Warn("server init manager failed to set min ready", zap.String("specKey", task.specKey), zap.Error(err))
		}
		scheduler := r.state.Scheduler()
		if scheduler == nil {
			return errors.New("scheduler not configured")
		}
		if err := scheduler.SetDesiredMinReady(causeCtx, task.specKey, task.minReady); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClientRegistry) deactivateSpecs(ctx context.Context, specKeys []string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	runtime := r.state.Runtime()
	registry := r.state.SpecRegistry()
	var firstErr error

	// filter specs to stop under the lock to avoid race with spec counts
	r.mu.Lock()
	specsToStop := make([]string, 0, len(order))
	for _, specKey := range order {
		if r.specCounts[specKey] > 0 {
			continue
		}
		spec, ok := registry[specKey]
		if ok && activation.ResolveActivationMode(runtime, spec) == domain.ActivationAlwaysOn {
			continue
		}
		specsToStop = append(specsToStop, specKey)
	}
	r.mu.Unlock()

	// Perform actual stop outside lock.
	for _, specKey := range specsToStop {
		startup := r.state.Startup()
		if startup != nil {
			_ = startup.SetMinReady(specKey, 0, domain.StartCause{})
		}
		scheduler := r.state.Scheduler()
		if scheduler == nil {
			if firstErr == nil {
				firstErr = errors.New("scheduler not configured")
			}
			continue
		}
		if err := scheduler.StopSpec(ctx, specKey, "client inactive"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
