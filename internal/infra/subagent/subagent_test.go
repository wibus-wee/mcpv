package subagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

// mockChatModel implements model.ToolCallingChatModel for testing.
type mockChatModel struct {
	generateFunc func(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
}

func (m *mockChatModel) Generate(ctx context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, messages)
	}
	return nil, errors.New("not implemented")
}

func (m *mockChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not implemented")
}

func (m *mockChatModel) BindTools(_ []*schema.ToolInfo) error {
	return nil
}

func (m *mockChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// mockControlPlane implements controlPlaneProvider for testing.
type mockControlPlane struct {
	getToolSnapshotFunc func(client string) (domain.ToolSnapshot, error)
}

func (m *mockControlPlane) GetToolSnapshotForClient(client string) (domain.ToolSnapshot, error) {
	if m.getToolSnapshotFunc != nil {
		return m.getToolSnapshotFunc(client)
	}
	return domain.ToolSnapshot{}, nil
}

// TestSelectToolsForClient_LLMFiltering verifies LLM-based tool filtering.
func TestSelectToolsForClient_LLMFiltering(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		availableTools []domain.ToolDefinition
		llmResponse    string
		llmError       error
		expectedNames  []string
		expectError    bool
	}{
		{
			name:  "query provided, LLM selects subset",
			query: "find weather",
			availableTools: []domain.ToolDefinition{
				{Name: "weather", Description: "Get weather info"},
				{Name: "calendar", Description: "Manage calendar"},
				{Name: "email", Description: "Send emails"},
			},
			llmResponse:   `["weather"]`,
			expectedNames: []string{"weather"},
			expectError:   false,
		},
		{
			name:  "no query, return all tools",
			query: "",
			availableTools: []domain.ToolDefinition{
				{Name: "weather", Description: "Get weather info"},
				{Name: "calendar", Description: "Manage calendar"},
			},
			expectedNames: []string{"weather", "calendar"},
			expectError:   false,
		},
		{
			name:  "LLM returns invalid JSON, fallback to all",
			query: "find weather",
			availableTools: []domain.ToolDefinition{
				{Name: "weather", Description: "Get weather info"},
			},
			llmResponse:   `invalid json`,
			expectedNames: []string{"weather"},
			expectError:   false,
		},
		{
			name:  "LLM returns invalid tool names, fallback to all",
			query: "find weather",
			availableTools: []domain.ToolDefinition{
				{Name: "weather", Description: "Get weather info"},
			},
			llmResponse:   `["invalid_tool"]`,
			expectedNames: []string{"weather"},
			expectError:   false,
		},
		{
			name:  "LLM returns empty array",
			query: "find weather",
			availableTools: []domain.ToolDefinition{
				{Name: "weather", Description: "Get weather info"},
			},
			llmResponse:   `[]`,
			expectedNames: []string{},
			expectError:   false,
		},
		{
			name:  "LLM error, fallback to all tools",
			query: "find weather",
			availableTools: []domain.ToolDefinition{
				{Name: "weather", Description: "Get weather info"},
			},
			llmError:      errors.New("LLM service unavailable"),
			expectedNames: []string{"weather"},
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &mockChatModel{
				generateFunc: func(_ context.Context, _ []*schema.Message) (*schema.Message, error) {
					if tt.llmError != nil {
						return nil, tt.llmError
					}
					return &schema.Message{
						Role:    "assistant",
						Content: tt.llmResponse,
					}, nil
				},
			}

			controlPlane := &mockControlPlane{
				getToolSnapshotFunc: func(_ string) (domain.ToolSnapshot, error) {
					return domain.ToolSnapshot{
						Tools: tt.availableTools,
						ETag:  "test-etag",
					}, nil
				},
			}

			agent := &EinoSubAgent{
				config: domain.SubAgentConfig{
					MaxToolsPerRequest: 50,
				},
				model:        model,
				cache:        domain.NewSessionCache(defaultSessionTTL, defaultMaxCacheSize),
				controlPlane: controlPlane,
				metrics:      &mockMetrics{},
				logger:       zap.NewNop(),
			}

			result, err := agent.SelectToolsForClient(context.Background(), "test-client", domain.AutomaticMCPParams{
				Query:     tt.query,
				SessionID: "test-session",
			})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedNames, toolNames(result.Tools))
				assert.Equal(t, "test-etag", result.ETag)
			}
		})
	}
}

