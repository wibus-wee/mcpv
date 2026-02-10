package app

func resolveObservabilityDefaults(opts *ObservabilityOptions) (bool, bool) {
	metricsEnabled := envBool("mcpv_METRICS_ENABLED")
	healthzEnabled := envBool("mcpv_HEALTHZ_ENABLED")
	if opts != nil {
		if opts.MetricsEnabled != nil {
			metricsEnabled = *opts.MetricsEnabled
		}
		if opts.HealthzEnabled != nil {
			healthzEnabled = *opts.HealthzEnabled
		}
	}
	return metricsEnabled, healthzEnabled
}
