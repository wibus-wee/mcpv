package rpc

import "mcpv/internal/domain"

type ControlPlaneAPI interface {
	domain.ControlPlaneCoreAPI
	domain.AutomationAPI
	domain.TasksAPI
}
