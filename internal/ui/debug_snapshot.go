package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mcpd/internal/domain"
)

type debugSnapshot struct {
	GeneratedAt        string                     `json:"generatedAt"`
	ConfigPath         string                     `json:"configPath,omitempty"`
	Core               debugCoreState             `json:"core"`
	Info               *InfoResponse              `json:"info,omitempty"`
	Bootstrap          *BootstrapProgressResponse `json:"bootstrap,omitempty"`
	Profiles           []ProfileSummary           `json:"profiles,omitempty"`
	Callers            map[string]string          `json:"callers,omitempty"`
	ActiveCallers      []ActiveCaller             `json:"activeCallers,omitempty"`
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

// ExportDebugSnapshot writes a debug snapshot to disk and returns its location.
func (s *DebugService) ExportDebugSnapshot(ctx context.Context) (DebugSnapshotResponse, error) {
	manager := s.deps.manager()
	if manager == nil {
		return DebugSnapshotResponse{}, NewUIError(ErrCodeInternal, "Manager not initialized")
	}

	now := time.Now().UTC()
	snapshot := debugSnapshot{
		GeneratedAt: now.Format(time.RFC3339Nano),
		ConfigPath:  strings.TrimSpace(manager.GetConfigPath()),
	}

	coreState, coreErr, uptime := manager.GetState()
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
			snapshot.ServerInitStatuses = mapServerInitStatuses(statuses)
		}

		pools, err := cp.GetPoolStatus(ctx)
		if err != nil {
			appendSnapshotError(&snapshot.Errors, "runtimeStatus", err)
		} else {
			snapshot.RuntimeStatuses = mapRuntimeStatuses(pools)
		}

		activeCallers, err := cp.ListActiveCallers(ctx)
		if err != nil {
			appendSnapshotError(&snapshot.Errors, "activeCallers", err)
		} else {
			snapshot.ActiveCallers = mapActiveCallers(activeCallers)
		}

		store := cp.GetProfileStore()
		snapshot.Profiles = mapProfileSummaries(store)
		if len(store.Callers) > 0 {
			snapshot.Callers = make(map[string]string, len(store.Callers))
			for key, value := range store.Callers {
				snapshot.Callers[key] = value
			}
		}
	}

	outputDir := snapshot.ConfigPath
	if outputDir == "" {
		outputDir = os.TempDir()
		snapshot.Errors = append(snapshot.Errors, debugSnapshotError{
			Source:  "configPath",
			Message: "config path is empty; using temp dir",
		})
	}

	debugDir := filepath.Join(outputDir, "debug")
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		return DebugSnapshotResponse{}, NewUIErrorWithDetails(ErrCodeInternal, "Failed to create debug directory", err.Error())
	}

	filename := fmt.Sprintf("mcpd-debug-%s.json", now.Format("20060102-150405"))
	path := filepath.Join(debugDir, filename)
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return DebugSnapshotResponse{}, NewUIErrorWithDetails(ErrCodeInternal, "Failed to serialize debug snapshot", err.Error())
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return DebugSnapshotResponse{}, NewUIErrorWithDetails(ErrCodeInternal, "Failed to write debug snapshot", err.Error())
	}

	return DebugSnapshotResponse{
		Path:        path,
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

func mapProfileSummaries(store domain.ProfileStore) []ProfileSummary {
	result := make([]ProfileSummary, 0, len(store.Profiles))
	for name, profile := range store.Profiles {
		result = append(result, ProfileSummary{
			Name:        name,
			ServerCount: len(profile.Catalog.Specs),
			IsDefault:   name == domain.DefaultProfileName,
		})
	}
	return result
}
