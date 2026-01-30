package controlplane

import "mcpv/internal/domain"

// API describes the surface exposed to external coordinators (e.g. UI).
type API interface {
	domain.ControlPlaneCoreAPI
	domain.StoreAPI
}
