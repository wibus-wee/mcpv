package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

type RemoteControlPlaneConfig struct {
	ClientConfig ClientConfig
	Caller       string
	Tags         []string
	Server       string
}

type RemoteControlPlane struct {
	client *Client
	caller string
	tags   []string
	server string
	pid    int64
	logger *zap.Logger

	mu         sync.Mutex
	registered bool
}

func NewRemoteControlPlane(ctx context.Context, cfg RemoteControlPlaneConfig, logger *zap.Logger) (*RemoteControlPlane, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	client, err := Dial(ctx, cfg.ClientConfig)
	if err != nil {
		return nil, err
	}
	caller := strings.TrimSpace(cfg.Caller)
	if caller == "" {
		caller = domain.InternalUIClientName
	}
	r := &RemoteControlPlane{
		client: client,
		caller: caller,
		tags:   normalizeTags(cfg.Tags),
		server: strings.TrimSpace(cfg.Server),
		pid:    int64(os.Getpid()),
		logger: logger.Named("remote-control-plane"),
	}
	if err := r.ensureRegistered(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return r, nil
}

func (r *RemoteControlPlane) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	registered := r.registered
	r.registered = false
	r.mu.Unlock()

	if registered {
		unregisterCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if client := r.control(); client != nil {
			_, _ = client.UnregisterCaller(unregisterCtx, &controlv1.UnregisterCallerRequest{
				Caller: r.caller,
			})
		}
	}
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}

func (r *RemoteControlPlane) Info(ctx context.Context) (domain.ControlPlaneInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := r.control()
	if client == nil {
		return domain.ControlPlaneInfo{}, domain.E(domain.CodeUnavailable, "get info", "rpc client unavailable", nil)
	}
	resp, err := client.GetInfo(ctx, &controlv1.GetInfoRequest{})
	if err != nil {
		return domain.ControlPlaneInfo{}, mapRPCError("get info", err)
	}
	return domain.ControlPlaneInfo{
		Name:    resp.GetName(),
		Version: resp.GetVersion(),
		Build:   resp.GetBuild(),
	}, nil
}

func (r *RemoteControlPlane) RegisterClient(ctx context.Context, client string, pid int, tags []string, server string) (domain.ClientRegistration, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(client) == "" {
		return domain.ClientRegistration{}, domain.E(domain.CodeInvalidArgument, "register client", "client is required", nil)
	}
	if pid <= 0 {
		return domain.ClientRegistration{}, domain.E(domain.CodeInvalidArgument, "register client", "pid must be > 0", nil)
	}
	resp, err := r.control().RegisterCaller(ctx, &controlv1.RegisterCallerRequest{
		Caller: strings.TrimSpace(client),
		Pid:    int64(pid),
		Tags:   normalizeTags(tags),
		Server: strings.TrimSpace(server),
	})
	if err != nil {
		return domain.ClientRegistration{}, mapRPCError("register client", err)
	}
	return domain.ClientRegistration{
		Client: resp.GetProfile(),
		Tags:   normalizeTags(tags),
	}, nil
}

func (r *RemoteControlPlane) UnregisterClient(ctx context.Context, client string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(client) == "" {
		return domain.E(domain.CodeInvalidArgument, "unregister client", "client is required", nil)
	}
	_, err := r.control().UnregisterCaller(ctx, &controlv1.UnregisterCallerRequest{Caller: strings.TrimSpace(client)})
	return mapRPCError("unregister client", err)
}

func (r *RemoteControlPlane) ListActiveClients(ctx context.Context) ([]domain.ActiveClient, error) {
	resp, err := withCaller(ctx, r, "list active clients", func(client controlv1.ControlPlaneServiceClient, _ string) (*controlv1.ListActiveClientsResponse, error) {
		return client.ListActiveClients(ctx, &controlv1.ListActiveClientsRequest{})
	})
	if err != nil {
		return nil, err
	}
	snapshot, err := fromProtoActiveClientsSnapshot(resp.GetSnapshot())
	if err != nil {
		return nil, err
	}
	return snapshot.Clients, nil
}

