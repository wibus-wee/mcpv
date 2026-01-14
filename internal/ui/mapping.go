package ui

import (
	"time"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mapping"
	"mcpd/internal/infra/mcpcodec"
)

func mapToolEntries(snapshot domain.ToolSnapshot) []ToolEntry {
	return mapping.MapSlice(snapshot.Tools, func(tool domain.ToolDefinition) ToolEntry {
		return ToolEntry{
			Name:       tool.Name,
			ToolJSON:   mcpcodec.MustMarshalToolDefinition(tool),
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
				ResourceJSON: mcpcodec.MustMarshalResourceDefinition(resource),
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
				PromptJSON: mcpcodec.MustMarshalPromptDefinition(prompt),
			}
		}),
	}
}

func mapRuntimeStatuses(pools []domain.PoolInfo) []ServerRuntimeStatus {
	return mapping.MapSlice(pools, mapPoolInfo)
}

func mapStartCause(cause *domain.StartCause) *StartCause {
	if cause == nil {
		return nil
	}
	timestamp := ""
	if !cause.Timestamp.IsZero() {
		timestamp = cause.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	mapped := &StartCause{
		Reason:    string(cause.Reason),
		Caller:    cause.Caller,
		ToolName:  cause.ToolName,
		Profile:   cause.Profile,
		Timestamp: timestamp,
	}
	if cause.Policy != nil {
		mapped.Policy = &StartCausePolicy{
			ActivationMode: string(cause.Policy.ActivationMode),
			MinReady:       cause.Policy.MinReady,
		}
	}
	return mapped
}

func mapPoolInfo(pool domain.PoolInfo) ServerRuntimeStatus {
	instances := make([]InstanceStatus, 0, len(pool.Instances))
	stats := PoolStats{}
	metrics := PoolMetrics{
		StartCount:      pool.Metrics.StartCount,
		StopCount:       pool.Metrics.StopCount,
		TotalCalls:      pool.Metrics.TotalCalls,
		TotalErrors:     pool.Metrics.TotalErrors,
		TotalDurationMs: pool.Metrics.TotalDuration.Milliseconds(),
	}
	if !pool.Metrics.LastCallAt.IsZero() {
		metrics.LastCallAt = pool.Metrics.LastCallAt.UTC().Format(time.RFC3339Nano)
	}
	for _, inst := range pool.Instances {
		instances = append(instances, InstanceStatus{
			ID:              inst.ID,
			State:           string(inst.State),
			BusyCount:       inst.BusyCount,
			LastActive:      inst.LastActive.Format("2006-01-02T15:04:05Z07:00"),
			SpawnedAt:       inst.SpawnedAt.Format("2006-01-02T15:04:05Z07:00"),
			HandshakedAt:    inst.HandshakedAt.Format("2006-01-02T15:04:05Z07:00"),
			LastHeartbeatAt: inst.LastHeartbeatAt.Format("2006-01-02T15:04:05Z07:00"),
			LastStartCause:  mapStartCause(inst.LastStartCause),
		})

		stats.Total++
		switch inst.State {
		case domain.InstanceStateReady:
			stats.Ready++
		case domain.InstanceStateBusy:
			stats.Busy++
		case domain.InstanceStateStarting:
			stats.Starting++
		case domain.InstanceStateInitializing:
			stats.Initializing++
		case domain.InstanceStateHandshaking:
			stats.Handshaking++
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
		Metrics:    metrics,
	}
}

func mapServerInitStatuses(statuses []domain.ServerInitStatus) []ServerInitStatus {
	return mapping.MapSlice(statuses, func(status domain.ServerInitStatus) ServerInitStatus {
		nextRetryAt := ""
		if !status.NextRetryAt.IsZero() {
			nextRetryAt = status.NextRetryAt.UTC().Format(time.RFC3339Nano)
		}
		return ServerInitStatus{
			SpecKey:     status.SpecKey,
			ServerName:  status.ServerName,
			MinReady:    status.MinReady,
			Ready:       status.Ready,
			Failed:      status.Failed,
			State:       string(status.State),
			LastError:   status.LastError,
			RetryCount:  status.RetryCount,
			NextRetryAt: nextRetryAt,
			UpdatedAt:   status.UpdatedAt.UTC().Format(time.RFC3339Nano),
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
	var httpCfg *StreamableHTTPConfigDetail
	if spec.HTTP != nil {
		headers := spec.HTTP.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		httpCfg = &StreamableHTTPConfigDetail{
			Endpoint:   spec.HTTP.Endpoint,
			Headers:    headers,
			MaxRetries: spec.HTTP.MaxRetries,
		}
	}

	return ServerSpecDetail{
		Name:                spec.Name,
		SpecKey:             specKey,
		Transport:           string(domain.NormalizeTransport(spec.Transport)),
		Cmd:                 spec.Cmd,
		Env:                 env,
		Cwd:                 spec.Cwd,
		IdleSeconds:         spec.IdleSeconds,
		MaxConcurrent:       spec.MaxConcurrent,
		Strategy:            string(spec.Strategy),
		SessionTTLSeconds:   spec.SessionTTLSeconds,
		Disabled:            spec.Disabled,
		MinReady:            spec.MinReady,
		ActivationMode:      string(spec.ActivationMode),
		DrainTimeoutSeconds: spec.DrainTimeoutSeconds,
		ProtocolVersion:     spec.ProtocolVersion,
		ExposeTools:         exposeTools,
		HTTP:                httpCfg,
	}
}

func mapRuntimeConfigDetail(cfg domain.RuntimeConfig) RuntimeConfigDetail {
	return RuntimeConfigDetail{
		RouteTimeoutSeconds:        cfg.RouteTimeoutSeconds,
		PingIntervalSeconds:        cfg.PingIntervalSeconds,
		ToolRefreshSeconds:         cfg.ToolRefreshSeconds,
		ToolRefreshConcurrency:     cfg.ToolRefreshConcurrency,
		CallerCheckSeconds:         cfg.CallerCheckSeconds,
		CallerInactiveSeconds:      cfg.CallerInactiveSeconds,
		ServerInitRetryBaseSeconds: cfg.ServerInitRetryBaseSeconds,
		ServerInitRetryMaxSeconds:  cfg.ServerInitRetryMaxSeconds,
		ServerInitMaxRetries:       cfg.ServerInitMaxRetries,
		BootstrapMode:              string(cfg.BootstrapMode),
		BootstrapConcurrency:       cfg.BootstrapConcurrency,
		BootstrapTimeoutSeconds:    cfg.BootstrapTimeoutSeconds,
		DefaultActivationMode:      string(cfg.DefaultActivationMode),
		ExposeTools:                cfg.ExposeTools,
		ToolNamespaceStrategy:      cfg.ToolNamespaceStrategy,
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
