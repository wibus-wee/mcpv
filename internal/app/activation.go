package app

import "mcpd/internal/domain"

func resolveActivationMode(runtime domain.RuntimeConfig, spec domain.ServerSpec) domain.ActivationMode {
	mode := spec.ActivationMode
	if mode == "" {
		mode = runtime.DefaultActivationMode
	}
	if mode == "" {
		mode = domain.DefaultActivationMode
	}
	return mode
}

func activeMinReady(spec domain.ServerSpec) int {
	if spec.MinReady < 1 {
		return 1
	}
	return spec.MinReady
}

func baselineMinReady(runtime domain.RuntimeConfig, spec domain.ServerSpec) int {
	if resolveActivationMode(runtime, spec) != domain.ActivationAlwaysOn {
		return 0
	}
	return activeMinReady(spec)
}

func policyStartCause(runtime domain.RuntimeConfig, spec domain.ServerSpec, minReady int) domain.StartCause {
	mode := resolveActivationMode(runtime, spec)
	reason := domain.StartCausePolicyMinReady
	if mode == domain.ActivationAlwaysOn && minReady <= 1 {
		reason = domain.StartCausePolicyAlwaysOn
	}
	return domain.StartCause{
		Reason: reason,
		Policy: &domain.StartCausePolicy{
			ActivationMode: mode,
			MinReady:       minReady,
		},
	}
}

func clientStartCause(runtime domain.RuntimeConfig, spec domain.ServerSpec, client string, minReady int) domain.StartCause {
	cause := policyStartCause(runtime, spec, minReady)
	cause.Reason = domain.StartCauseClientActivate
	cause.Client = client
	return cause
}
