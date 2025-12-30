package ui

import (
	"time"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mapping"
)

func mapToolEntries(snapshot domain.ToolSnapshot) []ToolEntry {
	return mapping.MapSlice(snapshot.Tools, func(tool domain.ToolDefinition) ToolEntry {
		return ToolEntry{
			Name:       tool.Name,
			ToolJSON:   tool.ToolJSON,
			SpecKey:    tool.SpecKey,
			ServerName: tool.ServerName,
		}
	})
}

func mapResourcePage(page domain.ResourcePage) *ResourcePage {
	return &ResourcePage{
		NextCursor: page.NextCursor,
		Resources: mapping.MapSlice(page.Snapshot.Resources, func(resource domain.ResourceDefinition) ResourceEntry {
			return ResourceEntry{
				URI:          resource.URI,
				ResourceJSON: resource.ResourceJSON,
			}
		}),
	}
}

func mapPromptPage(page domain.PromptPage) *PromptPage {
	return &PromptPage{
		NextCursor: page.NextCursor,
		Prompts: mapping.MapSlice(page.Snapshot.Prompts, func(prompt domain.PromptDefinition) PromptEntry {
			return PromptEntry{
				Name:       prompt.Name,
				PromptJSON: prompt.PromptJSON,
			}
		}),
	}
}

func mapRuntimeStatuses(pools []domain.PoolInfo) []ServerRuntimeStatus {
	return mapping.MapSlice(pools, mapPoolInfo)
}

func mapPoolInfo(pool domain.PoolInfo) ServerRuntimeStatus {
	instances := make([]InstanceStatus, 0, len(pool.Instances))
	stats := PoolStats{}
	for _, inst := range pool.Instances {
		instances = append(instances, InstanceStatus{
			ID:         inst.ID,
			State:      string(inst.State),
			BusyCount:  inst.BusyCount,
			LastActive: inst.LastActive.Format("2006-01-02T15:04:05Z07:00"),
		})

		stats.Total++
		switch inst.State {
		case domain.InstanceStateReady:
			stats.Ready++
		case domain.InstanceStateBusy:
			stats.Busy++
		case domain.InstanceStateStarting:
			stats.Starting++
		case domain.InstanceStateDraining:
			stats.Draining++
		case domain.InstanceStateFailed:
			stats.Failed++
		}
	}

	return ServerRuntimeStatus{
		SpecKey:    pool.SpecKey,
		ServerName: pool.ServerName,
		Instances:  instances,
		Stats:      stats,
	}
}

func mapServerInitStatuses(statuses []domain.ServerInitStatus) []ServerInitStatus {
	return mapping.MapSlice(statuses, func(status domain.ServerInitStatus) ServerInitStatus {
		return ServerInitStatus{
			SpecKey:    status.SpecKey,
			ServerName: status.ServerName,
			MinReady:   status.MinReady,
			Ready:      status.Ready,
			Failed:     status.Failed,
			State:      string(status.State),
			LastError:  status.LastError,
			UpdatedAt:  status.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}
	})
}

func mapActiveCallers(callers []domain.ActiveCaller) []ActiveCaller {
	return mapping.MapSlice(callers, func(caller domain.ActiveCaller) ActiveCaller {
		return ActiveCaller{
			Caller:        caller.Caller,
			PID:           caller.PID,
			Profile:       caller.Profile,
			LastHeartbeat: caller.LastHeartbeat.Format("2006-01-02T15:04:05.000Z07:00"),
		}
	})
}

func mapServerSpecDetail(spec domain.ServerSpec, specKey string) ServerSpecDetail {
	env := spec.Env
	if env == nil {
		env = make(map[string]string)
	}
	exposeTools := spec.ExposeTools
	if exposeTools == nil {
		exposeTools = []string{}
	}

	return ServerSpecDetail{
		Name:                spec.Name,
		SpecKey:             specKey,
		Cmd:                 spec.Cmd,
		Env:                 env,
		Cwd:                 spec.Cwd,
		IdleSeconds:         spec.IdleSeconds,
		MaxConcurrent:       spec.MaxConcurrent,
		Sticky:              spec.Sticky,
		Persistent:          spec.Persistent,
		Disabled:            spec.Disabled,
		MinReady:            spec.MinReady,
		DrainTimeoutSeconds: spec.DrainTimeoutSeconds,
		ProtocolVersion:     spec.ProtocolVersion,
		ExposeTools:         exposeTools,
	}
}

func mapRuntimeConfigDetail(cfg domain.RuntimeConfig) RuntimeConfigDetail {
	return RuntimeConfigDetail{
		RouteTimeoutSeconds:    cfg.RouteTimeoutSeconds,
		PingIntervalSeconds:    cfg.PingIntervalSeconds,
		ToolRefreshSeconds:     cfg.ToolRefreshSeconds,
		ToolRefreshConcurrency: cfg.ToolRefreshConcurrency,
		CallerCheckSeconds:     cfg.CallerCheckSeconds,
		ExposeTools:            cfg.ExposeTools,
		ToolNamespaceStrategy:  cfg.ToolNamespaceStrategy,
		Observability: ObservabilityConfigDetail{
			ListenAddress: cfg.Observability.ListenAddress,
		},
		RPC: RPCConfigDetail{
			ListenAddress:           cfg.RPC.ListenAddress,
			MaxRecvMsgSize:          cfg.RPC.MaxRecvMsgSize,
			MaxSendMsgSize:          cfg.RPC.MaxSendMsgSize,
			KeepaliveTimeSeconds:    cfg.RPC.KeepaliveTimeSeconds,
			KeepaliveTimeoutSeconds: cfg.RPC.KeepaliveTimeoutSeconds,
			SocketMode:              cfg.RPC.SocketMode,
			TLS: RPCTLSConfigDetail{
				Enabled:    cfg.RPC.TLS.Enabled,
				CertFile:   cfg.RPC.TLS.CertFile,
				KeyFile:    cfg.RPC.TLS.KeyFile,
				CAFile:     cfg.RPC.TLS.CAFile,
				ClientAuth: cfg.RPC.TLS.ClientAuth,
			},
		},
	}
}
