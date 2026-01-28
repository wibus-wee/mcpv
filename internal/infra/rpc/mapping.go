package rpc

import (
	"encoding/json"
	"fmt"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mapping"
	"mcpd/internal/infra/mcpcodec"
	controlv1 "mcpd/pkg/api/control/v1"
)

func toProtoSnapshot(snapshot domain.ToolSnapshot) (*controlv1.ToolsSnapshot, error) {
	tools := make([]*controlv1.ToolDefinition, 0, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		raw, err := mcpcodec.MarshalToolDefinition(tool)
		if err != nil {
			return nil, fmt.Errorf("marshal tool %q: %w", tool.Name, err)
		}
		tools = append(tools, &controlv1.ToolDefinition{
			Name:     tool.Name,
			ToolJson: raw,
		})
	}
	return &controlv1.ToolsSnapshot{
		Etag:  snapshot.ETag,
		Tools: tools,
	}, nil
}

func toProtoResourcesSnapshot(snapshot domain.ResourceSnapshot) (*controlv1.ResourcesSnapshot, error) {
	resources := make([]*controlv1.ResourceDefinition, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		raw, err := mcpcodec.MarshalResourceDefinition(resource)
		if err != nil {
			return nil, fmt.Errorf("marshal resource %q: %w", resource.URI, err)
		}
		resources = append(resources, &controlv1.ResourceDefinition{
			Uri:          resource.URI,
			ResourceJson: raw,
		})
	}
	return &controlv1.ResourcesSnapshot{
		Etag:      snapshot.ETag,
		Resources: resources,
	}, nil
}

func toProtoPromptsSnapshot(snapshot domain.PromptSnapshot) (*controlv1.PromptsSnapshot, error) {
	prompts := make([]*controlv1.PromptDefinition, 0, len(snapshot.Prompts))
	for _, prompt := range snapshot.Prompts {
		raw, err := mcpcodec.MarshalPromptDefinition(prompt)
		if err != nil {
			return nil, fmt.Errorf("marshal prompt %q: %w", prompt.Name, err)
		}
		prompts = append(prompts, &controlv1.PromptDefinition{
			Name:       prompt.Name,
			PromptJson: raw,
		})
	}
	return &controlv1.PromptsSnapshot{
		Etag:    snapshot.ETag,
		Prompts: prompts,
	}, nil
}

func toProtoLogEntry(entry domain.LogEntry) *controlv1.LogEntry {
	return &controlv1.LogEntry{
		Logger:            entry.Logger,
		Level:             toProtoLogLevel(entry.Level),
		TimestampUnixNano: entry.Timestamp.UnixNano(),
		DataJson:          mustMarshalLogData(entry.Data),
	}
}

