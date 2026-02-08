package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap/activation"
	"mcpv/internal/domain"
	"mcpv/internal/infra/hashutil"
	"mcpv/internal/infra/mcpcodec"
)

// Manager handles the asynchronous bootstrap process for MCP servers.
// It starts servers temporarily to fetch their metadata (tools, resources, prompts)
// and caches the metadata based on the bootstrap mode.
type Manager struct {
	scheduler domain.Scheduler
	lifecycle domain.Lifecycle
	specs     map[string]domain.ServerSpec
	specKeys  map[string]string // serverType -> specKey
	runtime   domain.RuntimeConfig
	cache     *domain.MetadataCache
	logger    *zap.Logger

	concurrency int
	timeout     time.Duration
	mode        domain.BootstrapMode

	mu            sync.RWMutex
	state         domain.BootstrapState
	progress      domain.BootstrapProgress
	completed     chan struct{}
	completedOnce sync.Once
	started       bool
}

type bootstrapTarget struct {
	specKey string
	spec    domain.ServerSpec
}

// Options configures the Manager.
type Options struct {
	Scheduler   domain.Scheduler
	Lifecycle   domain.Lifecycle
	Specs       map[string]domain.ServerSpec
	SpecKeys    map[string]string
	Runtime     domain.RuntimeConfig
	Cache       *domain.MetadataCache
	Logger      *zap.Logger
	Concurrency int
	Timeout     time.Duration
	Mode        domain.BootstrapMode
}

// NewManager creates a new Manager.
func NewManager(opts Options) *Manager {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = domain.DefaultBootstrapConcurrency
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = time.Duration(domain.DefaultBootstrapTimeoutSeconds) * time.Second
	}

	mode := opts.Mode
	if mode == "" {
		mode = domain.DefaultBootstrapMode
	}

	return &Manager{
		scheduler:   opts.Scheduler,
		lifecycle:   opts.Lifecycle,
		specs:       opts.Specs,
		specKeys:    opts.SpecKeys,
		runtime:     opts.Runtime,
		cache:       opts.Cache,
		logger:      logger.Named("bootstrap"),
		concurrency: concurrency,
		timeout:     timeout,
		mode:        mode,
		state:       domain.BootstrapPending,
		progress: domain.BootstrapProgress{
			State:  domain.BootstrapPending,
			Errors: make(map[string]string),
		},
		completed: make(chan struct{}),
	}
}

// Bootstrap starts the async bootstrap process. Returns immediately.
// Call WaitForCompletion() to block until done.
func (m *Manager) Bootstrap(ctx context.Context) {
	if m.mode == domain.BootstrapModeDisabled {
		m.completeBootstrap(true)
		return
	}

	targets := m.bootstrapTargets()

	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.state = domain.BootstrapRunning
	m.progress.State = domain.BootstrapRunning
	m.progress.Total = len(targets)
	m.mu.Unlock()

	m.logger.Info("bootstrap started",
		zap.Int("total", len(targets)),
		zap.String("mode", string(m.mode)),
		zap.Int("concurrency", m.concurrency),
	)

	go m.run(ctx, targets)
}

func (m *Manager) bootstrapTargets() []bootstrapTarget {
	targets := make(map[string]domain.ServerSpec)
	if len(m.specKeys) == 0 {
		for specKey, spec := range m.specs {
			targets[specKey] = spec
		}
	} else {
		for serverType, specKey := range m.specKeys {
			if specKey == "" {
				continue
			}
			spec, ok := m.specs[specKey]
			if !ok {
				m.logger.Warn("missing spec for server", zap.String("serverType", serverType), zap.String("specKey", specKey))
				continue
			}
			targets[specKey] = spec
		}
	}

	result := make([]bootstrapTarget, 0, len(targets))
	for specKey, spec := range targets {
		result = append(result, bootstrapTarget{specKey: specKey, spec: spec})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].specKey < result[j].specKey })
	return result
}