func (r *RemoteControlPlane) WatchActiveClients(ctx context.Context) (<-chan domain.ActiveClientSnapshot, error) {
	stream, err := withCallerStream(ctx, r, "watch active clients", func(client controlv1.ControlPlaneServiceClient, _ string) (controlv1.ControlPlaneService_WatchActiveClientsClient, error) {
		return client.WatchActiveClients(ctx, &controlv1.WatchActiveClientsRequest{})
	})
	if err != nil {
		return nil, err
	}
	out := make(chan domain.ActiveClientSnapshot)
	go func() {
		defer close(out)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if shouldSilenceStreamError(err) {
					return
				}
				r.logger.Debug("watch active clients stream ended", zap.Error(err))
				return
			}
			snapshot, err := fromProtoActiveClientsSnapshot(resp)
			if err != nil {
				r.logger.Warn("watch active clients decode failed", zap.Error(err))
				continue
			}
			out <- snapshot
		}
	}()
	return out, nil
}

func (r *RemoteControlPlane) ListTools(ctx context.Context, _ string) (domain.ToolSnapshot, error) {
	resp, err := withCaller(ctx, r, "list tools", func(client controlv1.ControlPlaneServiceClient, caller string) (*controlv1.ListToolsResponse, error) {
		return client.ListTools(ctx, &controlv1.ListToolsRequest{Caller: caller})
	})
	if err != nil {
		return domain.ToolSnapshot{}, err
	}
	return fromProtoToolSnapshot(resp.GetSnapshot())
}

func (r *RemoteControlPlane) ListToolCatalog(ctx context.Context) (domain.ToolCatalogSnapshot, error) {
	snapshot, err := r.ListTools(ctx, "")
	if err != nil {
		return domain.ToolCatalogSnapshot{}, err
	}
	entries := make([]domain.ToolCatalogEntry, 0, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		entries = append(entries, domain.ToolCatalogEntry{
			Definition: tool,
			Source:     domain.ToolSourceLive,
		})
	}
	return domain.ToolCatalogSnapshot{
		ETag:  snapshot.ETag,
		Tools: entries,
	}, nil
}

func (r *RemoteControlPlane) WatchTools(ctx context.Context, _ string) (<-chan domain.ToolSnapshot, error) {
	stream, err := withCallerStream(ctx, r, "watch tools", func(client controlv1.ControlPlaneServiceClient, caller string) (controlv1.ControlPlaneService_WatchToolsClient, error) {
		return client.WatchTools(ctx, &controlv1.WatchToolsRequest{Caller: caller})
	})
	if err != nil {
		return nil, err
	}
	out := make(chan domain.ToolSnapshot)
	go func() {
		defer close(out)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if shouldSilenceStreamError(err) {
					return
				}
				r.logger.Debug("watch tools stream ended", zap.Error(err))
				return
			}
			snapshot, err := fromProtoToolSnapshot(resp)
			if err != nil {
				r.logger.Warn("watch tools decode failed", zap.Error(err))
				continue
			}
			out <- snapshot
		}
	}()
	return out, nil
}

func (r *RemoteControlPlane) CallTool(ctx context.Context, _ string, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	resp, err := withCaller(ctx, r, "call tool", func(client controlv1.ControlPlaneServiceClient, caller string) (*controlv1.CallToolResponse, error) {
		return client.CallTool(ctx, &controlv1.CallToolRequest{
			Caller:        caller,
			Name:          name,
			ArgumentsJson: args,
			RoutingKey:    routingKey,
		})
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.GetResultJson()) == 0 {
		return nil, domain.E(domain.CodeInternal, "call tool", "empty response", nil)
	}
	return resp.GetResultJson(), nil
}

func (r *RemoteControlPlane) CallToolAll(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	return r.CallTool(ctx, "", name, args, routingKey)
}

func (r *RemoteControlPlane) ListResources(ctx context.Context, _ string, cursor string) (domain.ResourcePage, error) {
	resp, err := withCaller(ctx, r, "list resources", func(client controlv1.ControlPlaneServiceClient, caller string) (*controlv1.ListResourcesResponse, error) {
		return client.ListResources(ctx, &controlv1.ListResourcesRequest{
			Caller: caller,
			Cursor: cursor,
		})
	})
	if err != nil {
		return domain.ResourcePage{}, err
	}
	snapshot, err := fromProtoResourceSnapshot(resp.GetSnapshot())
	if err != nil {
		return domain.ResourcePage{}, err
	}
	return domain.ResourcePage{
		Snapshot:   snapshot,
		NextCursor: resp.GetNextCursor(),
	}, nil
}

func (r *RemoteControlPlane) ListResourcesAll(ctx context.Context, cursor string) (domain.ResourcePage, error) {
	return r.ListResources(ctx, "", cursor)
}

func (r *RemoteControlPlane) WatchResources(ctx context.Context, _ string) (<-chan domain.ResourceSnapshot, error) {
	stream, err := withCallerStream(ctx, r, "watch resources", func(client controlv1.ControlPlaneServiceClient, caller string) (controlv1.ControlPlaneService_WatchResourcesClient, error) {
		return client.WatchResources(ctx, &controlv1.WatchResourcesRequest{Caller: caller})
	})
	if err != nil {
		return nil, err
	}
	out := make(chan domain.ResourceSnapshot)
	go func() {
		defer close(out)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if shouldSilenceStreamError(err) {
					return
				}
				r.logger.Debug("watch resources stream ended", zap.Error(err))
				return
			}
			snapshot, err := fromProtoResourceSnapshot(resp)
			if err != nil {
				r.logger.Warn("watch resources decode failed", zap.Error(err))
				continue
			}
			out <- snapshot
		}
	}()
	return out, nil
}

