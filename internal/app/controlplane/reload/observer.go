package reload

import (
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
)

type Observer struct {
	metrics    domain.Metrics
	coreLogger *zap.Logger
	logger     *zap.Logger
}

func NewObserver(metrics domain.Metrics, coreLogger, logger *zap.Logger) *Observer {
	if coreLogger == nil {
		coreLogger = zap.NewNop()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Observer{
		metrics:    metrics,
		coreLogger: coreLogger,
		logger:     logger,
	}
}

func (o *Observer) SetCoreLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	o.coreLogger = logger
}

func (o *Observer) RecordReloadSuccess(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	if o.metrics == nil {
		return
	}
	o.metrics.RecordReloadSuccess(source, action)
}

func (o *Observer) RecordReloadFailure(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	if o.metrics == nil {
		return
	}
	o.metrics.RecordReloadFailure(source, action)
}

func (o *Observer) RecordReloadRestart(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	if o.metrics == nil {
		return
	}
	o.metrics.RecordReloadRestart(source, action)
}

func (o *Observer) RecordReloadActionFailures(source domain.CatalogUpdateSource, diff domain.CatalogDiff) {
	for range diff.AddedSpecKeys {
		o.RecordReloadFailure(source, domain.ReloadActionServerAdd)
	}
	for range diff.RemovedSpecKeys {
		o.RecordReloadFailure(source, domain.ReloadActionServerRemove)
	}
	for range diff.UpdatedSpecKeys {
		o.RecordReloadFailure(source, domain.ReloadActionServerUpdate)
	}
	for range diff.ReplacedSpecKeys {
		o.RecordReloadFailure(source, domain.ReloadActionServerReplace)
	}
}

func (o *Observer) HandleApplyError(update domain.CatalogUpdate, err error, duration time.Duration) {
	reloadMode := ResolveMode(update.Snapshot.Summary.Runtime.ReloadMode)
	stage := FailureStage(err)
	fields := []zap.Field{
		zap.Uint64("revision", update.Snapshot.Revision),
		zap.Int("servers", update.Snapshot.Summary.TotalServers),
		zap.Int("added", len(update.Diff.AddedSpecKeys)),
		zap.Int("removed", len(update.Diff.RemovedSpecKeys)),
		zap.Int("updated", len(update.Diff.UpdatedSpecKeys)),
		zap.String("reload_mode", string(reloadMode)),
		zap.String("failure_stage", stage),
		zap.String("failure_summary", err.Error()),
		zap.Duration("latency", duration),
		zap.Error(err),
	}
	o.ObserveReloadApply(reloadMode, domain.ReloadApplyResultFailure, stage, duration)
	if reloadMode == domain.ReloadModeStrict {
		o.coreLogger.Fatal("config reload apply failed; shutting down", fields...)
	}
	o.logger.Warn("config reload apply failed", fields...)
}

func (o *Observer) ObserveReloadApply(mode domain.ReloadMode, result domain.ReloadApplyResult, summary string, duration time.Duration) {
	if o.metrics == nil {
		return
	}
	o.metrics.ObserveReloadApply(domain.ReloadApplyMetric{
		Mode:     mode,
		Result:   result,
		Summary:  summary,
		Duration: duration,
	})
}

func (o *Observer) ObserveReloadRollback(mode domain.ReloadMode, result domain.ReloadRollbackResult, summary string, duration time.Duration) {
	if o.metrics == nil {
		return
	}
	o.metrics.ObserveReloadRollback(domain.ReloadRollbackMetric{
		Mode:     mode,
		Result:   result,
		Summary:  summary,
		Duration: duration,
	})
}