func (m *Manager) run(ctx context.Context, targets []bootstrapTarget) {
	startTime := time.Now()

	if len(targets) == 0 {
		m.logger.Info("bootstrap completed (no servers)")
		m.completeBootstrap(true)
		return
	}

	type bootstrapResult struct {
		specKey    string
		serverName string
		err        error
	}

	semaphore := make(chan struct{}, m.concurrency)
	results := make(chan bootstrapResult, len(targets))
	var wg sync.WaitGroup

	for _, target := range targets {
		specKey := target.specKey
		spec := target.spec

		wg.Add(1)
		go func(key string, sp domain.ServerSpec) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				results <- bootstrapResult{specKey: key, serverName: sp.Name, err: ctx.Err()}
				return
			}
			defer func() { <-semaphore }()

			m.setCurrentServer(sp.Name)
			err := m.bootstrapOne(ctx, key, sp)
			results <- bootstrapResult{specKey: key, serverName: sp.Name, err: err}
		}(specKey, spec)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		if result.err != nil {
			m.recordFailure(result.specKey, result.err)
			m.logger.Warn("bootstrap server failed",
				zap.String("specKey", result.specKey),
				zap.String("server", result.serverName),
				zap.Error(result.err),
			)
		} else {
			m.recordSuccess()
			m.logger.Info("bootstrap server ready",
				zap.String("specKey", result.specKey),
				zap.String("server", result.serverName),
			)
		}
	}

	m.mu.RLock()
	succeeded := m.progress.Completed
	failed := m.progress.Failed
	m.mu.RUnlock()

	m.logger.Info("bootstrap completed",
		zap.Int("total", len(targets)),
		zap.Int("succeeded", succeeded),
		zap.Int("failed", failed),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	m.completeBootstrap(failed == 0)
}

func (m *Manager) bootstrapOne(ctx context.Context, specKey string, spec domain.ServerSpec) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	instance, err := m.scheduler.AcquireReady(ctx, specKey, "")
	if err != nil {
		if !errors.Is(err, domain.ErrNoReadyInstance) {
			return fmt.Errorf("acquire ready instance: %w", err)
		}
		if m.shouldWaitForReady(spec) {
			waitCtx := ctx
			waitCancel := func() {}
			if !m.poolHasInstances(ctx, specKey) {
				waitCtx, waitCancel = context.WithTimeout(ctx, m.waitBudget())
			}
			waited, waitErr := m.waitForReadyInstance(waitCtx, specKey)
			waitCancel()
			if waitErr == nil {
				instance = waited
				err = nil
			}
		}
		if err != nil {
			causeCtx := domain.WithStartCause(ctx, domain.StartCause{Reason: domain.StartCauseBootstrap})
			instance, err = m.scheduler.Acquire(causeCtx, specKey, "")
			if err != nil {
				return fmt.Errorf("acquire instance: %w", err)
			}
		}
	}
	defer func() {
		_ = m.scheduler.Release(ctx, instance)
	}()

	// Fetch metadata
	if err := m.fetchAndCacheMetadata(ctx, specKey, spec, instance); err != nil {
		return fmt.Errorf("fetch metadata: %w", err)
	}

	return nil
}

func (m *Manager) shouldWaitForReady(spec domain.ServerSpec) bool {
	mode := activation.ResolveActivationMode(m.runtime, spec)
	if mode == domain.ActivationAlwaysOn {
		return true
	}
	return spec.MinReady > 0
}

func (m *Manager) waitBudget() time.Duration {
	budget := 2 * time.Second
	if m.timeout > 0 {
		if m.timeout/4 < budget {
			budget = m.timeout / 4
		}
	}
	if budget < 200*time.Millisecond {
		budget = 200 * time.Millisecond
	}
	return budget
}

func (m *Manager) waitForReadyInstance(ctx context.Context, specKey string) (*domain.Instance, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		inst, err := m.scheduler.AcquireReady(ctx, specKey, "")
		if err == nil {
			return inst, nil
		}
		if !errors.Is(err, domain.ErrNoReadyInstance) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *Manager) poolHasInstances(ctx context.Context, specKey string) bool {
	pools, err := m.scheduler.GetPoolStatus(ctx)
	if err != nil {
		return false
	}
	for _, pool := range pools {
		if pool.SpecKey == specKey {
			return len(pool.Instances) > 0
		}
	}
	return false
}

