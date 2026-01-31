package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/governance"
	"mcpv/internal/infra/pipeline"
	"mcpv/internal/infra/telemetry"
)

func TestPluginManagerHarness(t *testing.T) {
	binary := buildFakePluginBinary(t)
	rootDir := filepath.Join("/tmp", fmt.Sprintf("mcpv-harness-%d", time.Now().UnixNano()))
	require.NoError(t, os.MkdirAll(rootDir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(rootDir) })
	manager, err := NewManager(ManagerOptions{Logger: zap.NewNop(), RootDir: rootDir})
	require.NoError(t, err)
	t.Cleanup(func() { manager.Stop(context.Background()) })

	spec := domain.PluginSpec{
		Name:     "harness-plugin",
		Category: domain.PluginCategoryObservability,
		Required: true,
		Cmd:      []string{binary},
		Flows:    []domain.PluginFlow{domain.PluginFlowRequest},
	}

	require.NoError(t, manager.Apply(context.Background(), []domain.PluginSpec{spec}))

	decision, err := manager.Handle(context.Background(), spec, domain.GovernanceRequest{Method: "tools/list", Caller: "harness"})
	require.NoError(t, err)
	require.True(t, decision.Continue)

	engine := pipeline.NewEngine(manager, zap.NewNop(), telemetry.NewNoopMetrics())
	engine.Update([]domain.PluginSpec{spec})

	executor := governance.NewExecutor(engine)
	raw, err := executor.Execute(context.Background(), domain.GovernanceRequest{Method: "tools/list", Caller: "harness"}, func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, string(raw))
}

func buildFakePluginBinary(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "fake-plugin")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./internal/infra/plugin/testdata/fakeplugin")
	cmd.Dir = filepath.Join("..", "..", "..")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "build plugin: %s", string(output))
	return binPath
}
