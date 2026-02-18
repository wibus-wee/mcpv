package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	catalogeditor "mcpv/internal/infra/catalog/editor"
	catalogloader "mcpv/internal/infra/catalog/loader"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) GetConfigMode(ctx context.Context, _ *controlv1.GetConfigModeRequest) (*controlv1.GetConfigModeResponse, error) {
	path := strings.TrimSpace(s.control.ConfigPath())
	if path == "" {
		return &controlv1.GetConfigModeResponse{Mode: "unknown"}, nil
	}

	editor := catalogeditor.NewEditor(path, s.logger)
	info, err := editor.Inspect(ctx)
	if err != nil {
		return &controlv1.GetConfigModeResponse{Mode: "unknown", Path: path}, nil
	}

	return &controlv1.GetConfigModeResponse{
		Mode:       "file",
		Path:       info.Path,
		IsWritable: info.IsWritable,
	}, nil
}

func (s *ControlService) GetRuntimeConfig(ctx context.Context, _ *controlv1.GetRuntimeConfigRequest) (*controlv1.GetRuntimeConfigResponse, error) {
	path, err := s.configPath()
	if err != nil {
		return nil, err
	}

	loader := catalogloader.NewLoader(s.logger)
	runtimeCfg, err := loader.LoadRuntimeConfig(ctx, path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "get runtime config: %v", err)
	}

	raw, err := json.Marshal(runtimeConfigToPayload(runtimeCfg))
	if err != nil {
		return nil, status.Error(codes.Internal, "get runtime config: encode failed")
	}

	return &controlv1.GetRuntimeConfigResponse{RuntimeJson: raw}, nil
}

func (s *ControlService) UpdateRuntimeConfig(ctx context.Context, req *controlv1.UpdateRuntimeConfigRequest) (*controlv1.UpdateRuntimeConfigResponse, error) {
	raw := req.GetRuntimeJson()
	if len(raw) == 0 {
		return nil, status.Error(codes.InvalidArgument, "runtime_json is required")
	}

	var payload RuntimeConfigUpdatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, status.Error(codes.InvalidArgument, "runtime_json is invalid")
	}

	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}

	if err := editor.UpdateRuntimeConfig(ctx, runtimeUpdateFromPayload(payload)); err != nil {
		return nil, statusFromCatalogEditorError("update runtime config", err)
	}

	return &controlv1.UpdateRuntimeConfigResponse{}, nil
}

func (s *ControlService) ReloadConfig(ctx context.Context, _ *controlv1.ReloadConfigRequest) (*controlv1.ReloadConfigResponse, error) {
	if err := s.control.ReloadConfig(ctx); err != nil {
		return nil, statusFromError("reload config", err)
	}
	return &controlv1.ReloadConfigResponse{}, nil
}

func (s *ControlService) CreateServer(ctx context.Context, req *controlv1.CreateServerRequest) (*controlv1.CreateServerResponse, error) {
	spec, err := decodeServerSpec(req.GetServerJson())
	if err != nil {
		return nil, err
	}
	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.CreateServer(ctx, spec); err != nil {
		return nil, statusFromCatalogEditorError("create server", err)
	}
	return &controlv1.CreateServerResponse{}, nil
}

func (s *ControlService) UpdateServer(ctx context.Context, req *controlv1.UpdateServerRequest) (*controlv1.UpdateServerResponse, error) {
	spec, err := decodeServerSpec(req.GetServerJson())
	if err != nil {
		return nil, err
	}
	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.UpdateServer(ctx, spec); err != nil {
		return nil, statusFromCatalogEditorError("update server", err)
	}
	return &controlv1.UpdateServerResponse{}, nil
}

func (s *ControlService) SetServerDisabled(ctx context.Context, req *controlv1.SetServerDisabledRequest) (*controlv1.SetServerDisabledResponse, error) {
	server := strings.TrimSpace(req.GetServer())
	if server == "" {
		return nil, status.Error(codes.InvalidArgument, "server is required")
	}
	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.SetServerDisabled(ctx, server, req.GetDisabled()); err != nil {
		return nil, statusFromCatalogEditorError("set server disabled", err)
	}
	return &controlv1.SetServerDisabledResponse{}, nil
}

func (s *ControlService) DeleteServer(ctx context.Context, req *controlv1.DeleteServerRequest) (*controlv1.DeleteServerResponse, error) {
	server := strings.TrimSpace(req.GetServer())
	if server == "" {
		return nil, status.Error(codes.InvalidArgument, "server is required")
	}
	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.DeleteServer(ctx, server); err != nil {
		return nil, statusFromCatalogEditorError("delete server", err)
	}
	return &controlv1.DeleteServerResponse{}, nil
}

func (s *ControlService) ImportServers(ctx context.Context, req *controlv1.ImportServersRequest) (*controlv1.ImportServersResponse, error) {
	raw := req.GetServersJson()
	if len(raw) == 0 {
		return nil, status.Error(codes.InvalidArgument, "servers_json is required")
	}

	var servers []domain.ServerSpec
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil, status.Error(codes.InvalidArgument, "servers_json is invalid")
	}

	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.ImportServers(ctx, catalogeditor.ImportRequest{Servers: servers}); err != nil {
		return nil, statusFromCatalogEditorError("import servers", err)
	}
	return &controlv1.ImportServersResponse{}, nil
}

func (s *ControlService) GetSubAgentConfig(_ context.Context, _ *controlv1.GetSubAgentConfigRequest) (*controlv1.GetSubAgentConfigResponse, error) {
	catalog := s.control.GetCatalog()
	raw, err := json.Marshal(subAgentConfigToPayload(catalog.Runtime.SubAgent))
	if err != nil {
		return nil, status.Error(codes.Internal, "get subagent config: encode failed")
	}
	return &controlv1.GetSubAgentConfigResponse{ConfigJson: raw}, nil
}