// TestSelectToolsForClient_SessionCache verifies session cache deduplication.
func TestSelectToolsForClient_SessionCache(t *testing.T) {
	tools := []domain.ToolDefinition{
		{Name: "weather", Description: "Get weather", InputSchema: map[string]any{"type": "object"}},
	}

	tests := []struct {
		name         string
		sessionID    string
		forceRefresh bool
		firstCall    bool
		expectSend   bool
	}{
		{
			name:       "first request sends full schema",
			sessionID:  "session-1",
			firstCall:  true,
			expectSend: true,
		},
		{
			name:       "cached tools not resent",
			sessionID:  "session-1",
			firstCall:  false,
			expectSend: false,
		},
		{
			name:         "force refresh bypasses cache",
			sessionID:    "session-1",
			forceRefresh: true,
			firstCall:    false,
			expectSend:   true,
		},
		{
			name:       "different session creates new cache entry",
			sessionID:  "session-2",
			firstCall:  true,
			expectSend: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controlPlane := &mockControlPlane{
				getToolSnapshotFunc: func(_ string) (domain.ToolSnapshot, error) {
					return domain.ToolSnapshot{
						Tools: tools,
						ETag:  "test-etag",
					}, nil
				},
			}

			agent := &EinoSubAgent{
				config: domain.SubAgentConfig{
					MaxToolsPerRequest: 50,
				},
				model:        &mockChatModel{},
				cache:        domain.NewSessionCache(defaultSessionTTL, defaultMaxCacheSize),
				controlPlane: controlPlane,
				metrics:      &mockMetrics{},
				logger:       zap.NewNop(),
			}

			// Pre-populate cache if not first call
			if !tt.firstCall {
				_, _ = agent.SelectToolsForClient(context.Background(), "test-client", domain.AutomaticMCPParams{
					SessionID: "session-1",
				})
			}

			result, err := agent.SelectToolsForClient(context.Background(), "test-client", domain.AutomaticMCPParams{
				SessionID:    tt.sessionID,
				ForceRefresh: tt.forceRefresh,
			})

			require.NoError(t, err)
			if tt.expectSend {
				assert.NotEmpty(t, result.Tools, "Expected tools to be sent")
			} else {
				assert.Empty(t, result.Tools, "Expected no tools to be sent (cached)")
			}
		})
	}
}

// TestSelectToolsForClient_MaxToolsLimit verifies max tools enforcement.
func TestSelectToolsForClient_MaxToolsLimit(t *testing.T) {
	tests := []struct {
		name           string
		maxTools       int
		availableTools int
		expectedCount  int
	}{
		{
			name:           "more tools than limit, truncate",
			maxTools:       10,
			availableTools: 20,
			expectedCount:  10,
		},
		{
			name:           "fewer tools than limit, send all",
			maxTools:       10,
			availableTools: 5,
			expectedCount:  5,
		},
		{
			name:           "zero limit uses default",
			maxTools:       0,
			availableTools: 100,
			expectedCount:  50, // defaultMaxToolsReturn
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := make([]domain.ToolDefinition, tt.availableTools)
			for i := 0; i < tt.availableTools; i++ {
				tools[i] = domain.ToolDefinition{
					Name:        "tool" + string(rune('A'+i)),
					Description: "Test tool",
				}
			}

			controlPlane := &mockControlPlane{
				getToolSnapshotFunc: func(_ string) (domain.ToolSnapshot, error) {
					return domain.ToolSnapshot{
						Tools: tools,
						ETag:  "test-etag",
					}, nil
				},
			}

			agent := &EinoSubAgent{
				config: domain.SubAgentConfig{
					MaxToolsPerRequest: tt.maxTools,
				},
				model:        &mockChatModel{},
				cache:        domain.NewSessionCache(defaultSessionTTL, defaultMaxCacheSize),
				controlPlane: controlPlane,
				metrics:      &mockMetrics{},
				logger:       zap.NewNop(),
			}

			result, err := agent.SelectToolsForClient(context.Background(), "test-client", domain.AutomaticMCPParams{
				SessionID: "test-session",
			})

			require.NoError(t, err)
			assert.Equal(t, makeToolNames(tt.expectedCount), toolNames(result.Tools))
		})
	}
}

