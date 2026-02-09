package mapping

import (
	"fmt"
	"strings"
	"time"

	"mcpv/internal/domain"
	"mcpv/internal/infra/mapping"
	"mcpv/internal/infra/mcpcodec"
	"mcpv/internal/ui/types"
)

func MapToolCatalogEntries(snapshot domain.ToolCatalogSnapshot) ([]types.ToolEntry, error) {
	entries := make([]types.ToolEntry, 0, len(snapshot.Tools))
	for _, entry := range snapshot.Tools {
		tool := entry.Definition
		raw, err := mcpcodec.MarshalToolDefinition(tool)
		if err != nil {
			return nil, fmt.Errorf("marshal tool %q: %w", tool.Name, err)
		}
		cachedAt := ""
		if entry.Source == domain.ToolSourceCache && !entry.CachedAt.IsZero() {
			cachedAt = entry.CachedAt.UTC().Format(time.RFC3339Nano)
		}
		entries = append(entries, types.ToolEntry{
			Name:        tool.Name,
			Description: tool.Description,
			ToolJSON:    raw,
			SpecKey:     tool.SpecKey,
			ServerName:  tool.ServerName,
			Source:      string(entry.Source),
			CachedAt:    cachedAt,
		})
	}
	return entries, nil
}

func MapResourcePage(page domain.ResourcePage) (*types.ResourcePage, error) {
	resources := make([]types.ResourceEntry, 0, len(page.Snapshot.Resources))
	for _, resource := range page.Snapshot.Resources {
		raw, err := mcpcodec.MarshalResourceDefinition(resource)
		if err != nil {
			return nil, fmt.Errorf("marshal resource %q: %w", resource.URI, err)
		}
		resources = append(resources, types.ResourceEntry{
			URI:          resource.URI,
			ResourceJSON: raw,
		})
	}
	return &types.ResourcePage{
		NextCursor: page.NextCursor,
		Resources:  resources,
	}, nil
}

func MapPromptPage(page domain.PromptPage) (*types.PromptPage, error) {
	prompts := make([]types.PromptEntry, 0, len(page.Snapshot.Prompts))
	for _, prompt := range page.Snapshot.Prompts {
		raw, err := mcpcodec.MarshalPromptDefinition(prompt)
		if err != nil {
			return nil, fmt.Errorf("marshal prompt %q: %w", prompt.Name, err)
		}
		prompts = append(prompts, types.PromptEntry{
			Name:       prompt.Name,
			PromptJSON: raw,
		})
	}
	return &types.PromptPage{
		NextCursor: page.NextCursor,
		Prompts:    prompts,
	}, nil
}

func MapRuntimeStatuses(pools []domain.PoolInfo) []types.ServerRuntimeStatus {
	return mapping.MapSlice(pools, mapPoolInfo)
}

