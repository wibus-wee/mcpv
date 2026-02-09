package services

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"mcpv/internal/domain"
	"mcpv/internal/ui"
	"mcpv/internal/ui/mapping"
)

type debugSnapshot struct {
	GeneratedAt        string                     `json:"generatedAt"`
	ConfigPath         string                     `json:"configPath,omitempty"`
	Core               debugCoreState             `json:"core"`
	Info               *InfoResponse              `json:"info,omitempty"`
	Bootstrap          *BootstrapProgressResponse `json:"bootstrap,omitempty"`
	Servers            []ServerSpecDetail         `json:"servers,omitempty"`
	ActiveClients      []ActiveClient             `json:"activeClients,omitempty"`
	ServerInitStatuses []ServerInitStatus         `json:"serverInitStatuses,omitempty"`
	RuntimeStatuses    []ServerRuntimeStatus      `json:"runtimeStatuses,omitempty"`
	Errors             []debugSnapshotError       `json:"errors,omitempty"`
}

type debugCoreState struct {
	State    string `json:"state"`
	UptimeMs int64  `json:"uptimeMs"`
	Error    string `json:"error,omitempty"`
}

type debugSnapshotError struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

// ExportDebugSnapshot builds a debug snapshot and returns its JSON payload.
func (s *DebugService) ExportDebugSnapshot(ctx context.Context) (DebugSnapshotResponse, error) {
	manager := s.deps.manager()
	if manager == nil {
		return DebugSnapshotResponse{}, ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}

	now := time.Now().UTC()
	snapshot := debugSnapshot{
		GeneratedAt: now.Format(time.RFC3339Nano),
		ConfigPath:  strings.TrimSpace(manager.GetConfigPath()),
	}
	if snapshot.ConfigPath == "" {
		snapshot.Errors = append(snapshot.Errors, debugSnapshotError{
			Source:  "configPath",
			Message: "config path is empty",
		})
	}

	coreState, uptime, coreErr := manager.GetState()
	snapshot.Core = debugCoreState{
		State:    string(coreState),
		UptimeMs: uptime,
	}
	if coreErr != nil {
		snapshot.Core.Error = coreErr.Error()
	}

	cp, err := manager.GetControlPlane()
	if err != nil {
		snapshot.Errors = append(snapshot.Errors, debugSnapshotError{
			Source:  "controlPlane",
			Message: err.Error(),
		})
	} else {
		info, err := cp.Info(ctx)
		if err != nil {
			appendSnapshotError(&snapshot.Errors, "info", err)
		} else {
			snapshot.Info = &InfoResponse{
				Name:    info.Name,
				Version: info.Version,
				Build:   info.Build,
			}
		}

		progress, err := cp.GetBootstrapProgress(ctx)
		if err != nil {
			appendSnapshotError(&snapshot.Errors, "bootstrap", err)
		} else {
			snapshot.Bootstrap = &BootstrapProgressResponse{
				State:     string(progress.State),
				Total:     progress.Total,
				Completed: progress.Completed,
				Failed:    progress.Failed,
				Current:   progress.Current,
				Errors:    progress.Errors,
			}
		}

		statuses, err := cp.GetServerInitStatus(ctx)
		if err != nil {
			appendSnapshotError(&snapshot.Errors, "serverInitStatus", err)
		} else {
			snapshot.ServerInitStatuses = mapping.MapServerInitStatuses(statuses)
		}

		pools, err := cp.GetPoolStatus(ctx)
		if err != nil {
			appendSnapshotError(&snapshot.Errors, "runtimeStatus", err)
		} else {
			snapshot.RuntimeStatuses = mapping.MapRuntimeStatuses(pools)
		}

		activeClients, err := cp.ListActiveClients(ctx)
		if err != nil {
			appendSnapshotError(&snapshot.Errors, "activeClients", err)
		} else {
			snapshot.ActiveClients = mapping.MapActiveClients(activeClients)
		}

		catalog := cp.GetCatalog()
		snapshot.Servers = mapServerSummaries(catalog)
	}

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return DebugSnapshotResponse{}, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Failed to serialize debug snapshot", err.Error())
	}

	return DebugSnapshotResponse{
		Snapshot:    json.RawMessage(payload),
		Size:        int64(len(payload)),
		GeneratedAt: snapshot.GeneratedAt,
	}, nil
}

func appendSnapshotError(errors *[]debugSnapshotError, source string, err error) {
	if err == nil {
		return
	}
	*errors = append(*errors, debugSnapshotError{
		Source:  source,
		Message: err.Error(),
	})
}

func mapServerSummaries(catalog domain.Catalog) []ServerSpecDetail {
	if len(catalog.Specs) == 0 {
		return nil
	}
	result := make([]ServerSpecDetail, 0, len(catalog.Specs))
	for _, spec := range catalog.Specs {
		specKey := domain.SpecFingerprint(spec)
		result = append(result, mapping.MapServerSpecDetail(spec, specKey))
	}
	return result
}