// TestBuildFilterPrompt verifies prompt construction.
func TestBuildFilterPrompt(t *testing.T) {
	agent := &EinoSubAgent{
		logger: zap.NewNop(),
	}

	summaries := []toolSummary{
		{Name: "weather", Description: "Get weather info"},
		{Name: "calendar", Description: "Manage calendar"},
	}

	prompt := agent.buildFilterPrompt("find weather", summaries)

	assert.Contains(t, prompt, "User task: find weather")
	assert.Contains(t, prompt, "- weather: Get weather info")
	assert.Contains(t, prompt, "- calendar: Manage calendar")
	assert.Contains(t, prompt, "JSON array of tool names")
}

// TestParseSelectedTools verifies response parsing.
func TestParseSelectedTools(t *testing.T) {
	summaries := []toolSummary{
		{Name: "weather", Description: "Get weather"},
		{Name: "calendar", Description: "Manage calendar"},
	}

	tests := []struct {
		name        string
		response    string
		expected    []string
		expectError bool
	}{
		{
			name:        "valid JSON array",
			response:    `["weather"]`,
			expected:    []string{"weather"},
			expectError: false,
		},
		{
			name:        "multiple tools",
			response:    `["weather", "calendar"]`,
			expected:    []string{"weather", "calendar"},
			expectError: false,
		},
		{
			name:        "empty array",
			response:    `[]`,
			expected:    []string{},
			expectError: false,
		},
		{
			name:        "invalid JSON",
			response:    `invalid`,
			expectError: true,
		},
		{
			name:        "invalid tool name",
			response:    `["invalid_tool"]`,
			expectError: true,
		},
		{
			name:        "mixed valid and invalid",
			response:    `["weather", "invalid"]`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &EinoSubAgent{
				logger: zap.NewNop(),
			}

			result, err := agent.parseSelectedTools(tt.response, summaries)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestInvalidateSession verifies session cache invalidation.
func TestInvalidateSession(t *testing.T) {
	agent := &EinoSubAgent{
		cache:  domain.NewSessionCache(defaultSessionTTL, defaultMaxCacheSize),
		logger: zap.NewNop(),
	}

	// Populate cache
	sessionKey := domain.AutomaticMCPSessionKey("test-client", "session-1")
	agent.cache.Update(sessionKey, map[string]string{"tool1": "hash1"})

	// Verify cache has data (NeedsFull returns false when cached)
	assert.False(t, agent.cache.NeedsFull(sessionKey, "tool1", "hash1"))

	// Invalidate using the full session key
	agent.InvalidateSession(sessionKey)

	// Verify cache is cleared (NeedsFull returns true when not cached)
	assert.True(t, agent.cache.NeedsFull(sessionKey, "tool1", "hash1"))
}

// TestCountSchemaProperties verifies property counting.
func TestCountSchemaProperties(t *testing.T) {
	tests := []struct {
		name     string
		schema   any
		expected int
	}{
		{
			name: "object with properties",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "number"},
				},
			},
			expected: 2,
		},
		{
			name: "object without properties",
			schema: map[string]any{
				"type": "object",
			},
			expected: 0,
		},
		{
			name:     "nil schema",
			schema:   nil,
			expected: 0,
		},
		{
			name:     "non-object schema",
			schema:   "string",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countSchemaProperties(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAllToolNames verifies name extraction.
func TestAllToolNames(t *testing.T) {
	summaries := []toolSummary{
		{Name: "weather"},
		{Name: "calendar"},
		{Name: "email"},
	}

	names := allToolNames(summaries)
	assert.Equal(t, []string{"weather", "calendar", "email"}, names)
}

// TestSelectToolsForClient_EmptySnapshot verifies empty tool handling.
func TestSelectToolsForClient_EmptySnapshot(t *testing.T) {
	controlPlane := &mockControlPlane{
		getToolSnapshotFunc: func(_ string) (domain.ToolSnapshot, error) {
			return domain.ToolSnapshot{
				Tools: []domain.ToolDefinition{},
				ETag:  "empty-etag",
			}, nil
		},
	}

	agent := &EinoSubAgent{
		config:       domain.SubAgentConfig{},
		model:        &mockChatModel{},
		cache:        domain.NewSessionCache(defaultSessionTTL, defaultMaxCacheSize),
		controlPlane: controlPlane,
		metrics:      &mockMetrics{},
		logger:       zap.NewNop(),
	}

	result, err := agent.SelectToolsForClient(context.Background(), "test-client", domain.AutomaticMCPParams{
		Query:     "test query",
		SessionID: "test-session",
	})

	require.NoError(t, err)
	assert.Empty(t, result.Tools)
	assert.Equal(t, "empty-etag", result.ETag)
}

// TestSelectToolsForClient_ControlPlaneError verifies error handling.
func TestSelectToolsForClient_ControlPlaneError(t *testing.T) {
	controlPlane := &mockControlPlane{
		getToolSnapshotFunc: func(_ string) (domain.ToolSnapshot, error) {
			return domain.ToolSnapshot{}, errors.New("control plane unavailable")
		},
	}

	agent := &EinoSubAgent{
		config:       domain.SubAgentConfig{},
		model:        &mockChatModel{},
		cache:        domain.NewSessionCache(defaultSessionTTL, defaultMaxCacheSize),
		controlPlane: controlPlane,
		metrics:      &mockMetrics{},
		logger:       zap.NewNop(),
	}

	_, err := agent.SelectToolsForClient(context.Background(), "test-client", domain.AutomaticMCPParams{
		SessionID: "test-session",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get tool snapshot")
}

func makeToolNames(count int) []string {
	names := make([]string, count)
	for i := 0; i < count; i++ {
		names[i] = "tool" + string(rune('A'+i))
	}
	return names
}

func toolNames(tools []domain.ToolDefinition) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}

// mockMetrics implements domain.Metrics for testing.
type mockMetrics struct{}

func (m *mockMetrics) ObserveRoute(_ domain.RouteMetric)                                       {}
func (m *mockMetrics) AddInflightRoutes(_ string, _ int)                                       {}
func (m *mockMetrics) ObservePoolWait(_ string, _ time.Duration, _ domain.PoolWaitOutcome)     {}
func (m *mockMetrics) ObserveInstanceStart(_ string, _ time.Duration, _ error)                 {}
func (m *mockMetrics) ObserveInstanceStartCause(_ string, _ domain.StartCauseReason)           {}
func (m *mockMetrics) ObserveInstanceStop(_ string, _ error)                                   {}
func (m *mockMetrics) SetStartingInstances(_ string, _ int)                                    {}
func (m *mockMetrics) SetActiveInstances(_ string, _ int)                                      {}
func (m *mockMetrics) SetPoolCapacityRatio(_ string, _ float64)                                {}
func (m *mockMetrics) SetPoolWaiters(_ string, _ int)                                          {}
func (m *mockMetrics) ObservePoolAcquireFailure(_ string, _ domain.AcquireFailureReason)       {}
func (m *mockMetrics) ObserveSubAgentTokens(_ string, _ string, _ int)                         {}
func (m *mockMetrics) ObserveSubAgentLatency(_ string, _ string, _ time.Duration)              {}
func (m *mockMetrics) ObserveSubAgentFilterPrecision(_ string, _ string, _ float64)            {}
func (m *mockMetrics) RecordReloadSuccess(_ domain.CatalogUpdateSource, _ domain.ReloadAction) {}
func (m *mockMetrics) RecordReloadFailure(_ domain.CatalogUpdateSource, _ domain.ReloadAction) {}
func (m *mockMetrics) RecordReloadRestart(_ domain.CatalogUpdateSource, _ domain.ReloadAction) {}
func (m *mockMetrics) ObserveReloadApply(_ domain.ReloadApplyMetric)                           {}
func (m *mockMetrics) ObserveReloadRollback(_ domain.ReloadRollbackMetric)                     {}
func (m *mockMetrics) RecordGovernanceOutcome(_ domain.GovernanceOutcomeMetric)                {}
func (m *mockMetrics) RecordGovernanceRejection(_ domain.GovernanceRejectionMetric)            {}
func (m *mockMetrics) RecordPluginStart(_ domain.PluginStartMetric)                            {}
func (m *mockMetrics) RecordPluginHandshake(_ domain.PluginHandshakeMetric)                    {}
func (m *mockMetrics) SetPluginRunning(_ domain.PluginCategory, _ string, _ bool)              {}
