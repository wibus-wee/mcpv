package domain

import "time"

type Metrics interface {
	ObserveRoute(serverType string, duration time.Duration, err error)
	ObserveInstanceStart(specKey string, duration time.Duration, err error)
	ObserveInstanceStop(specKey string, err error)
	SetActiveInstances(specKey string, count int)
	ObserveSubAgentTokens(provider string, model string, tokens int)
	ObserveSubAgentLatency(provider string, model string, duration time.Duration)
	ObserveSubAgentFilterPrecision(provider string, model string, ratio float64)
}
