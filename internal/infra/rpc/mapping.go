package rpc

import (
	"mcpd/internal/domain"
	"mcpd/internal/infra/mapping"
	controlv1 "mcpd/pkg/api/control/v1"
)

func toProtoSnapshot(snapshot domain.ToolSnapshot) *controlv1.ToolsSnapshot {
	tools := mapping.MapSlice(snapshot.Tools, func(tool domain.ToolDefinition) *controlv1.ToolDefinition {
		return &controlv1.ToolDefinition{
			Name:     tool.Name,
			ToolJson: tool.ToolJSON,
		}
	})
	return &controlv1.ToolsSnapshot{
		Etag:  snapshot.ETag,
		Tools: tools,
	}
}

func toProtoResourcesSnapshot(snapshot domain.ResourceSnapshot) *controlv1.ResourcesSnapshot {
	resources := mapping.MapSlice(snapshot.Resources, func(resource domain.ResourceDefinition) *controlv1.ResourceDefinition {
		return &controlv1.ResourceDefinition{
			Uri:          resource.URI,
			ResourceJson: resource.ResourceJSON,
		}
	})
	return &controlv1.ResourcesSnapshot{
		Etag:      snapshot.ETag,
		Resources: resources,
	}
}

func toProtoPromptsSnapshot(snapshot domain.PromptSnapshot) *controlv1.PromptsSnapshot {
	prompts := mapping.MapSlice(snapshot.Prompts, func(prompt domain.PromptDefinition) *controlv1.PromptDefinition {
		return &controlv1.PromptDefinition{
			Name:       prompt.Name,
			PromptJson: prompt.PromptJSON,
		}
	})
	return &controlv1.PromptsSnapshot{
		Etag:    snapshot.ETag,
		Prompts: prompts,
	}
}

func toProtoLogEntry(entry domain.LogEntry) *controlv1.LogEntry {
	return &controlv1.LogEntry{
		Logger:            entry.Logger,
		Level:             toProtoLogLevel(entry.Level),
		TimestampUnixNano: entry.Timestamp.UnixNano(),
		DataJson:          entry.DataJSON,
	}
}

func fromProtoLogLevel(level controlv1.LogLevel) domain.LogLevel {
	switch level {
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
			Id:                 inst.ID,
			State:              string(inst.State),
			BusyCount:          int32(inst.BusyCount),
			LastActiveUnixNano: inst.LastActive.UnixNano(),
		}
	})
	return &controlv1.ServerRuntimeStatus{
		SpecKey:    s.SpecKey,
		ServerName: s.ServerName,
		Instances:  instances,
		Stats: &controlv1.PoolStats{
			Total:    int32(s.Stats.Total),
			Ready:    int32(s.Stats.Ready),
			Busy:     int32(s.Stats.Busy),
			Starting: int32(s.Stats.Starting),
			Draining: int32(s.Stats.Draining),
			Failed:   int32(s.Stats.Failed),
		},
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