func mustMarshalLogData(data map[string]any) []byte {
	if len(data) == 0 {
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return raw
}

func fromProtoLogLevel(level controlv1.LogLevel) domain.LogLevel {
	switch level {
	case controlv1.LogLevel_LOG_LEVEL_UNSPECIFIED:
		fallthrough
	case controlv1.LogLevel_LOG_LEVEL_INFO:
		return domain.LogLevelInfo
	case controlv1.LogLevel_LOG_LEVEL_NOTICE:
		return domain.LogLevelNotice
	case controlv1.LogLevel_LOG_LEVEL_WARNING:
		return domain.LogLevelWarning
	case controlv1.LogLevel_LOG_LEVEL_ERROR:
		return domain.LogLevelError
	case controlv1.LogLevel_LOG_LEVEL_CRITICAL:
		return domain.LogLevelCritical
	case controlv1.LogLevel_LOG_LEVEL_ALERT:
		return domain.LogLevelAlert
	case controlv1.LogLevel_LOG_LEVEL_EMERGENCY:
		return domain.LogLevelEmergency
	case controlv1.LogLevel_LOG_LEVEL_DEBUG:
		fallthrough
	default:
		return domain.LogLevelDebug
	}
}

func toProtoLogLevel(level domain.LogLevel) controlv1.LogLevel {
	switch level {
	case domain.LogLevelInfo:
		return controlv1.LogLevel_LOG_LEVEL_INFO
	case domain.LogLevelNotice:
		return controlv1.LogLevel_LOG_LEVEL_NOTICE
	case domain.LogLevelWarning:
		return controlv1.LogLevel_LOG_LEVEL_WARNING
	case domain.LogLevelError:
		return controlv1.LogLevel_LOG_LEVEL_ERROR
	case domain.LogLevelCritical:
		return controlv1.LogLevel_LOG_LEVEL_CRITICAL
	case domain.LogLevelAlert:
		return controlv1.LogLevel_LOG_LEVEL_ALERT
	case domain.LogLevelEmergency:
		return controlv1.LogLevel_LOG_LEVEL_EMERGENCY
	case domain.LogLevelDebug:
		fallthrough
	default:
		return controlv1.LogLevel_LOG_LEVEL_DEBUG
	}
}

func toProtoRuntimeStatusSnapshot(snapshot domain.RuntimeStatusSnapshot) *controlv1.RuntimeStatusSnapshot {
	statuses := mapping.MapSlice(snapshot.Statuses, toProtoServerRuntimeStatus)
	return &controlv1.RuntimeStatusSnapshot{
		Etag:                snapshot.ETag,
		Statuses:            statuses,
		GeneratedAtUnixNano: snapshot.GeneratedAt.UnixNano(),
	}
}

func toProtoServerRuntimeStatus(s domain.ServerRuntimeStatus) *controlv1.ServerRuntimeStatus {
	instances := mapping.MapSlice(s.Instances, func(inst domain.InstanceStatusInfo) *controlv1.InstanceStatus {
		return &controlv1.InstanceStatus{
			Id:                      inst.ID,
			State:                   string(inst.State),
			BusyCount:               int32(inst.BusyCount),
			LastActiveUnixNano:      inst.LastActive.UnixNano(),
			SpawnedAtUnixNano:       inst.SpawnedAt.UnixNano(),
			HandshakedAtUnixNano:    inst.HandshakedAt.UnixNano(),
			LastHeartbeatAtUnixNano: inst.LastHeartbeatAt.UnixNano(),
		}
	})
	return &controlv1.ServerRuntimeStatus{
		SpecKey:    s.SpecKey,
		ServerName: s.ServerName,
		Instances:  instances,
		Stats: &controlv1.PoolStats{
			Total:        int32(s.Stats.Total),
			Ready:        int32(s.Stats.Ready),
			Busy:         int32(s.Stats.Busy),
			Starting:     int32(s.Stats.Starting),
			Draining:     int32(s.Stats.Draining),
			Failed:       int32(s.Stats.Failed),
			Initializing: int32(s.Stats.Initializing),
			Handshaking:  int32(s.Stats.Handshaking),
		},
		Metrics: func() *controlv1.PoolMetrics {
			lastCallAtUnixNano := int64(0)
			if !s.Metrics.LastCallAt.IsZero() {
				lastCallAtUnixNano = s.Metrics.LastCallAt.UnixNano()
			}
			return &controlv1.PoolMetrics{
				StartCount:         int32(s.Metrics.StartCount),
				StopCount:          int32(s.Metrics.StopCount),
				TotalCalls:         s.Metrics.TotalCalls,
				TotalErrors:        s.Metrics.TotalErrors,
				TotalDurationMs:    s.Metrics.TotalDuration.Milliseconds(),
				LastCallAtUnixNano: lastCallAtUnixNano,
			}
		}(),
	}
}

func toProtoServerInitStatusSnapshot(snapshot domain.ServerInitStatusSnapshot) *controlv1.ServerInitStatusSnapshot {
	statuses := mapping.MapSlice(snapshot.Statuses, func(s domain.ServerInitStatus) *controlv1.ServerInitStatus {
		return &controlv1.ServerInitStatus{
			SpecKey:           s.SpecKey,
			ServerName:        s.ServerName,
			MinReady:          int32(s.MinReady),
			Ready:             int32(s.Ready),
			Failed:            int32(s.Failed),
			State:             string(s.State),
			LastError:         s.LastError,
			UpdatedAtUnixNano: s.UpdatedAt.UnixNano(),
		}
	})
	return &controlv1.ServerInitStatusSnapshot{
		Statuses:            statuses,
		GeneratedAtUnixNano: snapshot.GeneratedAt.UnixNano(),
	}
}
