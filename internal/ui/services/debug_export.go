package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry/diagnostics"
	"mcpv/internal/ui"
)

const (
	defaultDiagnosticsMode       = "safe"
	defaultMaxLogEntries         = 200
	defaultMaxEventEntries       = 2000
	defaultStuckThresholdSeconds = 30
)

type diagnosticsBundle struct {
	GeneratedAt string                        `json:"generatedAt"`
	Report      string                        `json:"report,omitempty"`
	Snapshot    json.RawMessage               `json:"snapshot,omitempty"`
	Metrics     string                        `json:"metrics,omitempty"`
	Logs        []diagnosticsLogEntry         `json:"logs,omitempty"`
	Events      map[string][]diagnosticsEvent `json:"events,omitempty"`
	Stuck       map[string]diagnosticsStuck   `json:"stuck,omitempty"`
	Dropped     diagnosticsDropped            `json:"dropped,omitempty"`
	Redaction   diagnosticsRedaction          `json:"redaction"`
	Errors      []diagnosticsBundleError      `json:"errors,omitempty"`
}

type diagnosticsDropped struct {
	Events uint64 `json:"events,omitempty"`
	Logs   uint64 `json:"logs,omitempty"`
}

type diagnosticsRedaction struct {
	Mode              string `json:"mode"`
	ContainsSensitive bool   `json:"containsSensitive"`
}

type diagnosticsBundleError struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

type diagnosticsLogEntry struct {
	Logger    string         `json:"logger,omitempty"`
	Level     string         `json:"level"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

type diagnosticsEvent struct {
	SpecKey    string            `json:"specKey,omitempty"`
	ServerName string            `json:"serverName,omitempty"`
	AttemptID  string            `json:"attemptId,omitempty"`
	Step       string            `json:"step"`
	Phase      string            `json:"phase"`
	Timestamp  string            `json:"timestamp"`
	DurationMs int64             `json:"durationMs,omitempty"`
	Error      string            `json:"error,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type diagnosticsStuck struct {
	Step       string `json:"step"`
	Since      string `json:"since"`
	DurationMs int64  `json:"durationMs"`
	LastError  string `json:"lastError,omitempty"`
	AttemptID  string `json:"attemptId,omitempty"`
}

// ExportDiagnosticsBundle exports a diagnostics bundle with events, logs, and metrics.
func (s *DebugService) ExportDiagnosticsBundle(ctx context.Context, options DiagnosticsExportOptions) (DiagnosticsBundleResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	mode := strings.TrimSpace(options.Mode)
	if mode == "" {
		mode = defaultDiagnosticsMode
	}

	includeSnapshot := options.IncludeSnapshot
	includeMetrics := options.IncludeMetrics
	includeLogs := options.IncludeLogs
	includeEvents := options.IncludeEvents
	includeStuck := options.IncludeStuck
	if !includeSnapshot && !includeMetrics && !includeLogs && !includeEvents && !includeStuck {
		includeSnapshot = true
		includeMetrics = true
		includeLogs = true
		includeEvents = true
		includeStuck = true
	}

	bundle := diagnosticsBundle{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Redaction: diagnosticsRedaction{
			Mode:              mode,
			ContainsSensitive: mode == "deep",
		},
	}

	if includeSnapshot {
		snapshot, err := s.ExportDebugSnapshot(ctx)
		if err != nil {
			bundle.Errors = append(bundle.Errors, diagnosticsBundleError{Source: "snapshot", Message: err.Error()})
		} else {
			bundle.Snapshot = snapshot.Snapshot
		}
	}

	coreApp, coreErr := s.deps.getCoreApp()
	if coreErr != nil {
		bundle.Errors = append(bundle.Errors, diagnosticsBundleError{Source: "coreApp", Message: coreErr.Error()})
	}

	if includeMetrics {
		if coreErr != nil {
			bundle.Errors = append(bundle.Errors, diagnosticsBundleError{Source: "metrics", Message: "core app unavailable"})
		} else {
			metrics, err := coreApp.MetricsText()
			if err != nil {
				bundle.Errors = append(bundle.Errors, diagnosticsBundleError{Source: "metrics", Message: err.Error()})
			} else {
				bundle.Metrics = metrics
			}
		}
	}

	hub := (*diagnostics.Hub)(nil)
	if coreErr == nil {
		hub = coreApp.Diagnostics()
	}
	if hub == nil {
		if includeEvents || includeLogs || includeStuck {
			bundle.Errors = append(bundle.Errors, diagnosticsBundleError{Source: "diagnostics", Message: "diagnostics hub unavailable"})
		}
	}

	if hub != nil {
		bundle.Dropped = diagnosticsDropped{
			Events: hub.DroppedEvents(),
			Logs:   hub.DroppedLogs(),
		}
	}

	var rawEvents []diagnostics.Event
	if hub != nil && (includeEvents || includeStuck) {
		maxEvents := options.MaxEventEntries
		if maxEvents <= 0 {
			maxEvents = defaultMaxEventEntries
		}
		rawEvents = hub.Events()
		if len(rawEvents) > maxEvents {
			rawEvents = rawEvents[len(rawEvents)-maxEvents:]
		}
	}
	if includeEvents && len(rawEvents) > 0 {
		bundle.Events = mapEvents(rawEvents, mode)
	}
	reportEvents := mapEvents(rawEvents, mode)

	if includeLogs && hub != nil {
		maxLogs := options.MaxLogEntries
		if maxLogs <= 0 {
			maxLogs = defaultMaxLogEntries
		}
		level := parseLogLevel(options.LogLevel)
		logs := hub.Logs()
		bundle.Logs = mapLogs(logs, level, maxLogs)
	}

	if includeStuck && len(rawEvents) > 0 {
		threshold := time.Duration(defaultStuckThresholdSeconds) * time.Second
		if options.StuckThresholdMs > 0 {
			threshold = time.Duration(options.StuckThresholdMs) * time.Millisecond
		}
		bundle.Stuck = computeStuck(reportEvents, threshold)
	}

	bundle.Report = buildDiagnosticsReport(bundle, reportEvents)

	payload, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return DiagnosticsBundleResponse{}, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Failed to serialize diagnostics bundle", err.Error())
	}

	return DiagnosticsBundleResponse{
		Payload:     json.RawMessage(payload),
		Size:        int64(len(payload)),
		GeneratedAt: bundle.GeneratedAt,
	}, nil
}

