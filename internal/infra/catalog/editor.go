package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/fsutil"
)

type EditorErrorKind string

const (
	EditorErrorInvalidRequest  EditorErrorKind = "invalid_request"
	EditorErrorProfileNotFound EditorErrorKind = "profile_not_found"
	EditorErrorInvalidConfig   EditorErrorKind = "invalid_config"
)

type EditorError struct {
	Kind    EditorErrorKind
	Message string
	Err     error
}

func (e *EditorError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *EditorError) Unwrap() error {
	return e.Err
}

type ConfigInfo struct {
	Path       string
	IsWritable bool
}

type ImportRequest struct {
	Profiles []string
	Servers  []domain.ServerSpec
}

type Editor struct {
	path   string
	logger *zap.Logger
}

func NewEditor(path string, logger *zap.Logger) *Editor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Editor{
		path:   strings.TrimSpace(path),
		logger: logger.Named("catalog_editor"),
	}
}

func (e *Editor) Inspect(ctx context.Context) (ConfigInfo, error) {
	path, err := e.storePath()
	if err != nil {
		return ConfigInfo{}, err
	}
	return ConfigInfo{
		Path:       path,
		IsWritable: isWritable(path),
	}, nil
}

func (e *Editor) ImportServers(ctx context.Context, req ImportRequest) error {
	normalized, err := NormalizeImportRequest(req)
	if err != nil {
		return err
	}
	storePath, err := e.storePath()
	if err != nil {
		return err
	}

	updates := make([]ProfileUpdate, 0, len(normalized.Profiles))
	for _, name := range normalized.Profiles {
		path, err := ResolveProfilePath(storePath, name)
		if err != nil {
			return &EditorError{Kind: EditorErrorProfileNotFound, Message: "Profile not found", Err: err}
		}
		update, err := BuildProfileUpdate(path, normalized.Servers)
		if err != nil {
			return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update profile", Err: err}
		}
		updates = append(updates, update)
	}

	for _, update := range updates {
		if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
			return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write profile file", Err: err}
		}
	}
	return nil
}

func (e *Editor) SetServerDisabled(ctx context.Context, profileName, serverName string, disabled bool) error {
	profileName = strings.TrimSpace(profileName)
	serverName = strings.TrimSpace(serverName)
	if profileName == "" || serverName == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Profile and server are required"}
	}

	storePath, err := e.storePath()
	if err != nil {
		return err
	}
	path, err := ResolveProfilePath(storePath, profileName)
	if err != nil {
		return &EditorError{Kind: EditorErrorProfileNotFound, Message: "Profile not found", Err: err}
	}
	update, err := SetServerDisabled(path, serverName, disabled)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update server", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write profile file", Err: err}
	}
	return nil
}

func (e *Editor) DeleteServer(ctx context.Context, profileName, serverName string) error {
	profileName = strings.TrimSpace(profileName)
	serverName = strings.TrimSpace(serverName)
	if profileName == "" || serverName == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Profile and server are required"}
	}

	storePath, err := e.storePath()
	if err != nil {
		return err
	}
	path, err := ResolveProfilePath(storePath, profileName)
	if err != nil {
		return &EditorError{Kind: EditorErrorProfileNotFound, Message: "Profile not found", Err: err}
	}
	update, err := DeleteServer(path, serverName)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to delete server", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write profile file", Err: err}
	}
	return nil
}

func (e *Editor) CreateProfile(ctx context.Context, profileName string) error {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Profile name is required"}
	}

	storePath, err := e.storePath()
	if err != nil {
		return err
	}
	if _, err := CreateProfile(storePath, profileName); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to create profile", Err: err}
	}
	return nil
}

func (e *Editor) DeleteProfile(ctx context.Context, profileName string) error {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Profile name is required"}
	}

	storePath, err := e.storePath()
	if err != nil {
		return err
	}

	storeLoader := NewProfileStoreLoader(e.logger)
	store, err := storeLoader.Load(ctx, storePath, ProfileStoreOptions{AllowCreate: false})
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to load profile store", Err: err}
	}
	for caller, profile := range store.Callers {
		if profile == profileName {
			return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Profile is referenced by callers", Err: errors.New(caller)}
		}
	}

	if err := DeleteProfile(storePath, profileName); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to delete profile", Err: err}
	}
	return nil
}

