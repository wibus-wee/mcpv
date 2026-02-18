package rpc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcpv/internal/domain"
	"mcpv/internal/infra/mcpcodec"
	controlv1 "mcpv/pkg/api/control/v1"
)

func fromProtoToolSnapshot(snapshot *controlv1.ToolsSnapshot) (domain.ToolSnapshot, error) {
	if snapshot == nil {
		return domain.ToolSnapshot{}, nil
	}
	tools := make([]domain.ToolDefinition, 0, len(snapshot.GetTools()))
	for _, tool := range snapshot.GetTools() {
		def, err := fromProtoToolDefinition(tool)
		if err != nil {
			return domain.ToolSnapshot{}, err
		}
		tools = append(tools, def)
	}
	return domain.ToolSnapshot{
		ETag:  snapshot.GetEtag(),
		Tools: tools,
	}, nil
}

func fromProtoToolDefinition(def *controlv1.ToolDefinition) (domain.ToolDefinition, error) {
	if def == nil {
		return domain.ToolDefinition{}, nil
	}
	if len(def.GetToolJson()) == 0 {
		return domain.ToolDefinition{Name: def.GetName()}, nil
	}
	var tool mcp.Tool
	if err := json.Unmarshal(def.GetToolJson(), &tool); err != nil {
		return domain.ToolDefinition{}, fmt.Errorf("decode tool %q: %w", def.GetName(), err)
	}
	domainTool := mcpcodec.ToolFromMCP(&tool)
	if domainTool.Name == "" {
		domainTool.Name = def.GetName()
	}
	return domainTool, nil
}

func fromProtoResourceSnapshot(snapshot *controlv1.ResourcesSnapshot) (domain.ResourceSnapshot, error) {
	if snapshot == nil {
		return domain.ResourceSnapshot{}, nil
	}
	resources := make([]domain.ResourceDefinition, 0, len(snapshot.GetResources()))
	for _, resource := range snapshot.GetResources() {
		def, err := fromProtoResourceDefinition(resource)
		if err != nil {
			return domain.ResourceSnapshot{}, err
		}
		resources = append(resources, def)
	}
	return domain.ResourceSnapshot{
		ETag:      snapshot.GetEtag(),
		Resources: resources,
	}, nil
}

func fromProtoResourceDefinition(def *controlv1.ResourceDefinition) (domain.ResourceDefinition, error) {
	if def == nil {
		return domain.ResourceDefinition{}, nil
	}
	if len(def.GetResourceJson()) == 0 {
		return domain.ResourceDefinition{URI: def.GetUri()}, nil
	}
	var resource mcp.Resource
	if err := json.Unmarshal(def.GetResourceJson(), &resource); err != nil {
		return domain.ResourceDefinition{}, fmt.Errorf("decode resource %q: %w", def.GetUri(), err)
	}
	domainResource := mcpcodec.ResourceFromMCP(&resource)
	if domainResource.URI == "" {
		domainResource.URI = def.GetUri()
	}
	return domainResource, nil
}

func fromProtoPromptSnapshot(snapshot *controlv1.PromptsSnapshot) (domain.PromptSnapshot, error) {
	if snapshot == nil {
		return domain.PromptSnapshot{}, nil
	}
	prompts := make([]domain.PromptDefinition, 0, len(snapshot.GetPrompts()))
	for _, prompt := range snapshot.GetPrompts() {
		def, err := fromProtoPromptDefinition(prompt)
		if err != nil {
			return domain.PromptSnapshot{}, err
		}
		prompts = append(prompts, def)
	}
	return domain.PromptSnapshot{
		ETag:    snapshot.GetEtag(),
		Prompts: prompts,
	}, nil
}

