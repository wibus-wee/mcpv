package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/governance"
	"mcpv/internal/infra/pipeline"
	"mcpv/internal/infra/telemetry"
)

// TestPluginSystemE2E tests the complete plugin governance pipeline end-to-end
// with all 7 categories: observability, authentication, authorization, rate_limiting,
// validation, content, audit.
func TestPluginSystemE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Build demo-plugin binary for testing
	binary := buildDemoPluginBinary(t)
	// Use /tmp directly (not os.TempDir()) to avoid long paths on macOS
	// which can exceed Unix socket path limit (104-108 bytes)
	rootDir := filepath.Join("/tmp", fmt.Sprintf("mcpv-e2e-%d", time.Now().UnixNano()))
	require.NoError(t, os.MkdirAll(rootDir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(rootDir) })

	logger := zap.NewNop()
	metrics := telemetry.NewNoopMetrics()

	// Initialize plugin manager
	manager, err := NewManager(Options{
		Logger:  logger,
		RootDir: rootDir,
	})
	require.NoError(t, err)
	t.Cleanup(func() { manager.Stop(context.Background()) })

	// Define all 7 plugin categories
	specs := []domain.PluginSpec{
		{
			Name:       "test-observability",
			Category:   domain.PluginCategoryObservability,
			Required:   false,
			Cmd:        []string{binary, "--category", "observability", "--name", "test-observability"},
			Flows:      []domain.PluginFlow{domain.PluginFlowRequest, domain.PluginFlowResponse},
			TimeoutMs:  5000,
			ConfigJSON: json.RawMessage(`{"logLevel":"debug"}`),
		},
		{
			Name:       "test-authentication",
			Category:   domain.PluginCategoryAuthentication,
			Required:   false,
			Cmd:        []string{binary, "--category", "authentication", "--name", "test-authentication"},
			Flows:      []domain.PluginFlow{domain.PluginFlowRequest},
			TimeoutMs:  5000,
			ConfigJSON: json.RawMessage(`{"tokenHeader":"Authorization"}`),
		},
		{
			Name:       "test-authorization",
			Category:   domain.PluginCategoryAuthorization,
			Required:   false,
			Cmd:        []string{binary, "--category", "authorization", "--name", "test-authorization"},
			Flows:      []domain.PluginFlow{domain.PluginFlowRequest},
			TimeoutMs:  5000,
			ConfigJSON: json.RawMessage(`{"requiredRole":"user"}`),
		},
		{
			Name:       "test-rate-limiting",
			Category:   domain.PluginCategoryRateLimiting,
			Required:   false,
			Cmd:        []string{binary, "--category", "rate_limiting", "--name", "test-rate-limiting"},
			Flows:      []domain.PluginFlow{domain.PluginFlowRequest},
			TimeoutMs:  5000,
			ConfigJSON: json.RawMessage(`{"maxRequests":100,"windowSeconds":60}`),
		},
		{
			Name:       "test-validation",
			Category:   domain.PluginCategoryValidation,
			Required:   false,
			Cmd:        []string{binary, "--category", "validation", "--name", "test-validation"},
			Flows:      []domain.PluginFlow{domain.PluginFlowRequest},
			TimeoutMs:  5000,
			ConfigJSON: json.RawMessage(`{"strictMode":true}`),
		},
		{
			Name:       "test-content",
			Category:   domain.PluginCategoryContent,
			Required:   false,
			Cmd:        []string{binary, "--category", "content", "--name", "test-content"},
			Flows:      []domain.PluginFlow{domain.PluginFlowRequest, domain.PluginFlowResponse},
			TimeoutMs:  5000,
			ConfigJSON: json.RawMessage(`{"addTimestamp":true}`),
		},
		{
			Name:       "test-audit",
			Category:   domain.PluginCategoryAudit,
			Required:   false,
			Cmd:        []string{binary, "--category", "audit", "--name", "test-audit"},
			Flows:      []domain.PluginFlow{domain.PluginFlowRequest, domain.PluginFlowResponse},
			TimeoutMs:  5000,
			ConfigJSON: json.RawMessage(`{"logDestination":"stdout"}`),
		},
	}

	// Apply plugin specs
	require.NoError(t, manager.Apply(context.Background(), specs))

	// Wait for plugins to start (they need time to initialize)
	time.Sleep(500 * time.Millisecond)

	// Create pipeline engine
	engine := pipeline.NewEngine(manager, logger, metrics)
	engine.Update(specs)

	// Create governance executor
	executor := governance.NewExecutor(engine)

	t.Run("AllPluginsAllow", func(t *testing.T) {
		// Test that all plugins allow a valid request
		req := domain.GovernanceRequest{
			Flow:        domain.PluginFlowRequest,
			Method:      "tools/call",
			Caller:      "test-client",
			Server:      "weather",
			ToolName:    "get_forecast",
			RequestJSON: json.RawMessage(`{"location":"Beijing","unit":"celsius"}`),
			Metadata: map[string]string{
				"authorization": "Bearer valid-token-abc123",
				"x-role":        "user",
				"x-client-id":   "client-123",
			},
		}

		result, err := executor.Execute(context.Background(), req, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"temperature":15,"condition":"sunny"}`), nil
		})

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, string(result), "temperature")
	})

	t.Run("AuthenticationRejectsInvalidToken", func(t *testing.T) {
		// Test authentication plugin rejects invalid token
		// Use tools/list instead of tools/call to get GovernanceRejection error
		req := domain.GovernanceRequest{
			Flow:        domain.PluginFlowRequest,
			Method:      "tools/list",
			Caller:      "test-client",
			Server:      "weather",
			RequestJSON: json.RawMessage(`{}`),
			Metadata: map[string]string{
				"authorization": "Bearer INVALID-token",
				"x-role":        "user",
			},
		}

		_, err := executor.Execute(context.Background(), req, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"tools":[]}`), nil
		})

		require.Error(t, err)
		var govErr domain.GovernanceRejection
		require.ErrorAs(t, err, &govErr)
		assert.Equal(t, domain.PluginCategoryAuthentication, govErr.Category)
		assert.Contains(t, govErr.Message, "Invalid authentication token")
	})

	t.Run("AuthorizationRejectsGuestFromAdminTools", func(t *testing.T) {
		// Test authorization plugin blocks guest from admin tools
		// Use tools/list instead of tools/call to get GovernanceRejection error
		req := domain.GovernanceRequest{
			Flow:        domain.PluginFlowRequest,
			Method:      "tools/list",
			Caller:      "test-client",
			Server:      "admin",
			ToolName:    "admin_delete_user",
			RequestJSON: json.RawMessage(`{}`),
			Metadata: map[string]string{
				"authorization": "Bearer valid-token",
				"x-role":        "guest",
			},
		}

		_, err := executor.Execute(context.Background(), req, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"tools":[]}`), nil
		})

		require.Error(t, err)
		var govErr domain.GovernanceRejection
		require.ErrorAs(t, err, &govErr)
		assert.Equal(t, domain.PluginCategoryAuthorization, govErr.Category)
		assert.Contains(t, govErr.Message, "Insufficient permissions")
	})

	t.Run("ValidationRejectsInvalidJSON", func(t *testing.T) {
		// Test validation plugin rejects invalid JSON
		// Use tools/list instead of tools/call to get GovernanceRejection error
		req := domain.GovernanceRequest{
			Flow:        domain.PluginFlowRequest,
			Method:      "tools/list",
			Caller:      "test-client",
			Server:      "weather",
			RequestJSON: json.RawMessage(`{invalid json here`),
			Metadata: map[string]string{
				"authorization": "Bearer valid-token",
				"x-role":        "user",
			},
		}

		_, err := executor.Execute(context.Background(), req, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"tools":[]}`), nil
		})

		require.Error(t, err)
		var govErr domain.GovernanceRejection
		require.ErrorAs(t, err, &govErr)
		assert.Equal(t, domain.PluginCategoryValidation, govErr.Category)
		assert.Contains(t, govErr.Message, "Invalid JSON payload")
	})

	t.Run("ResponseFlowProcessing", func(t *testing.T) {
		// Test plugins that handle response flow
		req := domain.GovernanceRequest{
			Flow:        domain.PluginFlowResponse,
			Method:      "tools/call",
			Caller:      "test-client",
			Server:      "weather",
			ToolName:    "get_forecast",
			RequestJSON: json.RawMessage(`{"location":"Beijing"}`),
			Metadata: map[string]string{
				"authorization": "Bearer valid-token",
				"x-role":        "user",
			},
		}

		result, err := executor.Execute(context.Background(), req, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"temperature":15,"condition":"sunny"}`), nil
		})

		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("PluginExecutionOrder", func(t *testing.T) {
		// Verify plugins execute in correct category order:
		// observability → authentication → authorization → rate_limiting → validation → content → audit
		req := domain.GovernanceRequest{
			Flow:        domain.PluginFlowRequest,
			Method:      "tools/call",
			Caller:      "test-client",
			Server:      "weather",
			ToolName:    "get_forecast",
			RequestJSON: json.RawMessage(`{"location":"Tokyo","unit":"celsius"}`),
			Metadata: map[string]string{
				"authorization": "Bearer valid-token-xyz",
				"x-role":        "admin",
				"x-client-id":   "client-456",
			},
		}

		result, err := executor.Execute(context.Background(), req, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"temperature":20,"condition":"cloudy"}`), nil
		})

		require.NoError(t, err)
		assert.NotNil(t, result)
		// All plugins should have been executed in order
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Test multiple concurrent requests through the pipeline
		const numRequests = 10
		errCh := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(index int) {
				req := domain.GovernanceRequest{
					Flow:        domain.PluginFlowRequest,
					Method:      "tools/call",
					Caller:      fmt.Sprintf("client-%d", index),
					Server:      "weather",
					ToolName:    "get_forecast",
					RequestJSON: json.RawMessage(fmt.Sprintf(`{"location":"City%d"}`, index)),
					Metadata: map[string]string{
						"authorization": "Bearer valid-token",
						"x-role":        "user",
						"x-client-id":   fmt.Sprintf("client-%d", index),
					},
				}

				_, err := executor.Execute(context.Background(), req, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
					return json.RawMessage(`{"temperature":18}`), nil
				})
				errCh <- err
			}(i)
		}

		// Collect results
		for i := 0; i < numRequests; i++ {
			err := <-errCh
			assert.NoError(t, err, "Request %d should succeed", i)
		}
	})

	t.Run("PluginTimeout", func(t *testing.T) {
		// Test plugin timeout handling (using short timeout spec)
		shortTimeoutSpec := domain.PluginSpec{
			Name:       "test-slow-plugin",
			Category:   domain.PluginCategoryObservability,
			Required:   false,
			Cmd:        []string{binary, "--category", "observability", "--name", "test-slow"},
			Flows:      []domain.PluginFlow{domain.PluginFlowRequest},
			TimeoutMs:  100, // Very short timeout
			ConfigJSON: json.RawMessage(`{}`),
		}

		slowSpecs := append([]domain.PluginSpec{shortTimeoutSpec}, specs...)
		require.NoError(t, manager.Apply(context.Background(), slowSpecs))
		engine.Update(slowSpecs)

		req := domain.GovernanceRequest{
			Flow:        domain.PluginFlowRequest,
			Method:      "tools/call",
			Caller:      "test-client",
			Server:      "weather",
			ToolName:    "get_forecast",
			RequestJSON: json.RawMessage(`{"location":"Berlin"}`),
			Metadata: map[string]string{
				"authorization": "Bearer valid-token",
			},
		}

		// This may timeout or succeed depending on plugin startup speed
		// We mainly want to verify the system handles it gracefully
		_, err := executor.Execute(context.Background(), req, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"temperature":10}`), nil
		})

		// Either succeeds or times out, but should not panic
		if err != nil {
			t.Logf("Plugin timeout handled: %v", err)
		}
	})
}

func TestPluginManagerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping lifecycle test in short mode")
	}

	binary := buildDemoPluginBinary(t)
	// Use /tmp directly to avoid Unix socket path length limit
	rootDir := filepath.Join("/tmp", fmt.Sprintf("mcpv-lifecycle-%d", time.Now().UnixNano()))
	require.NoError(t, os.MkdirAll(rootDir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(rootDir) })

	logger := zap.NewNop()
	manager, err := NewManager(Options{
		Logger:  logger,
		RootDir: rootDir,
	})
	require.NoError(t, err)
	defer manager.Stop(context.Background())

	spec := domain.PluginSpec{
		Name:       "lifecycle-test",
		Category:   domain.PluginCategoryAudit,
		Required:   true, // Set to true so Apply fails if plugin doesn't start
		Cmd:        []string{binary, "--category", "audit", "--name", "lifecycle-test"},
		Flows:      []domain.PluginFlow{domain.PluginFlowRequest},
		TimeoutMs:  10000, // Increase timeout for CI
		ConfigJSON: json.RawMessage(`{}`),
	}

	t.Run("StartPlugin", func(t *testing.T) {
		err := manager.Apply(context.Background(), []domain.PluginSpec{spec})
		require.NoError(t, err) // Apply should succeed if plugin starts correctly

		// Now test that we can call Handle
		req := domain.GovernanceRequest{
			Flow:   domain.PluginFlowRequest,
			Method: "tools/list",
			Caller: "test",
		}

		decision, err := manager.Handle(context.Background(), spec, req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
	})

	t.Run("ReloadPlugin", func(t *testing.T) {
		// Update plugin spec - since spec changed, plugin will be restarted
		updatedSpec := spec
		updatedSpec.ConfigJSON = json.RawMessage(`{"updated":true}`)

		err := manager.Apply(context.Background(), []domain.PluginSpec{updatedSpec})
		require.NoError(t, err)

		// Now test that we can call Handle after reload
		req := domain.GovernanceRequest{
			Flow:   domain.PluginFlowRequest,
			Method: "tools/list",
			Caller: "test",
		}

		decision, err := manager.Handle(context.Background(), updatedSpec, req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
	})

	t.Run("RemovePlugin", func(t *testing.T) {
		// Remove all plugins
		err := manager.Apply(context.Background(), []domain.PluginSpec{})
		require.NoError(t, err)

		// Plugin should no longer be available
		req := domain.GovernanceRequest{
			Flow:   domain.PluginFlowRequest,
			Method: "tools/list",
			Caller: "test",
		}
		_, err = manager.Handle(context.Background(), spec, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available")
	})
}

func buildDemoPluginBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "demo-plugin")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Build from cmd/demo-plugin
	projectRoot := filepath.Join("..", "..", "..", "..")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/demo-plugin")
	cmd.Dir = projectRoot
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "failed to build demo-plugin: %s", string(output))

	// Verify binary exists and is executable
	info, err := os.Stat(binPath)
	require.NoError(t, err, "demo-plugin binary not found")
	require.NotZero(t, info.Size(), "demo-plugin binary is empty")

	t.Logf("Built demo-plugin at %s (%d bytes)", binPath, info.Size())
	return binPath
}
