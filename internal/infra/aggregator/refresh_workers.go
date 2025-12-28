package aggregator

import "mcpd/internal/domain"

func refreshWorkerCount(cfg domain.RuntimeConfig, total int) int {
	if total <= 0 {
		return 0
	}
	limit := cfg.ToolRefreshConcurrency
	if limit <= 0 {
		limit = domain.DefaultToolRefreshConcurrency
	}
	if limit > total {
		return total
	}
	return limit
}