func (e *Editor) SetCallerMapping(ctx context.Context, caller, profile string) error {
	caller = strings.TrimSpace(caller)
	profile = strings.TrimSpace(profile)
	if caller == "" || profile == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Caller and profile are required"}
	}

	storePath, err := e.storePath()
	if err != nil {
		return err
	}
	storeLoader := NewProfileStoreLoader(e.logger)
	store, err := storeLoader.Load(ctx, storePath, ProfileStoreOptions{AllowCreate: false})
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to load profile store", Err: err}
	}

	update, err := SetCallerMapping(storePath, caller, profile, store.Profiles)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update caller mapping", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write callers file", Err: err}
	}
	return nil
}

func (e *Editor) RemoveCallerMapping(ctx context.Context, caller string) error {
	caller = strings.TrimSpace(caller)
	if caller == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Caller is required"}
	}

	storePath, err := e.storePath()
	if err != nil {
		return err
	}
	storeLoader := NewProfileStoreLoader(e.logger)
	store, err := storeLoader.Load(ctx, storePath, ProfileStoreOptions{AllowCreate: false})
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to load profile store", Err: err}
	}

	update, err := RemoveCallerMapping(storePath, caller, store.Profiles)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to remove caller mapping", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write callers file", Err: err}
	}
	return nil
}

func (e *Editor) SetProfileSubAgentEnabled(ctx context.Context, profileName string, enabled bool) error {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Profile name is required"}
	}

	storePath, err := e.storePath()
	if err != nil {
		return err
	}
	path, err := ResolveProfilePath(storePath, profileName)
	if err != nil {
		return &EditorError{Kind: EditorErrorProfileNotFound, Message: "Profile not found", Err: err}
	}
	update, err := SetProfileSubAgentEnabled(path, enabled)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update SubAgent config", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write profile file", Err: err}
	}
	return nil
}

func NormalizeImportRequest(req ImportRequest) (ImportRequest, error) {
	if len(req.Profiles) == 0 {
		return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: "At least one profile is required"}
	}
	if len(req.Servers) == 0 {
		return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: "At least one server is required"}
	}

	profileNames := make([]string, 0, len(req.Profiles))
	seenProfiles := make(map[string]struct{}, len(req.Profiles))
	for _, name := range req.Profiles {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: "Profile name is required"}
		}
		if _, exists := seenProfiles[trimmed]; exists {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: fmt.Sprintf("Duplicate profile name %q", trimmed)}
		}
		seenProfiles[trimmed] = struct{}{}
		profileNames = append(profileNames, trimmed)
	}

	servers := make([]domain.ServerSpec, 0, len(req.Servers))
	seenServers := make(map[string]struct{}, len(req.Servers))
	for _, server := range req.Servers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: "Server name is required"}
		}
		if len(server.Cmd) == 0 {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: fmt.Sprintf("Server %q: cmd is required", name)}
		}
		for _, cmd := range server.Cmd {
			if strings.TrimSpace(cmd) == "" {
				return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: fmt.Sprintf("Server %q: cmd contains empty value", name)}
			}
		}
		if _, exists := seenServers[name]; exists {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: fmt.Sprintf("Duplicate server name %q", name)}
		}
		seenServers[name] = struct{}{}

		servers = append(servers, domain.ServerSpec{
			Name:                name,
			Cmd:                 append([]string{}, server.Cmd...),
			Env:                 normalizeImportEnv(server.Env),
			Cwd:                 strings.TrimSpace(server.Cwd),
			IdleSeconds:         60,
			MaxConcurrent:       domain.DefaultMaxConcurrent,
			Sticky:              false,
			Persistent:          false,
			MinReady:            0,
			DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
			ProtocolVersion:     domain.DefaultProtocolVersion,
		})
	}

	return ImportRequest{
		Profiles: profileNames,
		Servers:  servers,
	}, nil
}

func normalizeImportEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	cleaned := make(map[string]string, len(env))
	for key, value := range env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		cleaned[key] = value
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func (e *Editor) storePath() (string, error) {
	if e.path == "" {
		return "", &EditorError{Kind: EditorErrorInvalidConfig, Message: "Profile store path is required"}
	}
	info, err := os.Stat(e.path)
	if err != nil {
		return "", &EditorError{Kind: EditorErrorInvalidConfig, Message: "Profile store path is not available", Err: err}
	}
	if !info.IsDir() {
		return "", &EditorError{Kind: EditorErrorInvalidConfig, Message: fmt.Sprintf("Profile store path must be a directory: %s", e.path)}
	}
	return e.path, nil
}

func isWritable(path string) bool {
	testFile := filepath.Join(path, ".write_test")
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	return os.Remove(testFile) == nil
}