func fromProtoPromptDefinition(def *controlv1.PromptDefinition) (domain.PromptDefinition, error) {
	if def == nil {
		return domain.PromptDefinition{}, nil
	}
	if len(def.GetPromptJson()) == 0 {
		return domain.PromptDefinition{Name: def.GetName()}, nil
	}
	var prompt mcp.Prompt
	if err := json.Unmarshal(def.GetPromptJson(), &prompt); err != nil {
		return domain.PromptDefinition{}, fmt.Errorf("decode prompt %q: %w", def.GetName(), err)
	}
	domainPrompt := mcpcodec.PromptFromMCP(&prompt)
	if domainPrompt.Name == "" {
		domainPrompt.Name = def.GetName()
	}
	return domainPrompt, nil
}

func fromProtoRuntimeStatusSnapshot(snapshot *controlv1.RuntimeStatusSnapshot) (domain.RuntimeStatusSnapshot, error) {
	if snapshot == nil {
		return domain.RuntimeStatusSnapshot{}, nil
	}
	statuses := make([]domain.ServerRuntimeStatus, 0, len(snapshot.GetStatuses()))
	for _, status := range snapshot.GetStatuses() {
		statuses = append(statuses, fromProtoServerRuntimeStatus(status))
	}
	updated := time.Time{}
	if snapshot.GetGeneratedAtUnixNano() > 0 {
		updated = time.Unix(0, snapshot.GetGeneratedAtUnixNano())
	}
	return domain.RuntimeStatusSnapshot{
		ETag:        snapshot.GetEtag(),
		Statuses:    statuses,
		GeneratedAt: updated,
	}, nil
}

func fromProtoServerRuntimeStatus(status *controlv1.ServerRuntimeStatus) domain.ServerRuntimeStatus {
	if status == nil {
		return domain.ServerRuntimeStatus{}
	}
	instances := make([]domain.InstanceStatusInfo, 0, len(status.GetInstances()))
	for _, inst := range status.GetInstances() {
		if inst == nil {
			continue
		}
		instances = append(instances, domain.InstanceStatusInfo{
			ID:              inst.GetId(),
			State:           domain.InstanceState(inst.GetState()),
			BusyCount:       int(inst.GetBusyCount()),
			LastActive:      unixNanoToTime(inst.GetLastActiveUnixNano()),
			SpawnedAt:       unixNanoToTime(inst.GetSpawnedAtUnixNano()),
			HandshakedAt:    unixNanoToTime(inst.GetHandshakedAtUnixNano()),
			LastHeartbeatAt: unixNanoToTime(inst.GetLastHeartbeatAtUnixNano()),
		})
	}

	stats := domain.PoolStats{}
	if status.GetStats() != nil {
		stats = domain.PoolStats{
			Total:        int(status.GetStats().GetTotal()),
			Ready:        int(status.GetStats().GetReady()),
			Busy:         int(status.GetStats().GetBusy()),
			Starting:     int(status.GetStats().GetStarting()),
			Initializing: int(status.GetStats().GetInitializing()),
			Handshaking:  int(status.GetStats().GetHandshaking()),
			Draining:     int(status.GetStats().GetDraining()),
			Failed:       int(status.GetStats().GetFailed()),
		}
	}

	metrics := domain.PoolMetrics{}
	if status.GetMetrics() != nil {
		metrics = domain.PoolMetrics{
			StartCount:    int(status.GetMetrics().GetStartCount()),
			StopCount:     int(status.GetMetrics().GetStopCount()),
			TotalCalls:    status.GetMetrics().GetTotalCalls(),
			TotalErrors:   status.GetMetrics().GetTotalErrors(),
			TotalDuration: time.Duration(status.GetMetrics().GetTotalDurationMs()) * time.Millisecond,
			LastCallAt:    unixNanoToTime(status.GetMetrics().GetLastCallAtUnixNano()),
		}
	}

	return domain.ServerRuntimeStatus{
		SpecKey:     status.GetSpecKey(),
		ServerName:  status.GetServerName(),
		Instances:   instances,
		Stats:       stats,
		Metrics:     metrics,
		Diagnostics: domain.PoolDiagnostics{},
	}
}

