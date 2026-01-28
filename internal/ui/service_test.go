package ui

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/app"
	"mcpd/internal/domain"
)

func testLogger() *zap.Logger {
	return zap.NewNop()
}

func TestNewServiceRegistry(t *testing.T) {
	registry := NewServiceRegistry(&app.App{}, testLogger())

	require.NotNil(t, registry)
	assert.NotNil(t, registry.System)
	assert.NotNil(t, registry.Core)
	assert.NotNil(t, registry.Discovery)
	assert.NotNil(t, registry.Log)
	assert.NotNil(t, registry.Config)
	assert.NotNil(t, registry.Server)
	assert.NotNil(t, registry.Runtime)
	assert.NotNil(t, registry.SubAgent)
	assert.NotNil(t, registry.Debug)
}

func TestSystemService_GetVersion(t *testing.T) {
	svc := NewSystemService(NewServiceDeps(&app.App{}, testLogger()))

	version := svc.GetVersion()

	assert.NotEmpty(t, version)
}

func TestSystemService_Ping(t *testing.T) {
	svc := NewSystemService(NewServiceDeps(&app.App{}, testLogger()))

	result := svc.Ping(context.Background())

	assert.Equal(t, "pong", result)
}

func TestCoreService_GetCoreState_NoManager(t *testing.T) {
	svc := NewCoreService(NewServiceDeps(&app.App{}, testLogger()))

	state := svc.GetCoreState()

	assert.Equal(t, "unknown", state.State)
}

func TestCoreService_GetCoreState_WithManager(t *testing.T) {
	deps := NewServiceDeps(&app.App{}, testLogger())
	manager := NewManager(nil, &app.App{}, "")
	deps.setManager(manager)
	service := NewCoreService(deps)

	state := service.GetCoreState()

	assert.Equal(t, string(CoreStateStopped), state.State)
	assert.Equal(t, int64(0), state.Uptime)
}

func TestCoreService_GetBootstrapProgress_NoManager(t *testing.T) {
	svc := NewCoreService(NewServiceDeps(&app.App{}, testLogger()))

	progress, err := svc.GetBootstrapProgress(context.Background())

	require.NoError(t, err)
	assert.Equal(t, string(domain.BootstrapPending), progress.State)
}

func TestCoreService_GetBootstrapProgress_CoreNotRunning(t *testing.T) {
	deps := NewServiceDeps(&app.App{}, testLogger())
	manager := NewManager(nil, &app.App{}, "")
	deps.setManager(manager)
	svc := NewCoreService(deps)

	progress, err := svc.GetBootstrapProgress(context.Background())

	require.NoError(t, err)
	assert.Equal(t, string(domain.BootstrapPending), progress.State)
}