func mapEvents(events []diagnostics.Event, mode string) map[string][]diagnosticsEvent {
	if len(events) == 0 {
		return nil
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	result := make(map[string][]diagnosticsEvent)
	for _, event := range events {
		attrs := formatAttributes(event.Attributes, event.Sensitive, mode)
		payload := diagnosticsEvent{
			SpecKey:    event.SpecKey,
			ServerName: event.ServerName,
			AttemptID:  event.AttemptID,
			Step:       event.Step,
			Phase:      string(event.Phase),
			Timestamp:  event.Timestamp.UTC().Format(time.RFC3339Nano),
			Error:      event.Error,
			Attributes: attrs,
		}
		if event.Duration > 0 {
			payload.DurationMs = event.Duration.Milliseconds()
		}
		key := event.ServerName
		if strings.TrimSpace(key) == "" {
			key = event.SpecKey
		}
		result[key] = append(result[key], payload)
	}
	return result
}

func formatAttributes(attrs map[string]string, sensitive map[string]string, mode string) map[string]string {
	if mode == "deep" {
		return mergeStringMaps(attrs, sensitive)
	}
	return diagnostics.RedactMap(attrs)
}

func mergeStringMaps(primary map[string]string, secondary map[string]string) map[string]string {
	if len(primary) == 0 && len(secondary) == 0 {
		return nil
	}
	out := make(map[string]string, len(primary)+len(secondary))
	for key, value := range primary {
		out[key] = value
	}
	for key, value := range secondary {
		out[key] = value
	}
	return out
}

func mapLogs(entries []domain.LogEntry, minLevel logLevel, maxEntries int) []diagnosticsLogEntry {
	if len(entries) == 0 {
		return nil
	}
	filtered := make([]diagnosticsLogEntry, 0, len(entries))
	for _, entry := range entries {
		if logLevelRank(entry.Level) < minLevel.rank {
			continue
		}
		filtered = append(filtered, diagnosticsLogEntry{
			Logger:    entry.Logger,
			Level:     string(entry.Level),
			Timestamp: entry.Timestamp.UTC().Format(time.RFC3339Nano),
			Data:      sanitizeLogData(entry.Data),
		})
	}
	if len(filtered) > maxEntries {
		filtered = filtered[len(filtered)-maxEntries:]
	}
	return filtered
}

type logLevel struct {
	name string
	rank int
}

func parseLogLevel(value string) logLevel {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return logLevel{name: "debug", rank: 10}
	case "info", "":
		return logLevel{name: "info", rank: 20}
	case "notice":
		return logLevel{name: "notice", rank: 25}
	case "warning", "warn":
		return logLevel{name: "warning", rank: 30}
	case "error":
		return logLevel{name: "error", rank: 40}
	case "critical":
		return logLevel{name: "critical", rank: 50}
	case "alert":
		return logLevel{name: "alert", rank: 60}
	case "emergency":
		return logLevel{name: "emergency", rank: 70}
	default:
		return logLevel{name: value, rank: 20}
	}
}

