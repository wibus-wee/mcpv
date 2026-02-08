package activation

import "mcpv/internal/domain"

func ResolveActivationMode(runtime domain.RuntimeConfig, spec domain.ServerSpec) domain.ActivationMode {
	mode := spec.ActivationMode
	if mode == "" {
		mode = runtime.DefaultActivationMode
	}
	if mode == "" {
		mode = domain.DefaultActivationMode
	}
	return mode
}

func ActiveMinReady(spec domain.ServerSpec) int {
	if spec.MinReady < 1 {
		return 1
	}
	return spec.MinReady
}

func BaselineMinReady(runtime domain.RuntimeConfig, spec domain.ServerSpec) int {
	if ResolveActivationMode(runtime, spec) != domain.ActivationAlwaysOn {
		return 0
	}
	return ActiveMinReady(spec)
}

func PolicyStartCause(runtime domain.RuntimeConfig, spec domain.ServerSpec, minReady int) domain.StartCause {
	mode := ResolveActivationMode(runtime, spec)
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

func ClientStartCause(runtime domain.RuntimeConfig, spec domain.ServerSpec, client string, minReady int) domain.StartCause {
	cause := PolicyStartCause(runtime, spec, minReady)
	cause.Reason = domain.StartCauseClientActivate
	cause.Client = client
	return cause
}
