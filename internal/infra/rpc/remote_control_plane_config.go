package rpc

import (
	"context"
	"encoding/json"
	"strings"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (r *RemoteControlPlane) GetConfigMode(ctx context.Context) (ConfigMode, error) {
	resp, err := withCaller(ctx, r, "get config mode", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.GetConfigModeResponse, error) {
		return client.GetConfigMode(ctx, &controlv1.GetConfigModeRequest{})
	})
	if err != nil {
		return ConfigMode{}, err
	}
	return ConfigMode{
		Mode:       resp.GetMode(),
		Path:       resp.GetPath(),
		IsWritable: resp.GetIsWritable(),
	}, nil
}

func (r *RemoteControlPlane) GetRuntimeConfig(ctx context.Context) (domain.RuntimeConfig, error) {
	resp, err := withCaller(ctx, r, "get runtime config", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.GetRuntimeConfigResponse, error) {
		return client.GetRuntimeConfig(ctx, &controlv1.GetRuntimeConfigRequest{})
	})
	if err != nil {
		return domain.RuntimeConfig{}, err
	}
	raw := resp.GetRuntimeJson()
	if len(raw) == 0 {
		return domain.RuntimeConfig{}, domain.E(domain.CodeInternal, "get runtime config", "empty response", nil)
	}
	var payload RuntimeConfigPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.RuntimeConfig{}, domain.E(domain.CodeInternal, "get runtime config", "invalid response", err)
	}
	return runtimeConfigFromPayload(payload), nil
}

func (r *RemoteControlPlane) UpdateRuntimeConfig(ctx context.Context, payload RuntimeConfigUpdatePayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return domain.E(domain.CodeInternal, "update runtime config", "encode payload", err)
	}
	_, err = withCaller(ctx, r, "update runtime config", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.UpdateRuntimeConfigResponse, error) {
		return client.UpdateRuntimeConfig(ctx, &controlv1.UpdateRuntimeConfigRequest{RuntimeJson: raw})
	})
	return err
}

func (r *RemoteControlPlane) ReloadConfig(ctx context.Context) error {
	_, err := withCaller(ctx, r, "reload config", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.ReloadConfigResponse, error) {
		return client.ReloadConfig(ctx, &controlv1.ReloadConfigRequest{})
	})
	return err
}

func (r *RemoteControlPlane) CreateServer(ctx context.Context, spec domain.ServerSpec) error {
	raw, err := json.Marshal(spec)
	if err != nil {
		return domain.E(domain.CodeInternal, "create server", "encode payload", err)
	}
	_, err = withCaller(ctx, r, "create server", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.CreateServerResponse, error) {
		return client.CreateServer(ctx, &controlv1.CreateServerRequest{ServerJson: raw})
	})
	return err
}

func (r *RemoteControlPlane) UpdateServer(ctx context.Context, spec domain.ServerSpec) error {
	raw, err := json.Marshal(spec)
	if err != nil {
		return domain.E(domain.CodeInternal, "update server", "encode payload", err)
	}
	_, err = withCaller(ctx, r, "update server", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.UpdateServerResponse, error) {
		return client.UpdateServer(ctx, &controlv1.UpdateServerRequest{ServerJson: raw})
	})
	return err
}

func (r *RemoteControlPlane) SetServerDisabled(ctx context.Context, server string, disabled bool) error {
	name := strings.TrimSpace(server)
	if name == "" {
		return domain.E(domain.CodeInvalidArgument, "set server disabled", "server is required", nil)
	}
	_, err := withCaller(ctx, r, "set server disabled", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.SetServerDisabledResponse, error) {
		return client.SetServerDisabled(ctx, &controlv1.SetServerDisabledRequest{Server: name, Disabled: disabled})
	})
	return err
}

func (r *RemoteControlPlane) DeleteServer(ctx context.Context, server string) error {
	name := strings.TrimSpace(server)
	if name == "" {
		return domain.E(domain.CodeInvalidArgument, "delete server", "server is required", nil)
	}
	_, err := withCaller(ctx, r, "delete server", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.DeleteServerResponse, error) {
		return client.DeleteServer(ctx, &controlv1.DeleteServerRequest{Server: name})
	})
	return err
}

