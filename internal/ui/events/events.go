package events

import (
	"encoding/json"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"mcpv/internal/domain"
	"mcpv/internal/ui/mapping"
	"mcpv/internal/ui/types"
)

// Event name constants for Wails event emission.
const (
	// Core lifecycle events.
	EventCoreState = "core:state"

	// Data update events.
	EventToolsUpdated     = "tools:updated"
	EventResourcesUpdated = "resources:updated"
	EventPromptsUpdated   = "prompts:updated"

	// Status update events.
	EventRuntimeStatusUpdated = "runtime:status"
	EventServerInitUpdated    = "server-init:status"
	EventActiveClientsUpdated = "clients:active"

	// Log streaming events.
	EventLogEntry = "logs:entry"

	// Deep link events.
	EventDeepLink = "deep-link"

	// Error events.
	EventError = "error"

	// Update check events.
	EventUpdateAvailable = "update:available"
)

// CoreStateEvent represents core state changes.
type CoreStateEvent struct {
	State  string  `json:"state"`
	Error  *string `json:"error,omitempty"`
	Uptime int64   `json:"uptime,omitempty"`
}

// ToolsUpdatedEvent represents tools snapshot updates.
type ToolsUpdatedEvent struct {
	ETag  string            `json:"etag"`
	Tools []types.ToolEntry `json:"tools"`
}

// ResourcesUpdatedEvent represents resources snapshot updates.
type ResourcesUpdatedEvent struct {
	ETag      string                `json:"etag"`
	Resources []types.ResourceEntry `json:"resources"`
}

// PromptsUpdatedEvent represents prompts snapshot updates.
type PromptsUpdatedEvent struct {
	ETag    string              `json:"etag"`
	Prompts []types.PromptEntry `json:"prompts"`
}

