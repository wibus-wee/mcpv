package controlplane

import "mcpd/internal/domain"

// API describes the surface exposed to external coordinators (e.g. UI).
type API interface {
	domain.ControlPlaneCoreAPI
	domain.StoreAPI
}