func fromProtoServerInitStatusSnapshot(snapshot *controlv1.ServerInitStatusSnapshot) (domain.ServerInitStatusSnapshot, error) {
	if snapshot == nil {
		return domain.ServerInitStatusSnapshot{}, nil
	}
	statuses := make([]domain.ServerInitStatus, 0, len(snapshot.GetStatuses()))
	for _, status := range snapshot.GetStatuses() {
		if status == nil {
			continue
		}
		statuses = append(statuses, domain.ServerInitStatus{
			SpecKey:    status.GetSpecKey(),
			ServerName: status.GetServerName(),
			MinReady:   int(status.GetMinReady()),
			Ready:      int(status.GetReady()),
			Failed:     int(status.GetFailed()),
			State:      domain.ServerInitState(status.GetState()),
			LastError:  status.GetLastError(),
			UpdatedAt:  unixNanoToTime(status.GetUpdatedAtUnixNano()),
		})
	}
	updated := time.Time{}
	if snapshot.GetGeneratedAtUnixNano() > 0 {
		updated = time.Unix(0, snapshot.GetGeneratedAtUnixNano())
	}
	return domain.ServerInitStatusSnapshot{
		Statuses:    statuses,
		GeneratedAt: updated,
	}, nil
}

func fromProtoLogEntry(entry *controlv1.LogEntry) (domain.LogEntry, error) {
	if entry == nil {
		return domain.LogEntry{}, nil
	}
	var data map[string]any
	if len(entry.GetDataJson()) > 0 {
		if err := json.Unmarshal(entry.GetDataJson(), &data); err != nil {
			return domain.LogEntry{}, fmt.Errorf("decode log entry: %w", err)
		}
	}
	return domain.LogEntry{
		Logger:    entry.GetLogger(),
		Level:     fromProtoLogLevel(entry.GetLevel()),
		Timestamp: unixNanoToTime(entry.GetTimestampUnixNano()),
		Data:      data,
	}, nil
}

func fromProtoActiveClientsSnapshot(snapshot *controlv1.ActiveClientsSnapshot) (domain.ActiveClientSnapshot, error) {
	if snapshot == nil {
		return domain.ActiveClientSnapshot{}, nil
	}
	clients := make([]domain.ActiveClient, 0, len(snapshot.GetClients()))
	for _, client := range snapshot.GetClients() {
		if client == nil {
			continue
		}
		clients = append(clients, domain.ActiveClient{
			Client:        client.GetClient(),
			PID:           int(client.GetPid()),
			Tags:          append([]string(nil), client.GetTags()...),
			Server:        client.GetServer(),
			LastHeartbeat: unixNanoToTime(client.GetLastHeartbeatUnixNano()),
		})
	}
	return domain.ActiveClientSnapshot{
		Clients:     clients,
		GeneratedAt: unixNanoToTime(snapshot.GetGeneratedAtUnixNano()),
	}, nil
}

func fromProtoAutomaticMCPResponse(resp *controlv1.AutomaticMCPResponse) (domain.AutomaticMCPResult, error) {
	if resp == nil {
		return domain.AutomaticMCPResult{}, nil
	}
	tools := make([]domain.ToolDefinition, 0, len(resp.GetToolsJson()))
	for i, raw := range resp.GetToolsJson() {
		if len(raw) == 0 {
			continue
		}
		var tool mcp.Tool
		if err := json.Unmarshal(raw, &tool); err != nil {
			return domain.AutomaticMCPResult{}, fmt.Errorf("decode automatic_mcp tool %d: %w", i, err)
		}
		tools = append(tools, mcpcodec.ToolFromMCP(&tool))
	}
	return domain.AutomaticMCPResult{
		ETag:           resp.GetEtag(),
		Tools:          tools,
		TotalAvailable: int(resp.GetTotalAvailable()),
		Filtered:       int(resp.GetFiltered()),
	}, nil
}

func unixNanoToTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(0, value)
}
