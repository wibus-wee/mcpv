package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
	"mcpd/internal/infra/telemetry"
)

type ControlPlane struct {
	info             domain.ControlPlaneInfo
	profiles         map[string]*profileRuntime
	callers          map[string]string
	specRegistry     map[string]domain.ServerSpec
	scheduler        domain.Scheduler
	initManager      *ServerInitializationManager
	runtime          domain.RuntimeConfig
	logs             *telemetry.LogBroadcaster
	logger           *zap.Logger
	ctx              context.Context
	profileStore     domain.ProfileStore
	runtimeStatusIdx *aggregator.RuntimeStatusIndex
	serverInitIdx    *aggregator.ServerInitIndex

	mu             sync.Mutex
	activeCallers  map[string]callerState
	profileCounts  map[string]int
	specCounts     map[string]int
	monitorStarted bool
}

type callerState struct {
	pid           int
	profile       string
	lastHeartbeat time.Time
}

type profileRuntime struct {
	name      string
	specKeys  []string
	tools     *aggregator.ToolIndex
	resources *aggregator.ResourceIndex
	prompts   *aggregator.PromptIndex

	mu     sync.Mutex
	active bool
}

func (p *profileRuntime) Activate(ctx context.Context) {
	p.mu.Lock()
	if p.active {
		p.mu.Unlock()
		return
	}
	p.active = true
	p.mu.Unlock()

	if p.tools != nil {
		p.tools.Start(ctx)
	}
	if p.resources != nil {
		p.resources.Start(ctx)
	}
	if p.prompts != nil {
		p.prompts.Start(ctx)
	}
}

func (p *profileRuntime) Deactivate() {
	p.mu.Lock()
	if !p.active {
		p.mu.Unlock()
		return
	}
	p.active = false
	p.mu.Unlock()

	if p.tools != nil {
		p.tools.Stop()
	}
	if p.resources != nil {
		p.resources.Stop()
	}
	if p.prompts != nil {
		p.prompts.Stop()
	}
}

func NewControlPlane(
	ctx context.Context,
	profiles map[string]*profileRuntime,
	callers map[string]string,
	specRegistry map[string]domain.ServerSpec,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	runtime domain.RuntimeConfig,
	store domain.ProfileStore,
	logs *telemetry.LogBroadcaster,
	logger *zap.Logger,
) *ControlPlane {
	if logger == nil {
		logger = zap.NewNop()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if callers == nil {
		callers = map[string]string{}
	}
	return &ControlPlane{
		info:          defaultControlPlaneInfo(),
		profiles:      profiles,
		callers:       callers,
		specRegistry:  specRegistry,
		scheduler:     scheduler,
		initManager:   initManager,
		runtime:       runtime,
		profileStore:  store,
		logs:          logs,
		logger:        logger.Named("control_plane"),
		ctx:           ctx,
		activeCallers: make(map[string]callerState),
		profileCounts: make(map[string]int),
		specCounts:    make(map[string]int),
	}
}

func (c *ControlPlane) StartCallerMonitor(ctx context.Context) {
	interval := time.Duration(c.runtime.CallerCheckSeconds) * time.Second
	if interval <= 0 {
		return
	}

	c.mu.Lock()
	if c.monitorStarted {
		c.mu.Unlock()
		return
	}
	c.monitorStarted = true
	c.mu.Unlock()

	if ctx == nil {
		ctx = c.ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.reapDeadCallers(ctx)
			}
		}
	}()
}

func (c *ControlPlane) Info(ctx context.Context) (domain.ControlPlaneInfo, error) {
	return c.info, nil
}

