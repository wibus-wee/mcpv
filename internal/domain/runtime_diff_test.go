package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiffRuntimeConfig_DynamicFields(t *testing.T) {
	prev := RuntimeConfig{
		RouteTimeoutSeconds:    1,
		ToolRefreshConcurrency: 2,
		ExposeTools:            true,
		ToolNamespaceStrategy:  ToolNamespaceStrategyPrefix,
	}
	next := prev
	next.RouteTimeoutSeconds = 10
	next.ToolRefreshConcurrency = 4
	next.ExposeTools = false
	next.ToolNamespaceStrategy = ToolNamespaceStrategyFlat

	diff := DiffRuntimeConfig(prev, next)

	require.Empty(t, diff.RestartRequiredFields)
	require.Contains(t, diff.DynamicFields, "routeTimeoutSeconds")
	require.Contains(t, diff.DynamicFields, "toolRefreshConcurrency")
	require.Contains(t, diff.DynamicFields, "exposeTools")
	require.Contains(t, diff.DynamicFields, "toolNamespaceStrategy")
	require.False(t, diff.IsEmpty())
	require.False(t, diff.RequiresRestart())
}

func TestDiffRuntimeConfig_RestartRequiredFields(t *testing.T) {
	prev := RuntimeConfig{
		Observability: ObservabilityConfig{ListenAddress: "127.0.0.1:9090"},
		RPC: RPCConfig{
			ListenAddress: "127.0.0.1:7000",
		},
		SubAgent:                SubAgentConfig{Provider: "openai", Model: "gpt-4"},
		BootstrapMode:           BootstrapModeMetadata,
		BootstrapConcurrency:    2,
		BootstrapTimeoutSeconds: 10,
		DefaultActivationMode:   ActivationAlwaysOn,
	}
	next := prev
	next.Observability.ListenAddress = "127.0.0.1:9091"
	next.RPC.ListenAddress = "127.0.0.1:7001"
	next.SubAgent = SubAgentConfig{Provider: "openai", Model: "gpt-4o"}
	next.BootstrapMode = BootstrapModeDisabled
	next.BootstrapConcurrency = 4
	next.BootstrapTimeoutSeconds = 20
	next.DefaultActivationMode = ActivationOnDemand

	diff := DiffRuntimeConfig(prev, next)

	require.Contains(t, diff.RestartRequiredFields, "observability")
	require.Contains(t, diff.RestartRequiredFields, "rpc")
	require.Contains(t, diff.RestartRequiredFields, "subAgent")
	require.Contains(t, diff.RestartRequiredFields, "bootstrapMode")
	require.Contains(t, diff.RestartRequiredFields, "bootstrapConcurrency")
	require.Contains(t, diff.RestartRequiredFields, "bootstrapTimeoutSeconds")
	require.Contains(t, diff.RestartRequiredFields, "defaultActivationMode")
	require.True(t, diff.RequiresRestart())
}
