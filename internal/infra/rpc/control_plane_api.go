package rpc

import "mcpv/internal/domain"

type ControlPlaneAPI interface {
	domain.ControlPlaneCoreAPI
	domain.StoreAPI
	domain.AutomationAPI
	domain.TasksAPI
}
