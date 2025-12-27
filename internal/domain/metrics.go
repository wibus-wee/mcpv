package domain

import "time"

type Metrics interface {
	ObserveRoute(serverType string, duration time.Duration, err error)
	ObserveInstanceStart(specKey string, duration time.Duration, err error)
	ObserveInstanceStop(specKey string, err error)
	SetActiveInstances(specKey string, count int)
}