func (m *Manager) fetchAndCacheMetadata(ctx context.Context, specKey string, spec domain.ServerSpec, instance *domain.Instance) error {
	if instance == nil {
		return errors.New("instance is nil")
	}
	if instance.Conn() == nil {
		return errors.New("instance has no connection")
	}

	// Fetch tools if exposed
	tools, err := m.fetchTools(ctx, instance)
	if err != nil {
		m.logger.Warn("failed to fetch tools", zap.String("specKey", specKey), zap.Error(err))
	} else if len(tools) > 0 {
		// Apply spec filtering and naming
		filteredTools := m.filterAndNameTools(tools, specKey, spec)
		etag := hashutil.ToolETag(m.logger, filteredTools)
		m.cache.SetTools(specKey, filteredTools, etag)
		m.logger.Debug("cached tools", zap.String("specKey", specKey), zap.Int("count", len(filteredTools)))
	}

	// Fetch resources
	resources, err := m.fetchResources(ctx, instance)
	if err != nil {
		m.logger.Warn("failed to fetch resources", zap.String("specKey", specKey), zap.Error(err))
	} else if len(resources) > 0 {
		filteredResources := m.filterAndNameResources(resources, specKey, spec)
		etag := hashutil.ResourceETag(m.logger, filteredResources)
		m.cache.SetResources(specKey, filteredResources, etag)
		m.logger.Debug("cached resources", zap.String("specKey", specKey), zap.Int("count", len(filteredResources)))
	}

	// Fetch prompts
	prompts, err := m.fetchPrompts(ctx, instance)
	if err != nil {
		m.logger.Warn("failed to fetch prompts", zap.String("specKey", specKey), zap.Error(err))
	} else if len(prompts) > 0 {
		filteredPrompts := m.filterAndNamePrompts(prompts, specKey, spec)
		etag := hashutil.PromptETag(m.logger, filteredPrompts)
		m.cache.SetPrompts(specKey, filteredPrompts, etag)
		m.logger.Debug("cached prompts", zap.String("specKey", specKey), zap.Int("count", len(filteredPrompts)))
	}

	return nil
}

func (m *Manager) fetchTools(ctx context.Context, instance *domain.Instance) ([]*mcp.Tool, error) {
	if instance.Conn() == nil {
		return nil, errors.New("instance has no connection")
	}

	params := &mcp.ListToolsParams{}
	payload, err := buildJSONRPCRequest("tools/list", params)
	if err != nil {
		return nil, err
	}

	resp, err := instance.Conn().Call(ctx, payload)
	if err != nil {
		return nil, err
	}

	var result mcp.ListToolsResult
	if err := decodeJSONRPCResult(resp, &result); err != nil {
		return nil, err
	}

	return result.Tools, nil
}

func (m *Manager) fetchResources(ctx context.Context, instance *domain.Instance) ([]*mcp.Resource, error) {
	if instance.Conn() == nil {
		return nil, errors.New("instance has no connection")
	}

	params := &mcp.ListResourcesParams{}
	payload, err := buildJSONRPCRequest("resources/list", params)
	if err != nil {
		return nil, err
	}

	resp, err := instance.Conn().Call(ctx, payload)
	if err != nil {
		return nil, err
	}

	var result mcp.ListResourcesResult
	if err := decodeJSONRPCResult(resp, &result); err != nil {
		return nil, err
	}

	return result.Resources, nil
}

func (m *Manager) fetchPrompts(ctx context.Context, instance *domain.Instance) ([]*mcp.Prompt, error) {
	if instance.Conn() == nil {
		return nil, errors.New("instance has no connection")
	}

	params := &mcp.ListPromptsParams{}
	payload, err := buildJSONRPCRequest("prompts/list", params)
	if err != nil {
		return nil, err
	}

	resp, err := instance.Conn().Call(ctx, payload)
	if err != nil {
		return nil, err
	}

	var result mcp.ListPromptsResult
	if err := decodeJSONRPCResult(resp, &result); err != nil {
		return nil, err
	}

	return result.Prompts, nil
}

func (m *Manager) filterAndNameTools(tools []*mcp.Tool, specKey string, spec domain.ServerSpec) []domain.ToolDefinition {
	allowed := allowedToolNames(spec)
	result := make([]domain.ToolDefinition, 0, len(tools))

	for _, tool := range tools {
		if tool == nil || tool.Name == "" {
			continue
		}
		if !allowed(tool.Name) {
			continue
		}

		def := mcpcodec.ToolFromMCP(tool)
		def.SpecKey = specKey
		def.ServerName = spec.Name
		result = append(result, def)
	}

	return result
}

