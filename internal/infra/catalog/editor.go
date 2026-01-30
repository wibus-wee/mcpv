package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/fsutil"
)

// EditorErrorKind classifies catalog editor errors.
type EditorErrorKind string

const (
	EditorErrorInvalidRequest EditorErrorKind = "invalid_request"
	EditorErrorInvalidConfig  EditorErrorKind = "invalid_config"
)

// EditorError wraps catalog editor failures with a kind and message.
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

// ConfigInfo reports the resolved catalog path and write capability.
type ConfigInfo struct {
	Path       string
	IsWritable bool
}

// ImportRequest describes a bulk server import.
type ImportRequest struct {
	Servers []domain.ServerSpec
}

// Editor manages catalog and runtime configuration updates.
type Editor struct {
	path   string
	logger *zap.Logger
}

// NewEditor constructs a catalog editor scoped to a profile store path.
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
	_ = ctx
	path, err := e.configPath(false)
	if err != nil {
		return ConfigInfo{}, err
	}
	return ConfigInfo{
		Path:       path,
		IsWritable: isWritableFile(path),
	}, nil
}

func (e *Editor) ImportServers(ctx context.Context, req ImportRequest) error {
	_ = ctx
	normalized, err := NormalizeImportRequest(req)
	if err != nil {
		return err
	}
	configPath, err := e.configPath(false)
	if err != nil {
		return err
	}

	update, err := BuildProfileUpdate(configPath, normalized.Servers)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update config", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write config file", Err: err}
	}
	return nil
}

// UpdateRuntimeConfig updates runtime.yaml in the profile store.
func (e *Editor) UpdateRuntimeConfig(ctx context.Context, update RuntimeConfigUpdate) error {
	_ = ctx
	configPath, err := e.configPath(false)
	if err != nil {
		return err
	}

	runtimeUpdate, err := UpdateRuntimeConfig(configPath, update)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update runtime config", Err: err}
	}
	if err := writeRuntimeUpdate(runtimeUpdate); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write runtime config", Err: err}
	}
	return nil
}

func (e *Editor) UpdateSubAgentConfig(ctx context.Context, update SubAgentConfigUpdate) error {
	_ = ctx
	configPath, err := e.configPath(false)
	if err != nil {
		return err
	}

	runtimeUpdate, err := UpdateSubAgentConfig(configPath, update)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update SubAgent config", Err: err}
	}
	if err := writeRuntimeUpdate(runtimeUpdate); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write runtime config", Err: err}
	}
	return nil
}

func (e *Editor) CreateServer(ctx context.Context, spec domain.ServerSpec) error {
	_ = ctx
	normalized, err := normalizeEditorServerSpec(spec)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: err.Error()}
	}
	if errs := validateServerSpec(normalized, 0); len(errs) > 0 {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: strings.Join(errs, "; ")}
	}

	configPath, err := e.configPath(false)
	if err != nil {
		return err
	}
	update, err := CreateServer(configPath, normalized)
	if err != nil {
		if errors.Is(err, ErrServerExists) {
			return &EditorError{Kind: EditorErrorInvalidRequest, Message: err.Error()}
		}
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to create server", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write config file", Err: err}
	}
	return nil
}

func (e *Editor) UpdateServer(ctx context.Context, spec domain.ServerSpec) error {
	_ = ctx
	normalized, err := normalizeEditorServerSpec(spec)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: err.Error()}
	}
	if errs := validateServerSpec(normalized, 0); len(errs) > 0 {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: strings.Join(errs, "; ")}
	}

	configPath, err := e.configPath(false)
	if err != nil {
		return err
	}
	update, err := UpdateServer(configPath, normalized)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return &EditorError{Kind: EditorErrorInvalidRequest, Message: err.Error()}
		}
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update server", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write config file", Err: err}
	}
	return nil
}