func MapStartCause(cause *domain.StartCause) *types.StartCause {
	if cause == nil {
		return nil
	}
	timestamp := ""
	if !cause.Timestamp.IsZero() {
		timestamp = cause.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	mapped := &types.StartCause{
		Reason:    string(cause.Reason),
		Client:    cause.Client,
		ToolName:  cause.ToolName,
		Timestamp: timestamp,
	}
	if cause.Policy != nil {
		mapped.Policy = &types.StartCausePolicy{
			ActivationMode: string(cause.Policy.ActivationMode),
			MinReady:       cause.Policy.MinReady,
		}
	}
	return mapped
}

func mapPoolInfo(pool domain.PoolInfo) types.ServerRuntimeStatus {
	instances := make([]types.InstanceStatus, 0, len(pool.Instances))
	stats := types.PoolStats{}
	metrics := types.PoolMetrics{
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
		instances = append(instances, types.InstanceStatus{
			ID:              inst.ID,
			State:           string(inst.State),
			BusyCount:       inst.BusyCount,
			LastActive:      inst.LastActive.Format("2006-01-02T15:04:05Z07:00"),
			SpawnedAt:       inst.SpawnedAt.Format("2006-01-02T15:04:05Z07:00"),
			HandshakedAt:    inst.HandshakedAt.Format("2006-01-02T15:04:05Z07:00"),
			LastHeartbeatAt: inst.LastHeartbeatAt.Format("2006-01-02T15:04:05Z07:00"),
			LastStartCause:  MapStartCause(inst.LastStartCause),
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
		case domain.InstanceStateStopped:
		case domain.InstanceStateFailed:
			stats.Failed++
		}
	}

	return types.ServerRuntimeStatus{
		SpecKey:    pool.SpecKey,
		ServerName: pool.ServerName,
		Instances:  instances,
		Stats:      stats,
		Metrics:    metrics,
	}
}

func MapServerInitStatuses(statuses []domain.ServerInitStatus) []types.ServerInitStatus {
	return mapping.MapSlice(statuses, func(status domain.ServerInitStatus) types.ServerInitStatus {
		nextRetryAt := ""
		if !status.NextRetryAt.IsZero() {
			nextRetryAt = status.NextRetryAt.UTC().Format(time.RFC3339Nano)
		}
		return types.ServerInitStatus{
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

func MapActiveClients(clients []domain.ActiveClient) []types.ActiveClient {
	return mapping.MapSlice(clients, func(client domain.ActiveClient) types.ActiveClient {
		return types.ActiveClient{
			Client:        client.Client,
			PID:           client.PID,
			Tags:          append([]string(nil), client.Tags...),
			Server:        client.Server,
			LastHeartbeat: client.LastHeartbeat.Format("2006-01-02T15:04:05.000Z07:00"),
		}
	})
}

func MapServerSummary(spec domain.ServerSpec, specKey string) types.ServerSummary {
	return types.ServerSummary{
		Name:      spec.Name,
		SpecKey:   specKey,
		Transport: string(domain.NormalizeTransport(spec.Transport)),
		Tags:      append([]string(nil), spec.Tags...),
		Disabled:  spec.Disabled,
	}
}

func MapServerSpecDetail(spec domain.ServerSpec, specKey string) types.ServerSpecDetail {
	env := spec.Env
	if env == nil {
		env = make(map[string]string)
	}
	exposeTools := spec.ExposeTools
	if exposeTools == nil {
		exposeTools = []string{}
	}
	var httpCfg *types.StreamableHTTPConfigDetail
	if spec.HTTP != nil {
		headers := spec.HTTP.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		httpCfg = &types.StreamableHTTPConfigDetail{
			Endpoint:   spec.HTTP.Endpoint,
			Headers:    headers,
			MaxRetries: spec.HTTP.MaxRetries,
		}
	}

	return types.ServerSpecDetail{
		Name:                spec.Name,
		SpecKey:             specKey,
		Transport:           string(domain.NormalizeTransport(spec.Transport)),
		Cmd:                 spec.Cmd,
		Env:                 env,
		Cwd:                 spec.Cwd,
		Tags:                append([]string(nil), spec.Tags...),
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

func MapServerSpecDetailToDomain(detail types.ServerSpecDetail) domain.ServerSpec {
	env := detail.Env
	if env == nil {
		env = make(map[string]string)
	}
	exposeTools := detail.ExposeTools
	if exposeTools == nil {
		exposeTools = []string{}
	}
	var httpCfg *domain.StreamableHTTPConfig
	if detail.HTTP != nil {
		headers := detail.HTTP.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		httpCfg = &domain.StreamableHTTPConfig{
			Endpoint:   detail.HTTP.Endpoint,
			Headers:    headers,
			MaxRetries: detail.HTTP.MaxRetries,
		}
	}

	return domain.ServerSpec{
		Name:                strings.TrimSpace(detail.Name),
		Transport:           domain.TransportKind(strings.TrimSpace(detail.Transport)),
		Cmd:                 append([]string(nil), detail.Cmd...),
		Env:                 env,
		Cwd:                 strings.TrimSpace(detail.Cwd),
		Tags:                append([]string(nil), detail.Tags...),
		IdleSeconds:         detail.IdleSeconds,
		MaxConcurrent:       detail.MaxConcurrent,
		Strategy:            domain.InstanceStrategy(strings.TrimSpace(detail.Strategy)),
		SessionTTLSeconds:   detail.SessionTTLSeconds,
		Disabled:            detail.Disabled,
		MinReady:            detail.MinReady,
		ActivationMode:      domain.ActivationMode(strings.TrimSpace(detail.ActivationMode)),
		DrainTimeoutSeconds: detail.DrainTimeoutSeconds,
		ProtocolVersion:     strings.TrimSpace(detail.ProtocolVersion),
		ExposeTools:         exposeTools,
		HTTP:                httpCfg,
	}
}

func MapRuntimeConfigDetail(cfg domain.RuntimeConfig) types.RuntimeConfigDetail {
	return types.RuntimeConfigDetail{
		RouteTimeoutSeconds:        cfg.RouteTimeoutSeconds,
		PingIntervalSeconds:        cfg.PingIntervalSeconds,
		ToolRefreshSeconds:         cfg.ToolRefreshSeconds,
		ToolRefreshConcurrency:     cfg.ToolRefreshConcurrency,
		ClientCheckSeconds:         cfg.ClientCheckSeconds,
		ClientInactiveSeconds:      cfg.ClientInactiveSeconds,
		ServerInitRetryBaseSeconds: cfg.ServerInitRetryBaseSeconds,
		ServerInitRetryMaxSeconds:  cfg.ServerInitRetryMaxSeconds,
		ServerInitMaxRetries:       cfg.ServerInitMaxRetries,
		ReloadMode:                 string(cfg.ReloadMode),
		BootstrapMode:              string(cfg.BootstrapMode),
		BootstrapConcurrency:       cfg.BootstrapConcurrency,
		BootstrapTimeoutSeconds:    cfg.BootstrapTimeoutSeconds,
		DefaultActivationMode:      string(cfg.DefaultActivationMode),
		ExposeTools:                cfg.ExposeTools,
		ToolNamespaceStrategy:      string(cfg.ToolNamespaceStrategy),
		Observability: types.ObservabilityConfigDetail{
			ListenAddress: cfg.Observability.ListenAddress,
		},
		RPC: types.RPCConfigDetail{
			ListenAddress:           cfg.RPC.ListenAddress,
			MaxRecvMsgSize:          cfg.RPC.MaxRecvMsgSize,
			MaxSendMsgSize:          cfg.RPC.MaxSendMsgSize,
			KeepaliveTimeSeconds:    cfg.RPC.KeepaliveTimeSeconds,
			KeepaliveTimeoutSeconds: cfg.RPC.KeepaliveTimeoutSeconds,
			SocketMode:              cfg.RPC.SocketMode,
			TLS: types.RPCTLSConfigDetail{
				Enabled:    cfg.RPC.TLS.Enabled,
				CertFile:   cfg.RPC.TLS.CertFile,
				KeyFile:    cfg.RPC.TLS.KeyFile,
				CAFile:     cfg.RPC.TLS.CAFile,
				ClientAuth: cfg.RPC.TLS.ClientAuth,
			},
		},
	}
}