func (r *RemoteControlPlane) ReadResource(ctx context.Context, _ string, uri string) (json.RawMessage, error) {
	resp, err := withCaller(ctx, r, "read resource", func(client controlv1.ControlPlaneServiceClient, caller string) (*controlv1.ReadResourceResponse, error) {
		return client.ReadResource(ctx, &controlv1.ReadResourceRequest{
			Caller: caller,
			Uri:    uri,
		})
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.GetResultJson()) == 0 {
		return nil, domain.E(domain.CodeInternal, "read resource", "empty response", nil)
	}
	return resp.GetResultJson(), nil
}

func (r *RemoteControlPlane) ReadResourceAll(ctx context.Context, uri string) (json.RawMessage, error) {
	return r.ReadResource(ctx, "", uri)
}

func (r *RemoteControlPlane) ListPrompts(ctx context.Context, _ string, cursor string) (domain.PromptPage, error) {
	resp, err := withCaller(ctx, r, "list prompts", func(client controlv1.ControlPlaneServiceClient, caller string) (*controlv1.ListPromptsResponse, error) {
		return client.ListPrompts(ctx, &controlv1.ListPromptsRequest{
			Caller: caller,
			Cursor: cursor,
		})
	})
	if err != nil {
		return domain.PromptPage{}, err
	}
	snapshot, err := fromProtoPromptSnapshot(resp.GetSnapshot())
	if err != nil {
		return domain.PromptPage{}, err
	}
	return domain.PromptPage{
		Snapshot:   snapshot,
		NextCursor: resp.GetNextCursor(),
	}, nil
}

func (r *RemoteControlPlane) ListPromptsAll(ctx context.Context, cursor string) (domain.PromptPage, error) {
	return r.ListPrompts(ctx, "", cursor)
}

func (r *RemoteControlPlane) WatchPrompts(ctx context.Context, _ string) (<-chan domain.PromptSnapshot, error) {
	stream, err := withCallerStream(ctx, r, "watch prompts", func(client controlv1.ControlPlaneServiceClient, caller string) (controlv1.ControlPlaneService_WatchPromptsClient, error) {
		return client.WatchPrompts(ctx, &controlv1.WatchPromptsRequest{Caller: caller})
	})
	if err != nil {
		return nil, err
	}
	out := make(chan domain.PromptSnapshot)
	go func() {
		defer close(out)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if shouldSilenceStreamError(err) {
					return
				}
				r.logger.Debug("watch prompts stream ended", zap.Error(err))
				return
			}
			snapshot, err := fromProtoPromptSnapshot(resp)
			if err != nil {
				r.logger.Warn("watch prompts decode failed", zap.Error(err))
				continue
			}
			out <- snapshot
		}
	}()
	return out, nil
}

