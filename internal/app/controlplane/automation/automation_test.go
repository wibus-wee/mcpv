package automation

import (
	"context"
	"testing"

	"mcpv/internal/domain"

	"github.com/stretchr/testify/assert"
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