func (e *Editor) SetServerDisabled(ctx context.Context, serverName string, disabled bool) error {
	_ = ctx
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Server name is required"}
	}

	configPath, err := e.configPath(false)
	if err != nil {
		return err
	}
	update, err := SetServerDisabled(configPath, serverName, disabled)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to update server", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write config file", Err: err}
	}
	return nil
}

func (e *Editor) DeleteServer(ctx context.Context, serverName string) error {
	_ = ctx
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return &EditorError{Kind: EditorErrorInvalidRequest, Message: "Server name is required"}
	}

	configPath, err := e.configPath(false)
	if err != nil {
		return err
	}
	update, err := DeleteServer(configPath, serverName)
	if err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to delete server", Err: err}
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return &EditorError{Kind: EditorErrorInvalidConfig, Message: "Failed to write config file", Err: err}
	}
	return nil
}

func normalizeEditorServerSpec(spec domain.ServerSpec) (domain.ServerSpec, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return domain.ServerSpec{}, fmt.Errorf("server name is required")
	}

	transport := domain.NormalizeTransport(spec.Transport)
	spec.Name = name
	spec.Transport = transport
	spec.Env = normalizeImportEnv(spec.Env)
	spec.Cwd = strings.TrimSpace(spec.Cwd)
	spec.Tags = normalizeTags(spec.Tags)

	if spec.Strategy == "" {
		spec.Strategy = domain.DefaultStrategy
	}
	if spec.MaxConcurrent == 0 {
		spec.MaxConcurrent = domain.DefaultMaxConcurrent
	}
	if spec.DrainTimeoutSeconds == 0 {
		spec.DrainTimeoutSeconds = domain.DefaultDrainTimeoutSeconds
	}
	if spec.ActivationMode == "" {
		spec.ActivationMode = domain.DefaultActivationMode
	}
	if spec.ProtocolVersion == "" {
		if transport == domain.TransportStreamableHTTP {
			spec.ProtocolVersion = domain.DefaultStreamableHTTPProtocolVersion
		} else {
			spec.ProtocolVersion = domain.DefaultProtocolVersion
		}
	}
	if spec.Strategy == domain.StrategyStateful && spec.SessionTTLSeconds == 0 {
		spec.SessionTTLSeconds = domain.DefaultSessionTTLSeconds
	}

	switch transport {
	case domain.TransportStdio:
		cmd := make([]string, 0, len(spec.Cmd))
		for _, entry := range spec.Cmd {
			trimmed := strings.TrimSpace(entry)
			if trimmed == "" {
				continue
			}
			cmd = append(cmd, trimmed)
		}
		spec.Cmd = cmd
		spec.HTTP = nil
	case domain.TransportStreamableHTTP:
		spec.Cmd = nil
		spec.Env = nil
		spec.Cwd = ""
		if spec.HTTP == nil {
			spec.HTTP = &domain.StreamableHTTPConfig{}
		}
		spec.HTTP.Endpoint = strings.TrimSpace(spec.HTTP.Endpoint)
		if spec.HTTP.Headers != nil {
			headers := make(map[string]string, len(spec.HTTP.Headers))
			for key, value := range spec.HTTP.Headers {
				trimmed := strings.TrimSpace(key)
				if trimmed == "" {
					continue
				}
				headers[trimmed] = strings.TrimSpace(value)
			}
			if len(headers) == 0 {
				headers = nil
			}
			spec.HTTP.Headers = headers
		}
		if spec.HTTP.MaxRetries == 0 {
			spec.HTTP.MaxRetries = domain.DefaultStreamableHTTPMaxRetries
		}
	default:
		return domain.ServerSpec{}, fmt.Errorf("transport must be stdio or streamable_http")
	}

	return spec, nil
}