func (r *RemoteControlPlane) ImportServers(ctx context.Context, specs []domain.ServerSpec) error {
	if len(specs) == 0 {
		return domain.E(domain.CodeInvalidArgument, "import servers", "servers are required", nil)
	}
	raw, err := json.Marshal(specs)
	if err != nil {
		return domain.E(domain.CodeInternal, "import servers", "encode payload", err)
	}
	_, err = withCaller(ctx, r, "import servers", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.ImportServersResponse, error) {
		return client.ImportServers(ctx, &controlv1.ImportServersRequest{ServersJson: raw})
	})
	return err
}

func (r *RemoteControlPlane) GetSubAgentConfig(ctx context.Context) (domain.SubAgentConfig, error) {
	resp, err := withCaller(ctx, r, "get subagent config", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.GetSubAgentConfigResponse, error) {
		return client.GetSubAgentConfig(ctx, &controlv1.GetSubAgentConfigRequest{})
	})
	if err != nil {
		return domain.SubAgentConfig{}, err
	}
	raw := resp.GetConfigJson()
	if len(raw) == 0 {
		return domain.SubAgentConfig{}, domain.E(domain.CodeInternal, "get subagent config", "empty response", nil)
	}
	var payload SubAgentConfigPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.SubAgentConfig{}, domain.E(domain.CodeInternal, "get subagent config", "invalid response", err)
	}
	return subAgentConfigFromPayload(payload), nil
}

func (r *RemoteControlPlane) UpdateSubAgentConfig(ctx context.Context, payload SubAgentUpdatePayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return domain.E(domain.CodeInternal, "update subagent config", "encode payload", err)
	}
	_, err = withCaller(ctx, r, "update subagent config", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.UpdateSubAgentConfigResponse, error) {
		return client.UpdateSubAgentConfig(ctx, &controlv1.UpdateSubAgentConfigRequest{ConfigJson: raw})
	})
	return err
}

func (r *RemoteControlPlane) GetPluginStatus(ctx context.Context) ([]PluginStatusPayload, error) {
	resp, err := withCaller(ctx, r, "get plugin status", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.GetPluginStatusResponse, error) {
		return client.GetPluginStatus(ctx, &controlv1.GetPluginStatusRequest{})
	})
	if err != nil {
		return nil, err
	}
	statuses := make([]PluginStatusPayload, 0, len(resp.GetStatuses()))
	for _, st := range resp.GetStatuses() {
		if st == nil {
			continue
		}
		statuses = append(statuses, PluginStatusPayload{
			Name:    st.GetName(),
			Running: st.GetRunning(),
			Error:   st.GetError(),
		})
	}
	return statuses, nil
}

func (r *RemoteControlPlane) CreatePlugin(ctx context.Context, spec domain.PluginSpec) error {
	raw, err := json.Marshal(spec)
	if err != nil {
		return domain.E(domain.CodeInternal, "create plugin", "encode payload", err)
	}
	_, err = withCaller(ctx, r, "create plugin", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.CreatePluginResponse, error) {
		return client.CreatePlugin(ctx, &controlv1.CreatePluginRequest{PluginJson: raw})
	})
	return err
}

func (r *RemoteControlPlane) UpdatePlugin(ctx context.Context, spec domain.PluginSpec) error {
	raw, err := json.Marshal(spec)
	if err != nil {
		return domain.E(domain.CodeInternal, "update plugin", "encode payload", err)
	}
	_, err = withCaller(ctx, r, "update plugin", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.UpdatePluginResponse, error) {
		return client.UpdatePlugin(ctx, &controlv1.UpdatePluginRequest{PluginJson: raw})
	})
	return err
}

func (r *RemoteControlPlane) DeletePlugin(ctx context.Context, name string) error {
	plugin := strings.TrimSpace(name)
	if plugin == "" {
		return domain.E(domain.CodeInvalidArgument, "delete plugin", "name is required", nil)
	}
	_, err := withCaller(ctx, r, "delete plugin", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.DeletePluginResponse, error) {
		return client.DeletePlugin(ctx, &controlv1.DeletePluginRequest{Name: plugin})
	})
	return err
}

func (r *RemoteControlPlane) TogglePlugin(ctx context.Context, name string, enabled bool) error {
	plugin := strings.TrimSpace(name)
	if plugin == "" {
		return domain.E(domain.CodeInvalidArgument, "toggle plugin", "name is required", nil)
	}
	_, err := withCaller(ctx, r, "toggle plugin", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.TogglePluginResponse, error) {
		return client.TogglePlugin(ctx, &controlv1.TogglePluginRequest{Name: plugin, Enabled: enabled})
	})
	return err
}
