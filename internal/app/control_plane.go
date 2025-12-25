package app

import (
	"context"
	"encoding/json"
	"runtime/debug"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
	"mcpd/internal/infra/telemetry"
)

type ControlPlane struct {
	info  domain.ControlPlaneInfo
	tools *aggregator.ToolIndex
	logs  *telemetry.LogBroadcaster
}

func NewControlPlane(tools *aggregator.ToolIndex, logs *telemetry.LogBroadcaster) *ControlPlane {
	return &ControlPlane{
		info:  defaultControlPlaneInfo(),
		tools: tools,
		logs:  logs,
	}
}

func (c *ControlPlane) Info(ctx context.Context) (domain.ControlPlaneInfo, error) {
	return c.info, nil
}

func (c *ControlPlane) ListTools(ctx context.Context) (domain.ToolSnapshot, error) {
	if c.tools == nil {
		return domain.ToolSnapshot{}, nil
	}
	return c.tools.Snapshot(), nil
}

func (c *ControlPlane) WatchTools(ctx context.Context) (<-chan domain.ToolSnapshot, error) {
	if c.tools == nil {
		ch := make(chan domain.ToolSnapshot)
		close(ch)
		return ch, nil
	}
	return c.tools.Subscribe(ctx), nil
}

func (c *ControlPlane) CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	if c.tools == nil {
		return nil, domain.ErrToolNotFound
	}
	return c.tools.CallTool(ctx, name, args, routingKey)
}

func (c *ControlPlane) StreamLogs(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	if c.logs == nil {
		ch := make(chan domain.LogEntry)
		close(ch)
		return ch, nil
	}
	if minLevel == "" {
		minLevel = domain.LogLevelDebug
	}
	source := c.logs.Subscribe(ctx)
	out := make(chan domain.LogEntry, 64)

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

func defaultControlPlaneInfo() domain.ControlPlaneInfo {
	info := domain.ControlPlaneInfo{
		Name:    "mcpd",
		Version: "dev",
		Build:   "unknown",
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Version != "" {
			info.Version = bi.Main.Version
		}
		for _, setting := range bi.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				info.Build = setting.Value
				break
			}
		}
	}
	return info
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

var _ domain.ControlPlane = (*ControlPlane)(nil)
