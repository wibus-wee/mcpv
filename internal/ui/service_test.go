package ui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/app"
)

func testLogger() *zap.Logger {
	return zap.NewNop()
}

func TestNewWailsService(t *testing.T) {
	coreApp := &app.App{}
	svc := NewWailsService(coreApp, testLogger())

	assert.NotNil(t, svc)
	assert.Equal(t, coreApp, svc.coreApp)
}

func TestWailsService_GetVersion(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	version := svc.GetVersion()

	assert.NotEmpty(t, version)
}

func TestWailsService_Ping(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	result := svc.Ping(context.Background())

	assert.Equal(t, "pong", result)
}

func TestWailsService_SetManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())
	manager := NewManager(nil, &app.App{}, "/path/to/config.yaml")

	svc.SetManager(manager)

	assert.Same(t, manager, svc.manager)
}

func TestWailsService_GetCoreState_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	state := svc.GetCoreState()

	assert.Equal(t, "unknown", state.State)
}

func TestWailsService_GetCoreState_WithManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())
	manager := NewManager(nil, &app.App{}, "")
	svc.SetManager(manager)

	state := svc.GetCoreState()

	assert.Equal(t, string(CoreStateStopped), state.State)
	assert.Equal(t, int64(0), state.Uptime)
}

func TestWailsService_StartCore_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	err := svc.StartCore(context.Background())

	require.Error(t, err)
	uiErr, ok := err.(*UIError)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestWailsService_StopCore_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	err := svc.StopCore()

	require.Error(t, err)
	uiErr, ok := err.(*UIError)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestWailsService_RestartCore_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	err := svc.RestartCore(context.Background())

	require.Error(t, err)
	uiErr, ok := err.(*UIError)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestWailsService_ListTools_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	tools, err := svc.ListTools(context.Background())

	assert.Nil(t, tools)
	require.Error(t, err)
}

func TestWailsService_ListResources_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	resources, err := svc.ListResources(context.Background(), "")

	assert.Nil(t, resources)
	require.Error(t, err)
}

func TestWailsService_ListPrompts_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	prompts, err := svc.ListPrompts(context.Background(), "")

	assert.Nil(t, prompts)
	require.Error(t, err)
}

func TestWailsService_CallTool_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	result, err := svc.CallTool(context.Background(), "test-tool", nil, "")

	assert.Nil(t, result)
	require.Error(t, err)
}

func TestWailsService_ReadResource_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	result, err := svc.ReadResource(context.Background(), "file:///test")

	assert.Nil(t, result)
	require.Error(t, err)
}

func TestWailsService_GetPrompt_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	result, err := svc.GetPrompt(context.Background(), "test-prompt", nil)

	assert.Nil(t, result)
	require.Error(t, err)
}

func TestWailsService_GetInfo_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	info, err := svc.GetInfo(context.Background())

	assert.Empty(t, info.Name)
	require.Error(t, err)
}

func TestWailsService_StartLogStream_NoManager(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	err := svc.StartLogStream(context.Background(), "info")

	require.Error(t, err)
	uiErr, ok := err.(*UIError)
	require.True(t, ok)
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestWailsService_StopLogStream_NoActive(t *testing.T) {
	svc := NewWailsService(&app.App{}, testLogger())

	// Should not panic when no stream is active
	svc.StopLogStream()
}