func TestCoreService_StartCore_NoManager(t *testing.T) {
	svc := NewCoreService(NewServiceDeps(&app.App{}, testLogger()))

	err := svc.StartCore(context.Background())

	require.Error(t, err)
	uiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestCoreService_StopCore_NoManager(t *testing.T) {
	svc := NewCoreService(NewServiceDeps(&app.App{}, testLogger()))

	err := svc.StopCore()

	require.Error(t, err)
	uiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestCoreService_RestartCore_NoManager(t *testing.T) {
	svc := NewCoreService(NewServiceDeps(&app.App{}, testLogger()))

	err := svc.RestartCore(context.Background())

	require.Error(t, err)
	uiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestDiscoveryService_ListTools_NoManager(t *testing.T) {
	svc := NewDiscoveryService(NewServiceDeps(&app.App{}, testLogger()))

	tools, err := svc.ListTools(context.Background())

	assert.Nil(t, tools)
	require.Error(t, err)
}

func TestDiscoveryService_ListResources_NoManager(t *testing.T) {
	svc := NewDiscoveryService(NewServiceDeps(&app.App{}, testLogger()))

	resources, err := svc.ListResources(context.Background(), "")

	assert.Nil(t, resources)
	require.Error(t, err)
}

func TestDiscoveryService_ListPrompts_NoManager(t *testing.T) {
	svc := NewDiscoveryService(NewServiceDeps(&app.App{}, testLogger()))

	prompts, err := svc.ListPrompts(context.Background(), "")

	assert.Nil(t, prompts)
	require.Error(t, err)
}

func TestDiscoveryService_CallTool_NoManager(t *testing.T) {
	svc := NewDiscoveryService(NewServiceDeps(&app.App{}, testLogger()))

	result, err := svc.CallTool(context.Background(), "test-tool", nil, "")

	assert.Nil(t, result)
	require.Error(t, err)
}

func TestDiscoveryService_ReadResource_NoManager(t *testing.T) {
	svc := NewDiscoveryService(NewServiceDeps(&app.App{}, testLogger()))

	result, err := svc.ReadResource(context.Background(), "file:///test")

	assert.Nil(t, result)
	require.Error(t, err)
}

func TestDiscoveryService_GetPrompt_NoManager(t *testing.T) {
	svc := NewDiscoveryService(NewServiceDeps(&app.App{}, testLogger()))

	result, err := svc.GetPrompt(context.Background(), "test-prompt", nil)

	assert.Nil(t, result)
	require.Error(t, err)
}

func TestServerService_ListServerGroups_NoManager(t *testing.T) {
	svc := NewServerService(NewServiceDeps(&app.App{}, testLogger()))

	groups, err := svc.ListServerGroups(context.Background())

	assert.Nil(t, groups)
	require.Error(t, err)
}

func TestCoreService_GetInfo_NoManager(t *testing.T) {
	svc := NewCoreService(NewServiceDeps(&app.App{}, testLogger()))

	info, err := svc.GetInfo(context.Background())

	assert.Empty(t, info.Name)
	require.Error(t, err)
}

func TestLogService_StartLogStream_NoManager(t *testing.T) {
	svc := NewLogService(NewServiceDeps(&app.App{}, testLogger()))

	err := svc.StartLogStream(context.Background(), "info")

	require.Error(t, err)
	uiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestLogService_StopLogStream_NoActive(t *testing.T) {
	svc := NewLogService(NewServiceDeps(&app.App{}, testLogger()))

	// Should not panic when no stream is active
	assert.NotPanics(t, func() {
		svc.StopLogStream()
	})
}

func TestLogService_StartLogStream_RestartsExisting(t *testing.T) {
	cp := &fakeControlPlane{}
	svc := newLogServiceWithControlPlane(cp)

	err := svc.StartLogStream(context.Background(), "info")
	require.NoError(t, err)

	err = svc.StartLogStream(context.Background(), "debug")
	require.NoError(t, err)

	streams := cp.streamContexts()
	require.Len(t, streams, 2)

	waitForContextDone(streams[0], t, time.Second)

	select {
	case <-streams[1].Done():
		t.Fatalf("expected new stream context to remain active")
	default:
	}
}

func TestLogService_StopLogStream_CancelsSession(t *testing.T) {
	cp := &fakeControlPlane{}
	svc := newLogServiceWithControlPlane(cp)

	err := svc.StartLogStream(context.Background(), "info")
	require.NoError(t, err)

	streams := cp.streamContexts()
	require.Len(t, streams, 1)

	svc.StopLogStream()

	waitForContextDone(streams[0], t, time.Second)
}

func newLogServiceWithControlPlane(cp app.ControlPlaneAPI) *LogService {
	manager := newRunningManagerForTest(cp)
	deps := NewServiceDeps(&app.App{}, testLogger())
	deps.setManager(manager)

	return NewLogService(deps)
}

func newRunningManagerForTest(cp app.ControlPlaneAPI) *Manager {
	manager := NewManager(nil, &app.App{}, "")
	manager.mu.Lock()
	manager.coreState = CoreStateRunning
	manager.controlPlane = cp
	manager.mu.Unlock()
	return manager
}

func waitForContextDone(ctx context.Context, t *testing.T, timeout time.Duration) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		t.Fatalf("expected context to be canceled within %s", timeout)
	}
}

type fakeControlPlane struct {
	mu      sync.Mutex
	streams []context.Context
}

func (f *fakeControlPlane) streamContexts() []context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]context.Context, len(f.streams))
	copy(out, f.streams)
	return out
}

func (f *fakeControlPlane) recordStream(ctx context.Context) {
	f.mu.Lock()
	f.streams = append(f.streams, ctx)
	f.mu.Unlock()
}

func (f *fakeControlPlane) Info(_ context.Context) (domain.ControlPlaneInfo, error) {
	return domain.ControlPlaneInfo{}, nil
}

func (f *fakeControlPlane) RegisterClient(_ context.Context, client string, _ int, _ []string, _ string) (domain.ClientRegistration, error) {
	return domain.ClientRegistration{Client: client}, nil
}

func (f *fakeControlPlane) UnregisterClient(_ context.Context, _ string) error {
	return nil
}

func (f *fakeControlPlane) ListActiveClients(_ context.Context) ([]domain.ActiveClient, error) {
	return nil, nil
}