func logLevelRank(level domain.LogLevel) int {
	return parseLogLevel(string(level)).rank
}

func sanitizeLogData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return nil
	}
	out := make(map[string]any, len(data))
	for key, value := range data {
		out[key] = sanitizeValue(key, value)
	}
	return out
}

func sanitizeValue(key string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeLogData(typed)
	case map[string]string:
		return diagnostics.RedactMap(typed)
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, sanitizeValue(key, item))
		}
		return result
	case string:
		return diagnostics.RedactValue(key, typed)
	default:
		if diagnostics.ContainsSensitiveKey(key) {
			return "***"
		}
		return value
	}
}

func computeStuck(events map[string][]diagnosticsEvent, threshold time.Duration) map[string]diagnosticsStuck {
	if len(events) == 0 {
		return nil
	}
	stuck := make(map[string]diagnosticsStuck)
	now := time.Now()
	for key, serverEvents := range events {
		var currentStep string
		var stepStarted time.Time
		var lastError string
		var attemptID string
		for _, event := range serverEvents {
			timestamp, err := time.Parse(time.RFC3339Nano, event.Timestamp)
			if err != nil {
				continue
			}
			switch event.Phase {
			case string(diagnostics.PhaseEnter):
				currentStep = event.Step
				stepStarted = timestamp
				attemptID = event.AttemptID
			case string(diagnostics.PhaseExit):
				if currentStep == event.Step {
					currentStep = ""
					stepStarted = time.Time{}
				}
			case string(diagnostics.PhaseError):
				lastError = event.Error
				if currentStep == "" {
					currentStep = event.Step
					stepStarted = timestamp
					attemptID = event.AttemptID
				}
			}
		}
		if currentStep == "" || stepStarted.IsZero() {
			continue
		}
		duration := now.Sub(stepStarted)
		if duration < threshold {
			continue
		}
		stuck[key] = diagnosticsStuck{
			Step:       currentStep,
			Since:      stepStarted.UTC().Format(time.RFC3339Nano),
			DurationMs: duration.Milliseconds(),
			LastError:  lastError,
			AttemptID:  attemptID,
		}
	}
	if len(stuck) == 0 {
		return nil
	}
	return stuck
}

