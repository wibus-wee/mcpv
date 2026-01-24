package ui

import (
	"context"
	"testing"

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
	uiErr, ok := err.(*UIError)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestCoreService_StopCore_NoManager(t *testing.T) {
	svc := NewCoreService(NewServiceDeps(&app.App{}, testLogger()))

	err := svc.StopCore()

	require.Error(t, err)
	uiErr, ok := err.(*UIError)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestCoreService_RestartCore_NoManager(t *testing.T) {
	svc := NewCoreService(NewServiceDeps(&app.App{}, testLogger()))

	err := svc.RestartCore(context.Background())

	require.Error(t, err)
	uiErr, ok := err.(*UIError)
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
	uiErr, ok := err.(*UIError)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestLogService_StopLogStream_NoActive(t *testing.T) {
	svc := NewLogService(NewServiceDeps(&app.App{}, testLogger()))

	// Should not panic when no stream is active
	svc.StopLogStream()
}