// LogEntryEvent represents a single log entry.
type LogEntryEvent struct {
	Logger    string          `json:"logger"`
	Level     string          `json:"level"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// ErrorEvent represents an error event.
type ErrorEvent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// DeepLinkEvent represents a deep link navigation event.
type DeepLinkEvent struct {
	Path   string            `json:"path"`
	Params map[string]string `json:"params"`
}

// RuntimeStatusUpdatedEvent represents runtime status updates.
type RuntimeStatusUpdatedEvent struct {
	ETag     string                      `json:"etag"`
	Statuses []types.ServerRuntimeStatus `json:"statuses"`
}

// ServerInitUpdatedEvent represents server init status updates.
type ServerInitUpdatedEvent struct {
	Statuses []types.ServerInitStatus `json:"statuses"`
}

// ActiveClientsUpdatedEvent represents active client updates.
type ActiveClientsUpdatedEvent struct {
	Clients []types.ActiveClient `json:"clients"`
}

// UpdateAvailableEvent represents update notifications.
type UpdateAvailableEvent struct {
	CurrentVersion string              `json:"currentVersion"`
	Latest         types.UpdateRelease `json:"latest"`
}

// Helper functions for event emission

func EmitCoreState(app *application.App, state string, err error) {
	if app == nil {
		return
	}
	event := CoreStateEvent{State: state}
	if err != nil {
		errMsg := err.Error()
		event.Error = &errMsg
	}
	app.Event.Emit(EventCoreState, event)
}

func EmitLogEntry(app *application.App, entry domain.LogEntry) {
	if app == nil {
		return
	}
	data := mustMarshalLogData(entry.Data)
	event := LogEntryEvent{
		Logger:    entry.Logger,
		Level:     string(entry.Level),
		Timestamp: formatTimestamp(entry.Timestamp),
		Data:      data,
	}
	app.Event.Emit(EventLogEntry, event)
}

func mustMarshalLogData(data map[string]any) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return raw
}

func EmitError(app *application.App, code, message, details string) {
	if app == nil {
		return
	}
	event := ErrorEvent{
		Code:    code,
		Message: message,
		Details: details,
	}
	app.Event.Emit(EventError, event)
}

func EmitDeepLink(app *application.App, path string, params map[string]string) {
	if app == nil {
		return
	}
	if params == nil {
		params = map[string]string{}
	}
	event := DeepLinkEvent{
		Path:   path,
		Params: params,
	}
	app.Event.Emit(EventDeepLink, event)
}

func EmitRuntimeStatusUpdated(app *application.App, snapshot domain.RuntimeStatusSnapshot) {
	if app == nil {
		return
	}
	statuses := make([]types.ServerRuntimeStatus, 0, len(snapshot.Statuses))
	for _, s := range snapshot.Statuses {
		instances := make([]types.InstanceStatus, 0, len(s.Instances))
		metrics := types.PoolMetrics{
			StartCount:      s.Metrics.StartCount,
			StopCount:       s.Metrics.StopCount,
			TotalCalls:      s.Metrics.TotalCalls,
			TotalErrors:     s.Metrics.TotalErrors,
			TotalDurationMs: s.Metrics.TotalDuration.Milliseconds(),
		}
		if !s.Metrics.LastCallAt.IsZero() {
			metrics.LastCallAt = s.Metrics.LastCallAt.UTC().Format(time.RFC3339Nano)
		}
		for _, inst := range s.Instances {
			instances = append(instances, types.InstanceStatus{
				ID:              inst.ID,
				State:           string(inst.State),
				BusyCount:       inst.BusyCount,
				LastActive:      formatTimestamp(inst.LastActive),
				SpawnedAt:       formatTimestamp(inst.SpawnedAt),
				HandshakedAt:    formatTimestamp(inst.HandshakedAt),
				LastHeartbeatAt: formatTimestamp(inst.LastHeartbeatAt),
				LastStartCause:  mapping.MapStartCause(inst.LastStartCause),
			})
		}
		statuses = append(statuses, types.ServerRuntimeStatus{
			SpecKey:    s.SpecKey,
			ServerName: s.ServerName,
			Instances:  instances,
			Stats: types.PoolStats{
				Total:        s.Stats.Total,
				Ready:        s.Stats.Ready,
				Busy:         s.Stats.Busy,
				Starting:     s.Stats.Starting,
				Initializing: s.Stats.Initializing,
				Handshaking:  s.Stats.Handshaking,
				Draining:     s.Stats.Draining,
				Failed:       s.Stats.Failed,
			},
			Metrics: metrics,
		})
	}
	event := RuntimeStatusUpdatedEvent{
		ETag:     snapshot.ETag,
		Statuses: statuses,
	}
	app.Event.Emit(EventRuntimeStatusUpdated, event)
}

func EmitServerInitUpdated(app *application.App, snapshot domain.ServerInitStatusSnapshot) {
	if app == nil {
		return
	}
	statuses := make([]types.ServerInitStatus, 0, len(snapshot.Statuses))
	for _, s := range snapshot.Statuses {
		statuses = append(statuses, types.ServerInitStatus{
			SpecKey:    s.SpecKey,
			ServerName: s.ServerName,
			MinReady:   s.MinReady,
			Ready:      s.Ready,
			Failed:     s.Failed,
			State:      string(s.State),
			LastError:  s.LastError,
			UpdatedAt:  formatTimestamp(s.UpdatedAt),
		})
	}
	event := ServerInitUpdatedEvent{
		Statuses: statuses,
	}
	app.Event.Emit(EventServerInitUpdated, event)
}

func EmitActiveClientsUpdated(app *application.App, snapshot domain.ActiveClientSnapshot) {
	if app == nil {
		return
	}
	clients := make([]types.ActiveClient, 0, len(snapshot.Clients))
	for _, client := range snapshot.Clients {
		clients = append(clients, types.ActiveClient{
			Client:        client.Client,
			PID:           client.PID,
			Tags:          append([]string(nil), client.Tags...),
			Server:        client.Server,
			LastHeartbeat: formatTimestamp(client.LastHeartbeat),
		})
	}
	event := ActiveClientsUpdatedEvent{
		Clients: clients,
	}
	app.Event.Emit(EventActiveClientsUpdated, event)
}

func EmitUpdateAvailable(app *application.App, event UpdateAvailableEvent) {
	if app == nil {
		return
	}
	app.Event.Emit(EventUpdateAvailable, event)
}

func formatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