func (r *RemoteControlPlane) GetPrompt(ctx context.Context, _ string, name string, args json.RawMessage) (json.RawMessage, error) {
	resp, err := withCaller(ctx, r, "get prompt", func(client controlv1.ControlPlaneServiceClient, caller string) (*controlv1.GetPromptResponse, error) {
		return client.GetPrompt(ctx, &controlv1.GetPromptRequest{
			Caller:        caller,
			Name:          name,
			ArgumentsJson: args,
		})
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.GetResultJson()) == 0 {
		return nil, domain.E(domain.CodeInternal, "get prompt", "empty response", nil)
	}
	return resp.GetResultJson(), nil
}

func (r *RemoteControlPlane) GetPromptAll(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	return r.GetPrompt(ctx, "", name, args)
}

func (r *RemoteControlPlane) StreamLogs(ctx context.Context, _ string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	stream, err := withCallerStream(ctx, r, "stream logs", func(client controlv1.ControlPlaneServiceClient, caller string) (controlv1.ControlPlaneService_StreamLogsClient, error) {
		return client.StreamLogs(ctx, &controlv1.StreamLogsRequest{
			Caller:   caller,
			MinLevel: toProtoLogLevel(minLevel),
		})
	})
	if err != nil {
		return nil, err
	}
	out := make(chan domain.LogEntry)
	go func() {
		defer close(out)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if shouldSilenceStreamError(err) {
					return
				}
				r.logger.Debug("stream logs ended", zap.Error(err))
				return
			}
			entry, err := fromProtoLogEntry(resp)
			if err != nil {
				r.logger.Warn("stream logs decode failed", zap.Error(err))
				continue
			}
			out <- entry
		}
	}()
	return out, nil
}

func (r *RemoteControlPlane) StreamLogsAllServers(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return r.StreamLogs(ctx, "", minLevel)
}

func (r *RemoteControlPlane) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	snapshot, err := r.fetchRuntimeSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	pools := make([]domain.PoolInfo, 0, len(snapshot.Statuses))
	for _, status := range snapshot.Statuses {
		pools = append(pools, poolInfoFromRuntime(status))
	}
	return pools, nil
}

func (r *RemoteControlPlane) GetServerInitStatus(ctx context.Context) ([]domain.ServerInitStatus, error) {
	snapshot, err := r.fetchServerInitSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Statuses, nil
}

func (r *RemoteControlPlane) RetryServerInit(_ context.Context, _ string) error {
	return domain.E(domain.CodeNotImplemented, "retry server init", "not supported over RPC", nil)
}

func (r *RemoteControlPlane) WatchRuntimeStatus(ctx context.Context, _ string) (<-chan domain.RuntimeStatusSnapshot, error) {
	stream, err := withCallerStream(ctx, r, "watch runtime status", func(client controlv1.ControlPlaneServiceClient, caller string) (controlv1.ControlPlaneService_WatchRuntimeStatusClient, error) {
		return client.WatchRuntimeStatus(ctx, &controlv1.WatchRuntimeStatusRequest{Caller: caller})
	})
	if err != nil {
		return nil, err
	}
	out := make(chan domain.RuntimeStatusSnapshot)
	go func() {
		defer close(out)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if shouldSilenceStreamError(err) {
					return
				}
				r.logger.Debug("watch runtime status ended", zap.Error(err))
				return
			}
			snapshot, err := fromProtoRuntimeStatusSnapshot(resp)
			if err != nil {
				r.logger.Warn("watch runtime status decode failed", zap.Error(err))
				continue
			}
			out <- snapshot
		}
	}()
	return out, nil
}

func (r *RemoteControlPlane) WatchRuntimeStatusAllServers(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	return r.WatchRuntimeStatus(ctx, "")
}

func (r *RemoteControlPlane) WatchServerInitStatus(ctx context.Context, _ string) (<-chan domain.ServerInitStatusSnapshot, error) {
	stream, err := withCallerStream(ctx, r, "watch server init status", func(client controlv1.ControlPlaneServiceClient, caller string) (controlv1.ControlPlaneService_WatchServerInitStatusClient, error) {
		return client.WatchServerInitStatus(ctx, &controlv1.WatchServerInitStatusRequest{Caller: caller})
	})
	if err != nil {
		return nil, err
	}
	out := make(chan domain.ServerInitStatusSnapshot)
	go func() {
		defer close(out)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if shouldSilenceStreamError(err) {
					return
				}
				r.logger.Debug("watch server init status ended", zap.Error(err))
				return
			}
			snapshot, err := fromProtoServerInitStatusSnapshot(resp)
			if err != nil {
				r.logger.Warn("watch server init status decode failed", zap.Error(err))
				continue
			}
			out <- snapshot
		}
	}()
	return out, nil
}

