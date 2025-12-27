package app

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
	"mcpd/internal/infra/telemetry"
)

type ControlPlane struct {
	info     domain.ControlPlaneInfo
	profiles map[string]*profileRuntime
	callers  map[string]string
	logs     *telemetry.LogBroadcaster
}

type profileRuntime struct {
	name      string
	tools     *aggregator.ToolIndex
	resources *aggregator.ResourceIndex
	prompts   *aggregator.PromptIndex
}

func NewControlPlane(profiles map[string]*profileRuntime, callers map[string]string, logs *telemetry.LogBroadcaster) *ControlPlane {
	return &ControlPlane{
		info:     defaultControlPlaneInfo(),
		profiles: profiles,
		callers:  callers,
		logs:     logs,
	}
}

func (c *ControlPlane) Info(ctx context.Context) (domain.ControlPlaneInfo, error) {
	return c.info, nil
}

func (c *ControlPlane) ListTools(ctx context.Context, caller string) (domain.ToolSnapshot, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		return domain.ToolSnapshot{}, err
	}
	if profile.tools == nil {
		return domain.ToolSnapshot{}, nil
	}
	return profile.tools.Snapshot(), nil
}

func (c *ControlPlane) WatchTools(ctx context.Context, caller string) (<-chan domain.ToolSnapshot, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		ch := make(chan domain.ToolSnapshot)
		close(ch)
		return ch, err
	}
	if profile.tools == nil {
		ch := make(chan domain.ToolSnapshot)
		close(ch)
		return ch, nil
	}
	return profile.tools.Subscribe(ctx), nil
}

func (c *ControlPlane) CallTool(ctx context.Context, caller, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		return nil, err
	}
	if profile.tools == nil {
		return nil, domain.ErrToolNotFound
	}
	return profile.tools.CallTool(ctx, name, args, routingKey)
}

func (c *ControlPlane) ListResources(ctx context.Context, caller string, cursor string) (domain.ResourcePage, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	if profile.resources == nil {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}
	snapshot := profile.resources.Snapshot()
	return paginateResources(snapshot, cursor)
}

func (c *ControlPlane) WatchResources(ctx context.Context, caller string) (<-chan domain.ResourceSnapshot, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		ch := make(chan domain.ResourceSnapshot)
		close(ch)
		return ch, err
	}
	if profile.resources == nil {
		ch := make(chan domain.ResourceSnapshot)
		close(ch)
		return ch, nil
	}
	return profile.resources.Subscribe(ctx), nil
}

func (c *ControlPlane) ReadResource(ctx context.Context, caller, uri string) (json.RawMessage, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		return nil, err
	}
	if profile.resources == nil {
		return nil, domain.ErrResourceNotFound
	}
	return profile.resources.ReadResource(ctx, uri)
}

func (c *ControlPlane) ListPrompts(ctx context.Context, caller string, cursor string) (domain.PromptPage, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		return domain.PromptPage{}, err
	}
	if profile.prompts == nil {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}
	snapshot := profile.prompts.Snapshot()
	return paginatePrompts(snapshot, cursor)
}

func (c *ControlPlane) WatchPrompts(ctx context.Context, caller string) (<-chan domain.PromptSnapshot, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		ch := make(chan domain.PromptSnapshot)
		close(ch)
		return ch, err
	}
	if profile.prompts == nil {
		ch := make(chan domain.PromptSnapshot)
		close(ch)
		return ch, nil
	}
	return profile.prompts.Subscribe(ctx), nil
}

func (c *ControlPlane) GetPrompt(ctx context.Context, caller, name string, args json.RawMessage) (json.RawMessage, error) {
	profile, err := c.resolveProfile(caller)
	if err != nil {
		return nil, err
	}
	if profile.prompts == nil {
		return nil, domain.ErrPromptNotFound
	}
	return profile.prompts.GetPrompt(ctx, name, args)
}

func (c *ControlPlane) StreamLogs(ctx context.Context, caller string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
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

func (c *ControlPlane) resolveProfile(caller string) (*profileRuntime, error) {
	profileName := domain.DefaultProfileName
	if caller != "" {
		if mapped, ok := c.callers[caller]; ok {
			profileName = mapped
		}
	}
	profile, ok := c.profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", profileName)
	}
	return profile, nil
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

const listPageSize = 200

func paginateResources(snapshot domain.ResourceSnapshot, cursor string) (domain.ResourcePage, error) {
	resources := snapshot.Resources
	start, err := indexAfterCursor(resources, cursor, func(item domain.ResourceDefinition) string {
		return item.URI
	})
	if err != nil {
		return domain.ResourcePage{}, err
	}
	end := start + listPageSize
	if end > len(resources) {
		end = len(resources)
	}
	page := append([]domain.ResourceDefinition(nil), resources[start:end]...)
	nextCursor := ""
	if end < len(resources) {
		nextCursor = resources[end-1].URI
	}
	return domain.ResourcePage{
		Snapshot: domain.ResourceSnapshot{
			ETag:      snapshot.ETag,
			Resources: page,
		},
		NextCursor: nextCursor,
	}, nil
}

func paginatePrompts(snapshot domain.PromptSnapshot, cursor string) (domain.PromptPage, error) {
	prompts := snapshot.Prompts
	start, err := indexAfterCursor(prompts, cursor, func(item domain.PromptDefinition) string {
		return item.Name
	})
	if err != nil {
		return domain.PromptPage{}, err
	}
	end := start + listPageSize
	if end > len(prompts) {
		end = len(prompts)
	}
	page := append([]domain.PromptDefinition(nil), prompts[start:end]...)
	nextCursor := ""
	if end < len(prompts) {
		nextCursor = prompts[end-1].Name
	}
	return domain.PromptPage{
		Snapshot: domain.PromptSnapshot{
			ETag:    snapshot.ETag,
			Prompts: page,
		},
		NextCursor: nextCursor,
	}, nil
}

func indexAfterCursor[T any](items []T, cursor string, key func(T) string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	for i, item := range items {
		if key(item) == cursor {
			return i + 1, nil
		}
	}
	return 0, domain.ErrInvalidCursor
}

var _ domain.ControlPlane = (*ControlPlane)(nil)
