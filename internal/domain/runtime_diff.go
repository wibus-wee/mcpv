package domain

import (
	"reflect"
	"sort"
)

// RuntimeDiff captures runtime-level changes that can be applied dynamically or require restart.
type RuntimeDiff struct {
	DynamicFields         []string
	RestartRequiredFields []string
}

// IsEmpty reports whether the runtime diff contains any changes.
func (d RuntimeDiff) IsEmpty() bool {
	return len(d.DynamicFields) == 0 && len(d.RestartRequiredFields) == 0
}

// RequiresRestart reports whether any runtime changes require a restart.
func (d RuntimeDiff) RequiresRestart() bool {
	return len(d.RestartRequiredFields) > 0
}

// DiffRuntimeConfig compares runtime configs and returns a classification of changed fields.
func DiffRuntimeConfig(prev, next RuntimeConfig) RuntimeDiff {
	diff := RuntimeDiff{}

	if prev.RouteTimeoutSeconds != next.RouteTimeoutSeconds {
		diff.DynamicFields = append(diff.DynamicFields, "routeTimeoutSeconds")
	}
	if prev.PingIntervalSeconds != next.PingIntervalSeconds {
		diff.DynamicFields = append(diff.DynamicFields, "pingIntervalSeconds")
	}
	if prev.ToolRefreshSeconds != next.ToolRefreshSeconds {
		diff.DynamicFields = append(diff.DynamicFields, "toolRefreshSeconds")
	}
	if prev.ToolRefreshConcurrency != next.ToolRefreshConcurrency {
		diff.DynamicFields = append(diff.DynamicFields, "toolRefreshConcurrency")
	}
	if prev.ClientCheckSeconds != next.ClientCheckSeconds {
		diff.DynamicFields = append(diff.DynamicFields, "clientCheckSeconds")
	}
	if prev.ClientInactiveSeconds != next.ClientInactiveSeconds {
		diff.DynamicFields = append(diff.DynamicFields, "clientInactiveSeconds")
	}
	if prev.ServerInitRetryBaseSeconds != next.ServerInitRetryBaseSeconds {
		diff.DynamicFields = append(diff.DynamicFields, "serverInitRetryBaseSeconds")
	}
	if prev.ServerInitRetryMaxSeconds != next.ServerInitRetryMaxSeconds {
		diff.DynamicFields = append(diff.DynamicFields, "serverInitRetryMaxSeconds")
	}
	if prev.ServerInitMaxRetries != next.ServerInitMaxRetries {
		diff.DynamicFields = append(diff.DynamicFields, "serverInitMaxRetries")
	}
	if prev.ReloadMode != next.ReloadMode {
		diff.DynamicFields = append(diff.DynamicFields, "reloadMode")
	}
	if prev.ExposeTools != next.ExposeTools {
		diff.DynamicFields = append(diff.DynamicFields, "exposeTools")
	}
	if prev.ToolNamespaceStrategy != next.ToolNamespaceStrategy {
		diff.DynamicFields = append(diff.DynamicFields, "toolNamespaceStrategy")
	}
	if !reflect.DeepEqual(prev.Observability, next.Observability) {
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "observability")
	}
	if !reflect.DeepEqual(prev.RPC, next.RPC) {
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "rpc")
	}
	if !reflect.DeepEqual(prev.SubAgent, next.SubAgent) {
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "subAgent")
	}
	if prev.BootstrapMode != next.BootstrapMode {
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "bootstrapMode")
	}
	if prev.BootstrapConcurrency != next.BootstrapConcurrency {
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "bootstrapConcurrency")
	}
	if prev.BootstrapTimeoutSeconds != next.BootstrapTimeoutSeconds {
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "bootstrapTimeoutSeconds")
	}
	if prev.DefaultActivationMode != next.DefaultActivationMode {
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "defaultActivationMode")
	}

	sort.Strings(diff.DynamicFields)
	sort.Strings(diff.RestartRequiredFields)
	return diff
}