func (r *RemoteControlPlane) WatchServerInitStatusAllServers(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	return r.WatchServerInitStatus(ctx, "")
}

func (r *RemoteControlPlane) GetBootstrapProgress(_ context.Context) (domain.BootstrapProgress, error) {
	return domain.BootstrapProgress{State: domain.BootstrapCompleted}, nil
}

func (r *RemoteControlPlane) WatchBootstrapProgress(_ context.Context) (<-chan domain.BootstrapProgress, error) {
	ch := make(chan domain.BootstrapProgress, 1)
	ch <- domain.BootstrapProgress{State: domain.BootstrapCompleted}
	close(ch)
	return ch, nil
}

func (r *RemoteControlPlane) AutomaticMCP(ctx context.Context, _ string, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	resp, err := withCaller(ctx, r, "automatic_mcp", func(client controlv1.ControlPlaneServiceClient, caller string) (*controlv1.AutomaticMCPResponse, error) {
		return client.AutomaticMCP(ctx, &controlv1.AutomaticMCPRequest{
			Caller:       caller,
			Query:        params.Query,
			SessionId:    params.SessionID,
			ForceRefresh: params.ForceRefresh,
		})
	})
	if err != nil {
		return domain.AutomaticMCPResult{}, err
	}
	return fromProtoAutomaticMCPResponse(resp)
}

func (r *RemoteControlPlane) AutomaticEval(ctx context.Context, _ string, params domain.AutomaticEvalParams) (json.RawMessage, error) {
	resp, err := withCaller(ctx, r, "automatic_eval", func(client controlv1.ControlPlaneServiceClient, caller string) (*controlv1.AutomaticEvalResponse, error) {
		return client.AutomaticEval(ctx, &controlv1.AutomaticEvalRequest{
			Caller:        caller,
			ToolName:      params.ToolName,
			ArgumentsJson: params.Arguments,
			RoutingKey:    params.RoutingKey,
		})
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, domain.E(domain.CodeInternal, "automatic_eval", "empty response", nil)
	}
	return resp.GetResultJson(), nil
}

func (r *RemoteControlPlane) IsSubAgentEnabled() bool {
	return r.IsSubAgentEnabledForClient(r.caller)
}

func (r *RemoteControlPlane) IsSubAgentEnabledForClient(client string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	caller := strings.TrimSpace(client)
	if caller == "" {
		caller = r.caller
	}
	resp, err := withCaller(ctx, r, "subagent enabled", func(control controlv1.ControlPlaneServiceClient, _ string) (*controlv1.IsSubAgentEnabledResponse, error) {
		return control.IsSubAgentEnabled(ctx, &controlv1.IsSubAgentEnabledRequest{Caller: caller})
	})
	if err != nil {
		return false
	}
	return resp.GetEnabled()
}

func (r *RemoteControlPlane) GetCatalog() domain.Catalog {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	resp, err := withCaller(ctx, r, "get catalog", func(control controlv1.ControlPlaneServiceClient, _ string) (*controlv1.GetCatalogResponse, error) {
		return control.GetCatalog(ctx, &controlv1.GetCatalogRequest{})
	})
	if err != nil {
		r.logger.Warn("get catalog failed", zap.Error(err))
		return emptyCatalog()
	}

	raw := resp.GetCatalogJson()
	if len(raw) == 0 {
		return emptyCatalog()
	}

	var catalog domain.Catalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		r.logger.Warn("catalog decode failed", zap.Error(err))
		return emptyCatalog()
	}
	if catalog.Specs == nil {
		catalog.Specs = map[string]domain.ServerSpec{}
	}
	return catalog
}

func emptyCatalog() domain.Catalog {
	return domain.Catalog{
		Specs:   map[string]domain.ServerSpec{},
		Plugins: nil,
		Runtime: domain.RuntimeConfig{},
	}
}

