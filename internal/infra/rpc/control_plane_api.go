package rpc

import "mcpd/internal/domain"

type ControlPlaneAPI interface {
	domain.ControlPlaneCoreAPI
	domain.AutomationAPI
	domain.TasksAPI
}