func (c *ControlPlane) RegisterCaller(ctx context.Context, caller string, pid int) (string, error) {
	if caller == "" {
		return "", errors.New("caller is required")
	}
	if pid <= 0 {
		return "", errors.New("pid must be > 0")
	}

	profileName, err := c.resolveProfileName(caller)
	if err != nil {
		return "", err
	}

	var toStartProfiles []string
	var toStopProfiles []string
	var toActivateSpecs []string
	var toDeactivateSpecs []string
	now := time.Now()

	c.mu.Lock()
	if existing, ok := c.activeCallers[caller]; ok {
		if existing.pid == pid && existing.profile == profileName {
			existing.lastHeartbeat = now
			c.activeCallers[caller] = existing
			c.mu.Unlock()
			return profileName, nil
		}
		if existing.profile == profileName {
			existing.pid = pid
			existing.lastHeartbeat = now
			c.activeCallers[caller] = existing
			c.mu.Unlock()
			return profileName, nil
		}
		c.removeProfileLocked(existing.profile, &toStopProfiles, &toDeactivateSpecs)
	}
	c.activeCallers[caller] = callerState{pid: pid, profile: profileName, lastHeartbeat: now}
	c.addProfileLocked(profileName, &toStartProfiles, &toActivateSpecs)
	c.mu.Unlock()

	toActivateSpecs, toDeactivateSpecs = filterOverlap(toActivateSpecs, toDeactivateSpecs)

	if err := c.activateSpecs(ctx, toActivateSpecs); err != nil {
		_ = c.UnregisterCaller(ctx, caller)
		return "", err
	}
	c.activateProfiles(toStartProfiles)
	c.deactivateProfiles(toStopProfiles)
	_ = c.deactivateSpecs(ctx, toDeactivateSpecs)

	c.logger.Info("caller registered", zap.String("caller", caller), zap.String("profile", profileName), zap.Int("pid", pid))
	return profileName, nil
}

func (c *ControlPlane) UnregisterCaller(ctx context.Context, caller string) error {
	if caller == "" {
		return errors.New("caller is required")
	}

	var toStopProfiles []string
	var toDeactivateSpecs []string

	c.mu.Lock()
	state, ok := c.activeCallers[caller]
	if !ok {
		c.mu.Unlock()
		return domain.ErrCallerNotRegistered
	}
	delete(c.activeCallers, caller)
	c.removeProfileLocked(state.profile, &toStopProfiles, &toDeactivateSpecs)
	c.mu.Unlock()

	c.deactivateProfiles(toStopProfiles)
	deactivateErr := c.deactivateSpecs(ctx, toDeactivateSpecs)
	c.logger.Info("caller unregistered", zap.String("caller", caller), zap.String("profile", state.profile))
	return deactivateErr
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

func (c *ControlPlane) ListToolsAllProfiles(ctx context.Context) (domain.ToolSnapshot, error) {
	profileNames := c.activeProfileNames()
	if len(profileNames) == 0 {
		return domain.ToolSnapshot{}, nil
	}

	merged := make([]domain.ToolDefinition, 0)
	seen := make(map[string]struct{})

	for _, name := range profileNames {
		runtime := c.profiles[name]
		if runtime == nil || runtime.tools == nil {
			continue
		}
		snapshot := runtime.tools.Snapshot()
		for _, tool := range snapshot.Tools {
			key := tool.SpecKey
			if key == "" {
				key = tool.ServerName
			}
			if key == "" {
				key = tool.Name
			}
			dedupeKey := key + "\x00" + tool.Name
			if _, ok := seen[dedupeKey]; ok {
				continue
			}
			seen[dedupeKey] = struct{}{}
			merged = append(merged, tool)
		}
	}

	if len(merged) == 0 {
		return domain.ToolSnapshot{}, nil
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].SpecKey != merged[j].SpecKey {
			return merged[i].SpecKey < merged[j].SpecKey
		}
		if merged[i].Name != merged[j].Name {
			return merged[i].Name < merged[j].Name
		}
		return merged[i].ServerName < merged[j].ServerName
	})

	return domain.ToolSnapshot{
		ETag:  hashTools(merged),
		Tools: merged,
	}, nil
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