func (r *RemoteControlPlane) ensureRegistered(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.Lock()
	if r.registered {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	if r.client == nil {
		return domain.E(domain.CodeUnavailable, "register caller", "rpc client unavailable", nil)
	}

	_, err := r.client.Control().RegisterCaller(ctx, &controlv1.RegisterCallerRequest{
		Caller: r.caller,
		Pid:    r.pid,
		Tags:   r.tags,
		Server: r.server,
	})
	if err != nil {
		return mapRPCError("register caller", err)
	}

	r.mu.Lock()
	r.registered = true
	r.mu.Unlock()
	return nil
}

func (r *RemoteControlPlane) invalidateRegistration() {
	r.mu.Lock()
	r.registered = false
	r.mu.Unlock()
}

func withCaller[T any](ctx context.Context, r *RemoteControlPlane, op string, fn func(controlv1.ControlPlaneServiceClient, string) (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.ensureRegistered(ctx); err != nil {
		return zero, err
	}
	client := r.control()
	if client == nil {
		return zero, domain.E(domain.CodeUnavailable, op, "rpc client unavailable", nil)
	}
	resp, err := fn(client, r.caller)
	if err == nil {
		return resp, nil
	}
	if status.Code(err) == codes.FailedPrecondition {
		r.invalidateRegistration()
		if regErr := r.ensureRegistered(ctx); regErr == nil {
			resp, err = fn(client, r.caller)
		}
	}
	if err != nil {
		return zero, mapRPCError(op, err)
	}
	return resp, nil
}

func withCallerStream[S any](ctx context.Context, r *RemoteControlPlane, op string, fn func(controlv1.ControlPlaneServiceClient, string) (S, error)) (S, error) {
	var zero S
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.ensureRegistered(ctx); err != nil {
		return zero, err
	}
	client := r.control()
	if client == nil {
		return zero, domain.E(domain.CodeUnavailable, op, "rpc client unavailable", nil)
	}
	stream, err := fn(client, r.caller)
	if err == nil {
		return stream, nil
	}
	if status.Code(err) == codes.FailedPrecondition {
		r.invalidateRegistration()
		if regErr := r.ensureRegistered(ctx); regErr == nil {
			stream, err = fn(client, r.caller)
		}
	}
	if err != nil {
		return zero, mapRPCError(op, err)
	}
	return stream, nil
}

func (r *RemoteControlPlane) fetchRuntimeSnapshot(ctx context.Context) (domain.RuntimeStatusSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
	}
	stream, err := withCallerStream(ctx, r, "runtime snapshot", func(client controlv1.ControlPlaneServiceClient, caller string) (controlv1.ControlPlaneService_WatchRuntimeStatusClient, error) {
		return client.WatchRuntimeStatus(ctx, &controlv1.WatchRuntimeStatusRequest{Caller: caller})
	})
	if err != nil {
		return domain.RuntimeStatusSnapshot{}, err
	}
	resp, err := stream.Recv()
	if err != nil {
		if shouldSilenceStreamError(err) {
			return domain.RuntimeStatusSnapshot{}, nil
		}
		return domain.RuntimeStatusSnapshot{}, mapRPCError("runtime snapshot", err)
	}
	return fromProtoRuntimeStatusSnapshot(resp)
}

func (r *RemoteControlPlane) fetchServerInitSnapshot(ctx context.Context) (domain.ServerInitStatusSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
	}
	stream, err := withCallerStream(ctx, r, "server init snapshot", func(client controlv1.ControlPlaneServiceClient, caller string) (controlv1.ControlPlaneService_WatchServerInitStatusClient, error) {
		return client.WatchServerInitStatus(ctx, &controlv1.WatchServerInitStatusRequest{Caller: caller})
	})
	if err != nil {
		return domain.ServerInitStatusSnapshot{}, err
	}
	resp, err := stream.Recv()
	if err != nil {
		if shouldSilenceStreamError(err) {
			return domain.ServerInitStatusSnapshot{}, nil
		}
		return domain.ServerInitStatusSnapshot{}, mapRPCError("server init snapshot", err)
	}
	return fromProtoServerInitStatusSnapshot(resp)
}

func (r *RemoteControlPlane) control() controlv1.ControlPlaneServiceClient {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Control()
}

func poolInfoFromRuntime(status domain.ServerRuntimeStatus) domain.PoolInfo {
	instances := make([]domain.InstanceInfo, 0, len(status.Instances))
	for _, inst := range status.Instances {
		instances = append(instances, domain.InstanceInfo(inst))
	}
	return domain.PoolInfo{
		SpecKey:     status.SpecKey,
		ServerName:  status.ServerName,
		Instances:   instances,
		Metrics:     status.Metrics,
		Diagnostics: status.Diagnostics,
	}
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func shouldSilenceStreamError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	code := status.Code(err)
	return code == codes.Canceled
}