func buildDiagnosticsReport(bundle diagnosticsBundle, events map[string][]diagnosticsEvent) string {
	var builder strings.Builder
	builder.WriteString("-------------------------------------\n")
	builder.WriteString("MCPV Diagnostics Report (Full Report Below)\n")
	builder.WriteString("-------------------------------------\n")
	builder.WriteString(fmt.Sprintf("Generated At:         %s\n", bundle.GeneratedAt))
	builder.WriteString(fmt.Sprintf("Redaction Mode:       %s\n", bundle.Redaction.Mode))
	builder.WriteString(fmt.Sprintf("Sensitive Included:   %t\n", bundle.Redaction.ContainsSensitive))
	builder.WriteString(fmt.Sprintf("Events Captured:      %d\n", countEvents(events)))
	builder.WriteString(fmt.Sprintf("Logs Captured:        %d\n", len(bundle.Logs)))
	builder.WriteString(fmt.Sprintf("Events Dropped:       %d\n", bundle.Dropped.Events))
	builder.WriteString(fmt.Sprintf("Logs Dropped:         %d\n", bundle.Dropped.Logs))
	builder.WriteString("\n")

	if snapshot := parseDebugSnapshot(bundle.Snapshot); snapshot != nil {
		builder.WriteString("Core Summary\n")
		builder.WriteString("------------\n")
		builder.WriteString(fmt.Sprintf("Core State:           %s\n", snapshot.Core.State))
		builder.WriteString(fmt.Sprintf("Uptime (ms):          %d\n", snapshot.Core.UptimeMs))
		if snapshot.Info != nil {
			builder.WriteString(fmt.Sprintf("Version:              %s\n", snapshot.Info.Version))
			builder.WriteString(fmt.Sprintf("Build:                %s\n", snapshot.Info.Build))
		}
		if snapshot.ConfigPath != "" {
			builder.WriteString(fmt.Sprintf("Config Path:          %s\n", snapshot.ConfigPath))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("Stuck Analysis\n")
	builder.WriteString("-------------\n")
	if len(bundle.Stuck) == 0 {
		builder.WriteString("No stuck servers detected.\n")
	} else {
		keys := sortedKeys(bundle.Stuck)
		for _, key := range keys {
			entry := bundle.Stuck[key]
			builder.WriteString(fmt.Sprintf("- %s: step=%s duration=%s lastError=%s\n",
				key,
				entry.Step,
				formatDuration(entry.DurationMs),
				formatInline(entry.LastError),
			))
		}
	}
	builder.WriteString("\n")

	builder.WriteString("Recent Errors\n")
	builder.WriteString("-------------\n")
	errors := collectErrorEvents(events, 12)
	if len(errors) == 0 {
		builder.WriteString("No recent error events.\n")
	} else {
		for _, entry := range errors {
			builder.WriteString(fmt.Sprintf("- %s %s %s: %s\n",
				entry.Timestamp,
				entry.ServerName,
				entry.Step,
				formatInline(entry.Error),
			))
		}
	}
	builder.WriteString("\n")

	builder.WriteString("Stage Timeline (latest)\n")
	builder.WriteString("-----------------------\n")
	if len(events) == 0 {
		builder.WriteString("No events captured.\n")
	} else {
		keys := sortedKeys(events)
		for _, key := range keys {
			builder.WriteString(fmt.Sprintf("\n[%s]\n", key))
			entries := events[key]
			if len(entries) > 12 {
				entries = entries[len(entries)-12:]
			}
			for _, event := range entries {
				builder.WriteString(fmt.Sprintf("%s %-18s %-6s %s\n",
					event.Timestamp,
					event.Step,
					event.Phase,
					formatInline(event.Error),
				))
			}
		}
	}
	builder.WriteString("\n")

	if len(bundle.Logs) > 0 {
		builder.WriteString("Recent Logs\n")
		builder.WriteString("-----------\n")
		logs := bundle.Logs
		if len(logs) > 10 {
			logs = logs[len(logs)-10:]
		}
		for _, entry := range logs {
			builder.WriteString(fmt.Sprintf("%s %-7s %s\n",
				entry.Timestamp,
				strings.ToUpper(entry.Level),
				formatInline(formatLogMessage(entry.Data)),
			))
		}
		builder.WriteString("\n")
	}

	if len(bundle.Errors) > 0 {
		builder.WriteString("Export Warnings\n")
		builder.WriteString("---------------\n")
		for _, exportErr := range bundle.Errors {
			builder.WriteString(fmt.Sprintf("- %s: %s\n", exportErr.Source, formatInline(exportErr.Message)))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("Raw Data\n")
	builder.WriteString("--------\n")
	builder.WriteString("See JSON payload below.\n")

	return builder.String()
}

func parseDebugSnapshot(raw json.RawMessage) *debugSnapshot {
	if len(raw) == 0 {
		return nil
	}
	var snapshot debugSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil
	}
	return &snapshot
}

func countEvents(events map[string][]diagnosticsEvent) int {
	count := 0
	for _, entries := range events {
		count += len(entries)
	}
	return count
}

func sortedKeys[T any](input map[string]T) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func collectErrorEvents(events map[string][]diagnosticsEvent, limit int) []diagnosticsEvent {
	if limit <= 0 {
		limit = 10
	}
	var out []diagnosticsEvent
	for _, entries := range events {
		for _, event := range entries {
			if event.Phase == string(diagnostics.PhaseError) && event.Error != "" {
				out = append(out, event)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp > out[j].Timestamp
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func formatDuration(durationMs int64) string {
	if durationMs <= 0 {
		return "0ms"
	}
	duration := time.Duration(durationMs) * time.Millisecond
	if duration < time.Second {
		return fmt.Sprintf("%dms", durationMs)
	}
	return duration.String()
}

func formatInline(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	if len(trimmed) > 160 {
		return trimmed[:157] + "..."
	}
	return trimmed
}

func formatLogMessage(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	if msg, ok := data["message"].(string); ok {
		return msg
	}
	return ""
}