func (s *ControlService) UpdateSubAgentConfig(ctx context.Context, req *controlv1.UpdateSubAgentConfigRequest) (*controlv1.UpdateSubAgentConfigResponse, error) {
	raw := req.GetConfigJson()
	if len(raw) == 0 {
		return nil, status.Error(codes.InvalidArgument, "config_json is required")
	}

	var payload SubAgentUpdatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, status.Error(codes.InvalidArgument, "config_json is invalid")
	}

	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}

	if err := editor.UpdateSubAgentConfig(ctx, subAgentUpdateFromPayload(payload)); err != nil {
		return nil, statusFromCatalogEditorError("update subagent config", err)
	}
	return &controlv1.UpdateSubAgentConfigResponse{}, nil
}

func (s *ControlService) GetPluginStatus(_ context.Context, _ *controlv1.GetPluginStatusRequest) (*controlv1.GetPluginStatusResponse, error) {
	catalog := s.control.GetCatalog()
	statuses := s.control.PluginStatus(catalog.Plugins)
	statusMap := make(map[string]pluginStatus, len(statuses))
	for _, st := range statuses {
		statusMap[st.Name] = pluginStatus{Running: st.Running, Error: st.Error}
	}

	out := make([]*controlv1.PluginStatus, 0, len(catalog.Plugins))
	for _, plugin := range catalog.Plugins {
		entry := pluginStatus{}
		if existing, ok := statusMap[plugin.Name]; ok {
			entry = existing
		} else if !plugin.Disabled {
			entry.Error = "Plugin failed to start or is not running"
		}
		out = append(out, &controlv1.PluginStatus{
			Name:    plugin.Name,
			Running: entry.Running,
			Error:   entry.Error,
		})
	}

	return &controlv1.GetPluginStatusResponse{Statuses: out}, nil
}

func (s *ControlService) CreatePlugin(ctx context.Context, req *controlv1.CreatePluginRequest) (*controlv1.CreatePluginResponse, error) {
	spec, err := decodePluginSpec(req.GetPluginJson())
	if err != nil {
		return nil, err
	}
	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.CreatePlugin(ctx, spec); err != nil {
		return nil, statusFromCatalogEditorError("create plugin", err)
	}
	return &controlv1.CreatePluginResponse{}, nil
}

func (s *ControlService) UpdatePlugin(ctx context.Context, req *controlv1.UpdatePluginRequest) (*controlv1.UpdatePluginResponse, error) {
	spec, err := decodePluginSpec(req.GetPluginJson())
	if err != nil {
		return nil, err
	}
	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.UpdatePlugin(ctx, spec); err != nil {
		return nil, statusFromCatalogEditorError("update plugin", err)
	}
	return &controlv1.UpdatePluginResponse{}, nil
}

func (s *ControlService) DeletePlugin(ctx context.Context, req *controlv1.DeletePluginRequest) (*controlv1.DeletePluginResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.DeletePlugin(ctx, name); err != nil {
		return nil, statusFromCatalogEditorError("delete plugin", err)
	}
	return &controlv1.DeletePluginResponse{}, nil
}

func (s *ControlService) TogglePlugin(ctx context.Context, req *controlv1.TogglePluginRequest) (*controlv1.TogglePluginResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	editor, err := s.catalogEditor()
	if err != nil {
		return nil, err
	}
	if err := editor.SetPluginDisabled(ctx, name, !req.GetEnabled()); err != nil {
		return nil, statusFromCatalogEditorError("toggle plugin", err)
	}
	return &controlv1.TogglePluginResponse{}, nil
}

type pluginStatus struct {
	Running bool
	Error   string
}

func (s *ControlService) configPath() (string, error) {
	path := strings.TrimSpace(s.control.ConfigPath())
	if path == "" {
		return "", status.Error(codes.FailedPrecondition, "configuration path is not available")
	}
	return path, nil
}

func (s *ControlService) catalogEditor() (*catalogeditor.Editor, error) {
	path, err := s.configPath()
	if err != nil {
		return nil, err
	}
	return catalogeditor.NewEditor(path, s.logger), nil
}

func decodeServerSpec(raw []byte) (domain.ServerSpec, error) {
	if len(raw) == 0 {
		return domain.ServerSpec{}, status.Error(codes.InvalidArgument, "server_json is required")
	}
	var spec domain.ServerSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return domain.ServerSpec{}, status.Error(codes.InvalidArgument, "server_json is invalid")
	}
	return spec, nil
}

func decodePluginSpec(raw []byte) (domain.PluginSpec, error) {
	if len(raw) == 0 {
		return domain.PluginSpec{}, status.Error(codes.InvalidArgument, "plugin_json is required")
	}
	var spec domain.PluginSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return domain.PluginSpec{}, status.Error(codes.InvalidArgument, "plugin_json is invalid")
	}
	return spec, nil
}

func statusFromCatalogEditorError(op string, err error) error {
	if err == nil {
		return nil
	}
	var editorErr *catalogeditor.Error
	if errors.As(err, &editorErr) {
		msg := editorErr.Message
		if editorErr.Err != nil {
			msg = fmt.Sprintf("%s: %v", msg, editorErr.Err)
		}
		code := codes.Internal
		switch editorErr.Kind {
		case catalogeditor.ErrorInvalidRequest:
			code = codes.InvalidArgument
		case catalogeditor.ErrorInvalidConfig:
			code = codes.FailedPrecondition
		}
		if strings.TrimSpace(op) != "" {
			msg = fmt.Sprintf("%s: %s", op, msg)
		}
		return status.Error(code, msg)
	}
	return statusFromError(op, err)
}