func (f *fakeControlPlane) WatchActiveClients(_ context.Context) (<-chan domain.ActiveClientSnapshot, error) {
	ch := make(chan domain.ActiveClientSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) ListTools(_ context.Context, _ string) (domain.ToolSnapshot, error) {
	return domain.ToolSnapshot{}, nil
}

func (f *fakeControlPlane) ListToolCatalog(_ context.Context) (domain.ToolCatalogSnapshot, error) {
	return domain.ToolCatalogSnapshot{}, nil
}

func (f *fakeControlPlane) WatchTools(_ context.Context, _ string) (<-chan domain.ToolSnapshot, error) {
	ch := make(chan domain.ToolSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) CallTool(_ context.Context, _, _ string, _ json.RawMessage, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`), nil
}

func (f *fakeControlPlane) CallToolAll(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	return f.CallTool(ctx, "", name, args, routingKey)
}

func (f *fakeControlPlane) ListResources(_ context.Context, _ string, _ string) (domain.ResourcePage, error) {
	return domain.ResourcePage{}, nil
}

func (f *fakeControlPlane) ListResourcesAll(_ context.Context, _ string) (domain.ResourcePage, error) {
	return domain.ResourcePage{}, nil
}

func (f *fakeControlPlane) WatchResources(_ context.Context, _ string) (<-chan domain.ResourceSnapshot, error) {
	ch := make(chan domain.ResourceSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) ReadResource(_ context.Context, _, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{"contents":[{"uri":"file:///a","text":"ok"}]}`), nil
}

func (f *fakeControlPlane) ReadResourceAll(ctx context.Context, uri string) (json.RawMessage, error) {
	return f.ReadResource(ctx, "", uri)
}

func (f *fakeControlPlane) ListPrompts(_ context.Context, _ string, _ string) (domain.PromptPage, error) {
	return domain.PromptPage{}, nil
}

func (f *fakeControlPlane) ListPromptsAll(_ context.Context, _ string) (domain.PromptPage, error) {
	return domain.PromptPage{}, nil
}

func (f *fakeControlPlane) WatchPrompts(_ context.Context, _ string) (<-chan domain.PromptSnapshot, error) {
	ch := make(chan domain.PromptSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) GetPrompt(_ context.Context, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"messages":[{"role":"user","content":{"type":"text","text":"ok"}}]}`), nil
}

func (f *fakeControlPlane) GetPromptAll(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	return f.GetPrompt(ctx, "", name, args)
}

func (f *fakeControlPlane) StreamLogs(ctx context.Context, _ string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return f.StreamLogsAllServers(ctx, minLevel)
}

func (f *fakeControlPlane) StreamLogsAllServers(ctx context.Context, _ domain.LogLevel) (<-chan domain.LogEntry, error) {
	f.recordStream(ctx)
	ch := make(chan domain.LogEntry)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func (f *fakeControlPlane) GetCatalog() domain.Catalog {
	return domain.Catalog{}
}

func (f *fakeControlPlane) GetPoolStatus(_ context.Context) ([]domain.PoolInfo, error) {
	return nil, nil
}

func (f *fakeControlPlane) GetServerInitStatus(_ context.Context) ([]domain.ServerInitStatus, error) {
	return nil, nil
}

func (f *fakeControlPlane) GetBootstrapProgress(_ context.Context) (domain.BootstrapProgress, error) {
	return domain.BootstrapProgress{State: domain.BootstrapCompleted}, nil
}

func (f *fakeControlPlane) WatchBootstrapProgress(_ context.Context) (<-chan domain.BootstrapProgress, error) {
	ch := make(chan domain.BootstrapProgress)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) RetryServerInit(_ context.Context, _ string) error {
	return nil
}

func (f *fakeControlPlane) WatchRuntimeStatus(_ context.Context, _ string) (<-chan domain.RuntimeStatusSnapshot, error) {
	ch := make(chan domain.RuntimeStatusSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) WatchRuntimeStatusAllServers(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	return f.WatchRuntimeStatus(ctx, "")
}

func (f *fakeControlPlane) WatchServerInitStatus(_ context.Context, _ string) (<-chan domain.ServerInitStatusSnapshot, error) {
	ch := make(chan domain.ServerInitStatusSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) WatchServerInitStatusAllServers(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	return f.WatchServerInitStatus(ctx, "")
}

func (f *fakeControlPlane) AutomaticMCP(_ context.Context, _ string, _ domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	return domain.AutomaticMCPResult{}, nil
}

func (f *fakeControlPlane) AutomaticEval(ctx context.Context, client string, params domain.AutomaticEvalParams) (json.RawMessage, error) {
	return f.CallTool(ctx, client, params.ToolName, params.Arguments, params.RoutingKey)
}

func (f *fakeControlPlane) IsSubAgentEnabled() bool {
	return false
}

func (f *fakeControlPlane) IsSubAgentEnabledForClient(_ string) bool {
	return false
}

func (f *fakeControlPlane) CallToolTask(_ context.Context, _, _ string, _ json.RawMessage, _ string, _ domain.TaskCreateOptions) (domain.Task, error) {
	return domain.Task{}, nil
}

func (f *fakeControlPlane) GetTask(_ context.Context, _, _ string) (domain.Task, error) {
	return domain.Task{}, nil
}

func (f *fakeControlPlane) ListTasks(_ context.Context, _, _ string, _ int) (domain.TaskPage, error) {
	return domain.TaskPage{}, nil
}

func (f *fakeControlPlane) GetTaskResult(_ context.Context, _, _ string) (domain.TaskResult, error) {
	return domain.TaskResult{}, nil
}

func (f *fakeControlPlane) CancelTask(_ context.Context, _, _ string) (domain.Task, error) {
	return domain.Task{}, nil
}