func NormalizeImportRequest(req ImportRequest) (ImportRequest, error) {
	if len(req.Servers) == 0 {
		return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: "At least one server is required"}
	}

	servers := make([]domain.ServerSpec, 0, len(req.Servers))
	seenServers := make(map[string]struct{}, len(req.Servers))
	for index, server := range req.Servers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: "Server name is required"}
		}
		if _, exists := seenServers[name]; exists {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: fmt.Sprintf("Duplicate server name %q", name)}
		}
		seenServers[name] = struct{}{}

		spec, err := normalizeImportServerSpec(name, server)
		if err != nil {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: err.Error()}
		}
		if errs := validateServerSpec(spec, index); len(errs) > 0 {
			return ImportRequest{}, &EditorError{Kind: EditorErrorInvalidRequest, Message: strings.Join(errs, "; ")}
		}
		servers = append(servers, spec)
	}

	return ImportRequest{Servers: servers}, nil
}

func normalizeImportServerSpec(name string, server domain.ServerSpec) (domain.ServerSpec, error) {
	transport := domain.NormalizeTransport(server.Transport)
	switch transport {
	case domain.TransportStdio:
		if len(server.Cmd) == 0 {
			return domain.ServerSpec{}, fmt.Errorf("server %q: cmd is required", name)
		}
		for _, cmd := range server.Cmd {
			if strings.TrimSpace(cmd) == "" {
				return domain.ServerSpec{}, fmt.Errorf("server %q: cmd contains empty value", name)
			}
		}
	case domain.TransportStreamableHTTP:
		if server.HTTP == nil || strings.TrimSpace(server.HTTP.Endpoint) == "" {
			return domain.ServerSpec{}, fmt.Errorf("server %q: http.endpoint is required", name)
		}
		if len(server.Cmd) > 0 {
			return domain.ServerSpec{}, fmt.Errorf("server %q: cmd must be empty for streamable_http transport", name)
		}
	default:
		return domain.ServerSpec{}, fmt.Errorf("server %q: transport must be stdio or streamable_http", name)
	}

	spec := domain.ServerSpec{
		Name:                name,
		Transport:           transport,
		Cmd:                 append([]string{}, server.Cmd...),
		Env:                 normalizeImportEnv(server.Env),
		Cwd:                 strings.TrimSpace(server.Cwd),
		Tags:                normalizeTags(server.Tags),
		IdleSeconds:         60,
		MaxConcurrent:       domain.DefaultMaxConcurrent,
		Strategy:            domain.DefaultStrategy,
		MinReady:            0,
		DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
		ProtocolVersion:     strings.TrimSpace(server.ProtocolVersion),
		HTTP:                server.HTTP,
	}
	if spec.ProtocolVersion == "" {
		if transport == domain.TransportStreamableHTTP {
			spec.ProtocolVersion = domain.DefaultStreamableHTTPProtocolVersion
		} else {
			spec.ProtocolVersion = domain.DefaultProtocolVersion
		}
	}
	if transport == domain.TransportStreamableHTTP && spec.HTTP != nil {
		spec.HTTP.Endpoint = strings.TrimSpace(spec.HTTP.Endpoint)
		if spec.HTTP.MaxRetries == 0 {
			spec.HTTP.MaxRetries = domain.DefaultStreamableHTTPMaxRetries
		}
	}

	return spec, nil
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

func (e *Editor) configPath(allowCreate bool) (string, error) {
	if e.path == "" {
		return "", &EditorError{Kind: EditorErrorInvalidConfig, Message: "Config path is required"}
	}
	info, err := os.Stat(e.path)
	if err != nil {
		if os.IsNotExist(err) && allowCreate {
			return e.path, nil
		}
		return "", &EditorError{Kind: EditorErrorInvalidConfig, Message: "Config path is not available", Err: err}
	}
	if info.IsDir() {
		return "", &EditorError{Kind: EditorErrorInvalidConfig, Message: fmt.Sprintf("Config path must be a file: %s", e.path)}
	}
	return e.path, nil
}

func isWritableFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return false
		}
		file.Close()
		return true
	}
	if !os.IsNotExist(err) {
		return false
	}
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".write_test_*")
	if err != nil {
		return false
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return false
	}
	return os.Remove(name) == nil
}