func (c *ControlPlane) CallToolAllProfiles(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	profileNames := c.activeProfileNames()
	for _, profileName := range profileNames {
		runtime := c.profiles[profileName]
		if runtime == nil || runtime.tools == nil {
			continue
		}
		result, err := runtime.tools.CallTool(ctx, name, args, routingKey)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, domain.ErrToolNotFound) {
			continue
		}
		return nil, err
	}
	return nil, domain.ErrToolNotFound
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

func (c *ControlPlane) ListResourcesAllProfiles(ctx context.Context, cursor string) (domain.ResourcePage, error) {
	profileNames := c.activeProfileNames()
	if len(profileNames) == 0 {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}

	merged := make([]domain.ResourceDefinition, 0)
	seen := make(map[string]struct{})

	for _, profileName := range profileNames {
		runtime := c.profiles[profileName]
		if runtime == nil || runtime.resources == nil {
			continue
		}
		snapshot := runtime.resources.Snapshot()
		for _, resource := range snapshot.Resources {
			if resource.URI == "" {
				continue
			}
			if _, ok := seen[resource.URI]; ok {
				continue
			}
			seen[resource.URI] = struct{}{}
			merged = append(merged, resource)
		}
	}

	if len(merged) == 0 {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].URI < merged[j].URI })
	snapshot := domain.ResourceSnapshot{
		ETag:      hashResources(merged),
		Resources: merged,
	}
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

func (c *ControlPlane) ReadResourceAllProfiles(ctx context.Context, uri string) (json.RawMessage, error) {
	profileNames := c.activeProfileNames()
	for _, profileName := range profileNames {
		runtime := c.profiles[profileName]
		if runtime == nil || runtime.resources == nil {
			continue
		}
		result, err := runtime.resources.ReadResource(ctx, uri)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, domain.ErrResourceNotFound) {
			continue
		}
		return nil, err
	}
	return nil, domain.ErrResourceNotFound
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

func (c *ControlPlane) ListPromptsAllProfiles(ctx context.Context, cursor string) (domain.PromptPage, error) {
	profileNames := c.activeProfileNames()
	if len(profileNames) == 0 {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}

	merged := make([]domain.PromptDefinition, 0)
	seen := make(map[string]struct{})

	for _, profileName := range profileNames {
		runtime := c.profiles[profileName]
		if runtime == nil || runtime.prompts == nil {
			continue
		}
		snapshot := runtime.prompts.Snapshot()
		for _, prompt := range snapshot.Prompts {
			if prompt.Name == "" {
				continue
			}
			if _, ok := seen[prompt.Name]; ok {
				continue
			}
			seen[prompt.Name] = struct{}{}
			merged = append(merged, prompt)
		}
	}

	if len(merged) == 0 {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })
	snapshot := domain.PromptSnapshot{
		ETag:    hashPrompts(merged),
		Prompts: merged,
	}
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

func (c *ControlPlane) GetPromptAllProfiles(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	profileNames := c.activeProfileNames()
	for _, profileName := range profileNames {
		runtime := c.profiles[profileName]
		if runtime == nil || runtime.prompts == nil {
			continue
		}
		result, err := runtime.prompts.GetPrompt(ctx, name, args)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, domain.ErrPromptNotFound) {
			continue
		}
		return nil, err
	}
	return nil, domain.ErrPromptNotFound
}

func (c *ControlPlane) StreamLogs(ctx context.Context, caller string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	if _, err := c.resolveProfile(caller); err != nil {
		ch := make(chan domain.LogEntry)
		close(ch)
		return ch, err
	}
	return c.streamLogs(ctx, minLevel)
}

func (c *ControlPlane) StreamLogsAllProfiles(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return c.streamLogs(ctx, minLevel)
}

func (c *ControlPlane) streamLogs(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
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
	if caller == "" {
		return nil, domain.ErrCallerNotRegistered
	}
	c.mu.Lock()
	state, ok := c.activeCallers[caller]
	if ok {
		state.lastHeartbeat = time.Now()
		c.activeCallers[caller] = state
	}
	c.mu.Unlock()
	if !ok {
		return nil, domain.ErrCallerNotRegistered
	}
	profile, ok := c.profiles[state.profile]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", state.profile)
	}
	return profile, nil
}

