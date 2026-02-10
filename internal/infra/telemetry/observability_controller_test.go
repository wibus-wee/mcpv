package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"mcpv/internal/domain"
)

func TestResolveObservabilityState(t *testing.T) {
	defaults := ObservabilityControllerOptions{
		DefaultMetricsEnabled: true,
		DefaultHealthzEnabled: false,
	}

	state := resolveObservabilityState(defaults, domain.ObservabilityConfig{
		ListenAddress: "127.0.0.1:9090",
	})
	require.True(t, state.metricsEnabled)
	require.False(t, state.healthzEnabled)
	require.Equal(t, "127.0.0.1:9090", state.addr)

	state = resolveObservabilityState(defaults, domain.ObservabilityConfig{
		ListenAddress:  "",
		MetricsEnabled: boolPtr(false),
		HealthzEnabled: boolPtr(true),
	})
	require.False(t, state.metricsEnabled)
	require.True(t, state.healthzEnabled)
	require.Equal(t, domain.DefaultObservabilityListenAddress, state.addr)
}

func boolPtr(value bool) *bool {
	return &value
}
