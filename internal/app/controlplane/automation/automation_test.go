package automation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/app/controlplane/registry"
	"mcpv/internal/domain"
)

// mockSubAgent implements SubAgent for testing.
type mockSubAgent struct{}

func (m *mockSubAgent) SelectToolsForClient(_ context.Context, _ string, _ domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	return domain.AutomaticMCPResult{}, nil
}

func (m *mockSubAgent) InvalidateSession(_ string) {}

func (m *mockSubAgent) Close() error {
	return nil
}

type fakeAutomationState struct {
	runtime domain.RuntimeConfig
}

func (s fakeAutomationState) Runtime() domain.RuntimeConfig {
	return s.runtime
}

func (s fakeAutomationState) Logger() *zap.Logger {
	return zap.NewNop()
}

type fakeRegistryState struct {
	runtime domain.RuntimeConfig
}

func (s fakeRegistryState) Catalog() domain.Catalog {
	return domain.Catalog{}
}

func (s fakeRegistryState) ServerSpecKeys() map[string]string {
	return map[string]string{}
}

func (s fakeRegistryState) SpecRegistry() map[string]domain.ServerSpec {
	return map[string]domain.ServerSpec{}
}

func (s fakeRegistryState) Runtime() domain.RuntimeConfig {
	return s.runtime
}

func (s fakeRegistryState) Logger() *zap.Logger {
	return zap.NewNop()
}

func (s fakeRegistryState) Context() context.Context {
	return context.Background()
}

func (s fakeRegistryState) Scheduler() domain.Scheduler {
	return nil
}

func (s fakeRegistryState) Startup() *bootstrap.ServerStartupOrchestrator {
	return nil
}

// TestHasTagOverlap verifies tag overlap detection.
func TestHasTagOverlap(t *testing.T) {
	tests := []struct {
		name     string
		left     []string
		right    []string
		expected bool
	}{
		{
			name:     "overlapping tags",
			left:     []string{"chat", "web"},
			right:    []string{"web", "admin"},
			expected: true,
		},
		{
			name:     "no overlap",
			left:     []string{"chat"},
			right:    []string{"admin"},
			expected: false,
		},
		{
			name:     "empty left",
			left:     []string{},
			right:    []string{"admin"},
			expected: false,
		},
		{
			name:     "empty right",
			left:     []string{"chat"},
			right:    []string{},
			expected: false,
		},
		{
			name:     "both empty",
			left:     []string{},
			right:    []string{},
			expected: false,
		},
		{
			name:     "multiple overlaps",
			left:     []string{"chat", "web", "admin"},
			right:    []string{"web", "admin"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasTagOverlap(tt.left, tt.right)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsSubAgentEnabled verifies SubAgent enabled check.
func TestIsSubAgentEnabled(t *testing.T) {
	t.Run("enabled when SubAgent set", func(t *testing.T) {
		service := &Service{}
		service.SetSubAgent(&mockSubAgent{})
		assert.True(t, service.IsSubAgentEnabled())
	})

	t.Run("disabled when SubAgent nil", func(t *testing.T) {
		service := &Service{}
		assert.False(t, service.IsSubAgentEnabled())
	})
}

// TestSetSubAgent verifies SubAgent can be set.
func TestSetSubAgent(t *testing.T) {
	service := &Service{}
	assert.Nil(t, service.subAgent)

	agent := &mockSubAgent{}
	service.SetSubAgent(agent)
	assert.NotNil(t, service.subAgent)
	assert.True(t, service.IsSubAgentEnabled())
}

func TestIsSubAgentEnabledForClient(t *testing.T) {
	tests := []struct {
		name         string
		config       domain.SubAgentConfig
		clientTags   []string
		expectEnable bool
	}{
		{
			name: "enabled with matching tag",
			config: domain.SubAgentConfig{
				Enabled:     true,
				EnabledTags: []string{"vscode"},
			},
			clientTags:   []string{"vscode"},
			expectEnable: true,
		},
		{
			name: "enabled with non-matching tag",
			config: domain.SubAgentConfig{
				Enabled:     true,
				EnabledTags: []string{"vscode"},
			},
			clientTags:   []string{"cli"},
			expectEnable: false,
		},
		{
			name: "disabled globally",
			config: domain.SubAgentConfig{
				Enabled:     false,
				EnabledTags: []string{"vscode"},
			},
			clientTags:   []string{"vscode"},
			expectEnable: false,
		},
		{
			name: "empty enabled tags",
			config: domain.SubAgentConfig{
				Enabled:     true,
				EnabledTags: []string{},
			},
			clientTags:   []string{"vscode"},
			expectEnable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := domain.RuntimeConfig{SubAgent: tt.config}
			regState := fakeRegistryState{runtime: runtime}
			reg := registry.NewClientRegistry(regState)

			_, err := reg.RegisterClient(context.Background(), "client", 1, tt.clientTags, "")
			require.NoError(t, err)

			service := NewAutomationService(fakeAutomationState{runtime: runtime}, reg, nil)
			service.SetSubAgent(&mockSubAgent{})

			assert.Equal(t, tt.expectEnable, service.IsSubAgentEnabledForClient("client"))
		})
	}
}
