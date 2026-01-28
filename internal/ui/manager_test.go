package ui

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcpd/internal/app"
	"mcpd/internal/domain"
)

func TestNewManager(t *testing.T) {
	coreApp := &app.App{}
	configPath := "/path/to/config.yaml"

	manager := NewManager(nil, coreApp, configPath)

	assert.NotNil(t, manager)
	assert.Equal(t, CoreStateStopped, manager.coreState)
	assert.NotNil(t, manager.state)
	assert.Equal(t, coreApp, manager.coreApp)
	assert.Equal(t, configPath, manager.configPath)
}

func TestManager_GetState_Initial(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	state, uptime, err := manager.GetState()

	assert.Equal(t, CoreStateStopped, state)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), uptime)
}

func TestManager_GetControlPlane_NotRunning(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	cp, err := manager.GetControlPlane()

	assert.Nil(t, cp)
	assert.NotNil(t, err)

	uiErr, ok := err.(*Error)
	require.True(t, ok, "expected Error")
	assert.Equal(t, ErrCodeCoreNotRunning, uiErr.Code)
}

func TestManager_GetControlPlane_Running(t *testing.T) {
	coreApp := &app.App{}
	manager := NewManager(nil, coreApp, "")

	// Create a mock ControlPlane (use coreApp as it should implement the interface in real use)
	// For testing, we'll need to inject a controlPlane instance
	// Since we can't create one easily, this test validates the error case
	manager.mu.Lock()
	manager.coreState = CoreStateRunning
	// Note: controlPlane is nil, should return error
	manager.mu.Unlock()

	cp, err := manager.GetControlPlane()

	assert.Nil(t, cp)
	assert.NotNil(t, err)

	uiErr, ok := err.(*Error)
	require.True(t, ok, "expected Error")
	assert.Equal(t, ErrCodeInternal, uiErr.Code)
}

func TestManager_SetControlPlane(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	// Create a mock control plane (we'll use nil for this test)
	// In real usage, this would be set from app.ControlPlane
	var mockCP domain.ControlPlane
	manager.SetControlPlane(mockCP)

	// Verify it was set (even though it's nil)
	manager.mu.RLock()
	assert.Equal(t, mockCP, manager.controlPlane)
	manager.mu.RUnlock()
}

func TestManager_Stop_NotRunning(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	err := manager.Stop()

	assert.NotNil(t, err)
	uiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, ErrCodeCoreNotRunning, uiErr.Code)
}

func TestManager_Start_AlreadyRunning(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	// Set to running state
	manager.mu.Lock()
	manager.coreState = CoreStateRunning
	manager.mu.Unlock()

	err := manager.Start(context.Background())

	assert.NotNil(t, err)
	uiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, ErrCodeCoreAlreadyRunning, uiErr.Code)
}

func TestManager_GetSharedState(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	state := manager.GetSharedState()

	assert.NotNil(t, state)
	assert.Same(t, manager.state, state)
}

func TestManager_Shutdown(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	// Create a mock context
	ctx, cancel := context.WithCancel(context.Background())
	manager.mu.Lock()
	manager.coreCtx = ctx
	manager.coreCancel = cancel
	manager.coreState = CoreStateRunning
	manager.mu.Unlock()

	// Should not panic
	assert.NotPanics(t, func() {
		manager.Shutdown()
	})
}

func TestManager_StateTransitions(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	tests := []struct {
		name      string
		setState  CoreState
		wantState CoreState
	}{
		{"initial stopped", CoreStateStopped, CoreStateStopped},
		{"starting", CoreStateStarting, CoreStateStarting},
		{"running", CoreStateRunning, CoreStateRunning},
		{"stopping", CoreStateStopping, CoreStateStopping},
		{"error", CoreStateError, CoreStateError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager.mu.Lock()
			manager.coreState = tt.setState
			manager.mu.Unlock()

			state, _, _ := manager.GetState()
			assert.Equal(t, tt.wantState, state)
		})
	}
}

func TestManager_GetState_WithUptime(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	// Set running state with start time
	manager.mu.Lock()
	manager.coreState = CoreStateRunning
	manager.coreStarted = time.Now().Add(-5 * time.Second)
	manager.mu.Unlock()

	state, uptime, err := manager.GetState()

	assert.Equal(t, CoreStateRunning, state)
	assert.Nil(t, err)
	assert.Greater(t, uptime, int64(4000)) // At least 4 seconds
	assert.Less(t, uptime, int64(6000))    // Less than 6 seconds
}

func TestManager_SetWailsApp(t *testing.T) {
	manager := NewManager(nil, &app.App{}, "")

	// Note: We can't easily create a real *application.App for testing
	// but we verify the method exists and sets the field
	manager.SetWailsApp(nil)

	manager.mu.RLock()
	assert.Nil(t, manager.wails)
	manager.mu.RUnlock()
}