func (c *ControlPlane) resolveProfileName(caller string) (string, error) {
	profileName := domain.DefaultProfileName
	if caller != "" {
		if mapped, ok := c.callers[caller]; ok {
			profileName = mapped
		}
	}
	if _, ok := c.profiles[profileName]; !ok {
		return "", fmt.Errorf("profile %q not found", profileName)
	}
	return profileName, nil
}

func (c *ControlPlane) activeProfileNames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	names := make([]string, 0, len(c.profileCounts))
	for name, count := range c.profileCounts {
		if count > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func paginateResources(snapshot domain.ResourceSnapshot, cursor string) (domain.ResourcePage, error) {
	resources := snapshot.Resources
	start := 0
	if cursor != "" {
		start = indexAfterResourceCursor(resources, cursor)
		if start < 0 {
			return domain.ResourcePage{}, domain.ErrInvalidCursor
		}
	}

	end := start + 200
	if end > len(resources) {
		end = len(resources)
	}
	nextCursor := ""
	if end < len(resources) {
		nextCursor = resources[end-1].URI
	}
	page := domain.ResourceSnapshot{
		ETag:      snapshot.ETag,
		Resources: append([]domain.ResourceDefinition(nil), resources[start:end]...),
	}
	return domain.ResourcePage{Snapshot: page, NextCursor: nextCursor}, nil
}

func paginatePrompts(snapshot domain.PromptSnapshot, cursor string) (domain.PromptPage, error) {
	prompts := snapshot.Prompts
	start := 0
	if cursor != "" {
		start = indexAfterPromptCursor(prompts, cursor)
		if start < 0 {
			return domain.PromptPage{}, domain.ErrInvalidCursor
		}
	}

	end := start + 200
	if end > len(prompts) {
		end = len(prompts)
	}
	nextCursor := ""
	if end < len(prompts) {
		nextCursor = prompts[end-1].Name
	}
	page := domain.PromptSnapshot{
		ETag:    snapshot.ETag,
		Prompts: append([]domain.PromptDefinition(nil), prompts[start:end]...),
	}
	return domain.PromptPage{Snapshot: page, NextCursor: nextCursor}, nil
}

func indexAfterResourceCursor(resources []domain.ResourceDefinition, cursor string) int {
	idx := sort.Search(len(resources), func(i int) bool {
		return resources[i].URI >= cursor
	})
	if idx >= len(resources) || resources[idx].URI != cursor {
		return -1
	}
	return idx + 1
}

func indexAfterPromptCursor(prompts []domain.PromptDefinition, cursor string) int {
	idx := sort.Search(len(prompts), func(i int) bool {
		return prompts[i].Name >= cursor
	})
	if idx >= len(prompts) || prompts[idx].Name != cursor {
		return -1
	}
	return idx + 1
}

func (c *ControlPlane) addProfileLocked(profile string, profileStarts *[]string, specStarts *[]string) {
	runtime, ok := c.profiles[profile]
	if !ok {
		return
	}
	count := c.profileCounts[profile] + 1
	c.profileCounts[profile] = count
	if count == 1 {
		*profileStarts = append(*profileStarts, profile)
	}
	for _, specKey := range runtime.specKeys {
		specCount := c.specCounts[specKey] + 1
		c.specCounts[specKey] = specCount
		if specCount == 1 {
			*specStarts = append(*specStarts, specKey)
		}
	}
}

func (c *ControlPlane) removeProfileLocked(profile string, profileStops *[]string, specStops *[]string) {
	runtime, ok := c.profiles[profile]
	if !ok {
		return
	}
	count := c.profileCounts[profile]
	switch {
	case count <= 1:
		delete(c.profileCounts, profile)
		if count > 0 {
			*profileStops = append(*profileStops, profile)
		}
	default:
		c.profileCounts[profile] = count - 1
	}
	for _, specKey := range runtime.specKeys {
		specCount := c.specCounts[specKey]
		switch {
		case specCount <= 1:
			delete(c.specCounts, specKey)
			if specCount > 0 {
				*specStops = append(*specStops, specKey)
			}
		default:
			c.specCounts[specKey] = specCount - 1
		}
	}
}

func (c *ControlPlane) activateSpecs(ctx context.Context, specKeys []string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	for _, specKey := range order {
		spec, ok := c.specRegistry[specKey]
		if !ok {
			return fmt.Errorf("unknown spec key %q", specKey)
		}
		minReady := spec.MinReady
		if minReady < 1 {
			minReady = 1
		}
		if c.initManager != nil {
			err := c.initManager.SetMinReady(specKey, minReady)
			if err == nil {
				continue
			}
			c.logger.Warn("server init manager failed to set min ready", zap.String("specKey", specKey), zap.Error(err))
		}
		if c.scheduler == nil {
			return errors.New("scheduler not configured")
		}
		if err := c.scheduler.SetDesiredMinReady(ctx, specKey, minReady); err != nil {
			return err
		}
	}
	return nil
}

func (c *ControlPlane) deactivateSpecs(ctx context.Context, specKeys []string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	var firstErr error
	for _, specKey := range order {
		if c.initManager != nil {
			_ = c.initManager.SetMinReady(specKey, 0)
		}
		if c.scheduler == nil {
			if firstErr == nil {
				firstErr = errors.New("scheduler not configured")
			}
			continue
		}
		if err := c.scheduler.StopSpec(ctx, specKey, "caller inactive"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *ControlPlane) activateProfiles(profiles []string) {
	for _, profile := range profiles {
		if runtime, ok := c.profiles[profile]; ok {
			runtime.Activate(c.ctx)
		}
	}
}

func (c *ControlPlane) deactivateProfiles(profiles []string) {
	for _, profile := range profiles {
		if runtime, ok := c.profiles[profile]; ok {
			runtime.Deactivate()
		}
	}
}

func (c *ControlPlane) reapDeadCallers(ctx context.Context) {
	now := time.Now()
	timeout := time.Duration(c.runtime.CallerCheckSeconds*2) * time.Second
	c.mu.Lock()
	callers := make([]string, 0, len(c.activeCallers))
	for caller, state := range c.activeCallers {
		if timeout > 0 && !state.lastHeartbeat.IsZero() && now.Sub(state.lastHeartbeat) <= timeout {
			continue
		}
		if !pidAlive(state.pid) {
			callers = append(callers, caller)
		}
	}
	c.mu.Unlock()

	for _, caller := range callers {
		if err := c.UnregisterCaller(ctx, caller); err != nil {
			c.logger.Warn("caller reap failed", zap.String("caller", caller), zap.Error(err))
		}
	}
}

func filterOverlap(activate []string, deactivate []string) ([]string, []string) {
	if len(activate) == 0 || len(deactivate) == 0 {
		return activate, deactivate
	}
	deactivateSet := make(map[string]struct{}, len(deactivate))
	for _, key := range deactivate {
		deactivateSet[key] = struct{}{}
	}
	filteredActivate := make([]string, 0, len(activate))
	for _, key := range activate {
		if _, ok := deactivateSet[key]; ok {
			delete(deactivateSet, key)
			continue
		}
		filteredActivate = append(filteredActivate, key)
	}
	filteredDeactivate := make([]string, 0, len(deactivateSet))
	for _, key := range deactivate {
		if _, ok := deactivateSet[key]; ok {
			filteredDeactivate = append(filteredDeactivate, key)
		}
	}
	return filteredActivate, filteredDeactivate
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

func hashTools(tools []domain.ToolDefinition) string {
	hasher := sha256.New()
	for _, tool := range tools {
		_, _ = hasher.Write([]byte(tool.Name))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(tool.ToolJSON)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func hashResources(resources []domain.ResourceDefinition) string {
	hasher := sha256.New()
	for _, resource := range resources {
		_, _ = hasher.Write([]byte(resource.URI))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(resource.ResourceJSON)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func hashPrompts(prompts []domain.PromptDefinition) string {
	hasher := sha256.New()
	for _, prompt := range prompts {
		_, _ = hasher.Write([]byte(prompt.Name))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(prompt.PromptJSON)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func (c *ControlPlane) GetProfileStore() domain.ProfileStore {
	return c.profileStore
}

// GetPoolStatus returns the current status of all instance pools.
func (c *ControlPlane) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	if c.scheduler == nil {
		return nil, nil
	}
	return c.scheduler.GetPoolStatus(ctx)
}

func (c *ControlPlane) GetServerInitStatus(ctx context.Context) ([]domain.ServerInitStatus, error) {
	if c.initManager == nil {
		return nil, nil
	}
	return c.initManager.Statuses(), nil
}

// WatchRuntimeStatus returns a channel that receives runtime status snapshots.
func (c *ControlPlane) WatchRuntimeStatus(ctx context.Context, caller string) (<-chan domain.RuntimeStatusSnapshot, error) {
	if _, err := c.resolveProfile(caller); err != nil {
		ch := make(chan domain.RuntimeStatusSnapshot)
		close(ch)
		return ch, err
	}
	if c.runtimeStatusIdx == nil {
		ch := make(chan domain.RuntimeStatusSnapshot)
		close(ch)
		return ch, nil
	}
	return c.runtimeStatusIdx.Subscribe(ctx), nil
}

// WatchRuntimeStatusAllProfiles returns a channel that receives runtime status snapshots.
func (c *ControlPlane) WatchRuntimeStatusAllProfiles(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	if c.runtimeStatusIdx == nil {
		ch := make(chan domain.RuntimeStatusSnapshot)
		close(ch)
		return ch, nil
	}
	return c.runtimeStatusIdx.Subscribe(ctx), nil
}

// WatchServerInitStatus returns a channel that receives server init status snapshots.
func (c *ControlPlane) WatchServerInitStatus(ctx context.Context, caller string) (<-chan domain.ServerInitStatusSnapshot, error) {
	if _, err := c.resolveProfile(caller); err != nil {
		ch := make(chan domain.ServerInitStatusSnapshot)
		close(ch)
		return ch, err
	}
	if c.serverInitIdx == nil {
		ch := make(chan domain.ServerInitStatusSnapshot)
		close(ch)
		return ch, nil
	}
	return c.serverInitIdx.Subscribe(ctx), nil
}

// WatchServerInitStatusAllProfiles returns a channel that receives server init status snapshots.
func (c *ControlPlane) WatchServerInitStatusAllProfiles(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	if c.serverInitIdx == nil {
		ch := make(chan domain.ServerInitStatusSnapshot)
		close(ch)
		return ch, nil
	}
	return c.serverInitIdx.Subscribe(ctx), nil
}

// SetRuntimeStatusIndex sets the runtime status index and starts its refresh worker.
func (c *ControlPlane) SetRuntimeStatusIndex(idx *aggregator.RuntimeStatusIndex) {
	c.runtimeStatusIdx = idx
	if idx != nil {
		go c.runRuntimeStatusWorker()
	}
}

// SetServerInitIndex sets the server init index and starts its refresh worker.
func (c *ControlPlane) SetServerInitIndex(idx *aggregator.ServerInitIndex) {
	c.serverInitIdx = idx
	if idx != nil {
		go c.runServerInitWorker()
	}
}

// runRuntimeStatusWorker periodically refreshes runtime status (500ms intervals).
func (c *ControlPlane) runRuntimeStatusWorker() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.runtimeStatusIdx.Refresh(c.ctx); err != nil {
				c.logger.Warn("runtime status refresh failed", zap.Error(err))
			}
		}
	}
}

// runServerInitWorker periodically refreshes server init status (1s intervals).
func (c *ControlPlane) runServerInitWorker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.serverInitIdx.Refresh(c.ctx); err != nil {
				c.logger.Warn("server init status refresh failed", zap.Error(err))
			}
		}
	}
}