func (m *Manager) filterAndNameResources(resources []*mcp.Resource, specKey string, spec domain.ServerSpec) []domain.ResourceDefinition {
	result := make([]domain.ResourceDefinition, 0, len(resources))

	for _, resource := range resources {
		if resource == nil || resource.URI == "" {
			continue
		}

		def := mcpcodec.ResourceFromMCP(resource)
		def.SpecKey = specKey
		def.ServerName = spec.Name
		result = append(result, def)
	}

	return result
}

func (m *Manager) filterAndNamePrompts(prompts []*mcp.Prompt, specKey string, spec domain.ServerSpec) []domain.PromptDefinition {
	result := make([]domain.PromptDefinition, 0, len(prompts))

	for _, prompt := range prompts {
		if prompt == nil || prompt.Name == "" {
			continue
		}

		def := mcpcodec.PromptFromMCP(prompt)
		def.SpecKey = specKey
		def.ServerName = spec.Name
		result = append(result, def)
	}

	return result
}

func (m *Manager) setCurrentServer(name string) {
	m.mu.Lock()
	m.progress.Current = name
	m.mu.Unlock()
}

func (m *Manager) recordSuccess() {
	m.mu.Lock()
	m.progress.Completed++
	m.progress.Current = ""
	m.mu.Unlock()
}

func (m *Manager) recordFailure(specKey string, err error) {
	m.mu.Lock()
	m.progress.Failed++
	m.progress.Current = ""
	if m.progress.Errors == nil {
		m.progress.Errors = make(map[string]string)
	}
	m.progress.Errors[specKey] = err.Error()
	m.mu.Unlock()
}

func (m *Manager) completeBootstrap(success bool) {
	m.mu.Lock()
	if m.state == domain.BootstrapCompleted || m.state == domain.BootstrapFailed {
		m.mu.Unlock()
		return
	}
	if success {
		m.state = domain.BootstrapCompleted
		m.progress.State = domain.BootstrapCompleted
	} else {
		m.state = domain.BootstrapFailed
		m.progress.State = domain.BootstrapFailed
	}
	m.mu.Unlock()

	m.completedOnce.Do(func() {
		close(m.completed)
	})
}

// WaitForCompletion blocks until bootstrap completes or context cancels.
func (m *Manager) WaitForCompletion(ctx context.Context) error {
	select {
	case <-m.completed:
		m.mu.RLock()
		state := m.state
		m.mu.RUnlock()

		if state == domain.BootstrapFailed {
			return errors.New("bootstrap failed")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetProgress returns the current bootstrap progress.
func (m *Manager) GetProgress() domain.BootstrapProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	errors := make(map[string]string, len(m.progress.Errors))
	for k, v := range m.progress.Errors {
		errors[k] = v
	}

	return domain.BootstrapProgress{
		State:     m.progress.State,
		Total:     m.progress.Total,
		Completed: m.progress.Completed,
		Failed:    m.progress.Failed,
		Current:   m.progress.Current,
		Errors:    errors,
	}
}

// IsCompleted returns true if bootstrap has finished.
func (m *Manager) IsCompleted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == domain.BootstrapCompleted || m.state == domain.BootstrapFailed
}

// GetCache returns the metadata cache.
func (m *Manager) GetCache() *domain.MetadataCache {
	return m.cache
}

// helper functions

func buildJSONRPCRequest(method string, params any) (json.RawMessage, error) {
	id, err := jsonrpc.MakeID("bootstrap")
	if err != nil {
		return nil, err
	}

	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req := &jsonrpc.Request{
		ID:     id,
		Method: method,
		Params: rawParams,
	}

	return jsonrpc.EncodeMessage(req)
}

func decodeJSONRPCResult(raw json.RawMessage, result any) error {
	msg, err := jsonrpc.DecodeMessage(raw)
	if err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	resp, ok := msg.(*jsonrpc.Response)
	if !ok {
		return errors.New("response is not a response message")
	}

	if resp.Error != nil {
		return fmt.Errorf("rpc error: %w", resp.Error)
	}

	if len(resp.Result) == 0 {
		return errors.New("response missing result")
	}

	return json.Unmarshal(resp.Result, result)
}

func allowedToolNames(spec domain.ServerSpec) func(string) bool {
	if len(spec.ExposeTools) == 0 {
		return func(_ string) bool { return true }
	}

	allowed := make(map[string]struct{}, len(spec.ExposeTools))
	for _, name := range spec.ExposeTools {
		allowed[name] = struct{}{}
	}
	return func(name string) bool {
		_, ok := allowed[name]
		return ok
	}
}
