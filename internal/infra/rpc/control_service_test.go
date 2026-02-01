package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	"mcpv/internal/infra/scheduler"
	controlv1 "mcpv/pkg/api/control/v1"
)

func TestControlService_CallToolNotFound(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		callToolErr: domain.ErrToolNotFound,
	}, nil, nil)

	_, err := svc.CallTool(context.Background(), &controlv1.CallToolRequest{
		Caller:        "caller",
		Name:          "missing",
		ArgumentsJson: json.RawMessage(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestControlService_CallToolMissingName(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{}, nil, nil)

	_, err := svc.CallTool(context.Background(), &controlv1.CallToolRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestControlService_GetPromptMissingName(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{}, nil, nil)

	_, err := svc.GetPrompt(context.Background(), &controlv1.GetPromptRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestControlService_ReadResourceMissingURI(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{}, nil, nil)

	_, err := svc.ReadResource(context.Background(), &controlv1.ReadResourceRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestControlService_CallToolDeadlineExceeded(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		callToolErr: context.DeadlineExceeded,
	}, nil, nil)

	_, err := svc.CallTool(context.Background(), &controlv1.CallToolRequest{
		Caller:        "caller",
		Name:          "echo.echo",
		ArgumentsJson: json.RawMessage(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, codes.DeadlineExceeded, status.Code(err))
}

func TestControlService_CallToolUnavailable(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		callToolErr: scheduler.ErrNoCapacity,
	}, nil, nil)

	_, err := svc.CallTool(context.Background(), &controlv1.CallToolRequest{
		Caller:        "caller",
		Name:          "echo.echo",
		ArgumentsJson: json.RawMessage(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestControlService_CallToolInvalidArgument(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		callToolErr: domain.ErrInvalidRequest,
	}, nil, nil)

	_, err := svc.CallTool(context.Background(), &controlv1.CallToolRequest{
		Caller:        "caller",
		Name:          "echo.echo",
		ArgumentsJson: json.RawMessage(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestControlService_ListTools(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		snapshot: domain.ToolSnapshot{
			ETag: "v1",
			Tools: []domain.ToolDefinition{
				{Name: "echo.echo", InputSchema: map[string]any{"type": "object"}},
			},
		},
	}, nil, nil)

	resp, err := svc.ListTools(context.Background(), &controlv1.ListToolsRequest{Caller: "caller"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.GetSnapshot())
	require.Equal(t, "v1", resp.GetSnapshot().GetEtag())
	require.Len(t, resp.GetSnapshot().GetTools(), 1)
	require.Equal(t, "echo.echo", resp.GetSnapshot().GetTools()[0].GetName())
}

func TestControlService_RegisterCaller(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		registerRegistration: domain.ClientRegistration{Client: "caller"},
	}, nil, nil)

	resp, err := svc.RegisterCaller(context.Background(), &controlv1.RegisterCallerRequest{
		Caller: "caller",
		Pid:    1234,
	})
	require.NoError(t, err)
	require.Equal(t, "caller", resp.GetProfile())
}

func TestControlService_ListToolsRequiresCaller(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		listToolsErr: domain.ErrClientNotRegistered,
	}, nil, nil)

	_, err := svc.ListTools(context.Background(), &controlv1.ListToolsRequest{Caller: "caller"})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestControlService_WatchToolsInitialSnapshot(t *testing.T) {
	// Test that WatchTools atomically sends initial snapshot without race condition
	initialSnapshot := domain.ToolSnapshot{
		ETag: "v1",
		Tools: []domain.ToolDefinition{
			{Name: "tool1", InputSchema: map[string]any{"type": "object"}},
		},
	}

	updateSnapshot := domain.ToolSnapshot{
		ETag: "v2",
		Tools: []domain.ToolDefinition{
			{Name: "tool1", InputSchema: map[string]any{"type": "object"}},
			{Name: "tool2", InputSchema: map[string]any{"type": "object"}},
		},
	}

	watchCh := make(chan domain.ToolSnapshot, 2)
	watchCh <- initialSnapshot
	watchCh <- updateSnapshot
	close(watchCh)

	svc := NewControlService(&fakeControlPlane{
		watchToolsCh: watchCh,
	}, nil, nil)

	stream := &fakeWatchToolsStream{
		ctx:       context.Background(),
		snapshots: []domain.ToolSnapshot{},
	}

	err := svc.WatchTools(&controlv1.WatchToolsRequest{Caller: "caller"}, stream)
	require.NoError(t, err)

	// Should receive both initial and update snapshots
	require.Len(t, stream.snapshots, 2)
	require.Equal(t, "v1", stream.snapshots[0].ETag)
	require.Equal(t, "v2", stream.snapshots[1].ETag)
}

func TestControlService_WatchToolsSkipsDuplicateETag(t *testing.T) {
	// Test that WatchTools skips sending snapshot if client already has it
	snapshot := domain.ToolSnapshot{
		ETag: "v1",
		Tools: []domain.ToolDefinition{
			{Name: "tool1", InputSchema: map[string]any{"type": "object"}},
		},
	}

	watchCh := make(chan domain.ToolSnapshot, 1)
	watchCh <- snapshot
	close(watchCh)

	svc := NewControlService(&fakeControlPlane{
		watchToolsCh: watchCh,
	}, nil, nil)

	stream := &fakeWatchToolsStream{
		ctx:       context.Background(),
		snapshots: []domain.ToolSnapshot{},
	}

	// Client already has v1
	err := svc.WatchTools(&controlv1.WatchToolsRequest{
		Caller:   "caller",
		LastEtag: "v1",
	}, stream)
	require.NoError(t, err)

	// Should not receive any snapshots since client already has v1
	require.Len(t, stream.snapshots, 0)
}

type fakeWatchToolsStream struct {
	ctx       context.Context
	snapshots []domain.ToolSnapshot
}

func (f *fakeWatchToolsStream) Send(snapshot *controlv1.ToolsSnapshot) error {
	tools := make([]domain.ToolDefinition, len(snapshot.GetTools()))
	for i, t := range snapshot.GetTools() {
		tools[i] = domain.ToolDefinition{
			Name:        t.GetName(),
			InputSchema: make(map[string]any),
		}
	}
	f.snapshots = append(f.snapshots, domain.ToolSnapshot{
		ETag:  snapshot.GetEtag(),
		Tools: tools,
	})
	return nil
}

func (f *fakeWatchToolsStream) Context() context.Context {
	return f.ctx
}

func (f *fakeWatchToolsStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeWatchToolsStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeWatchToolsStream) SetTrailer(metadata.MD)       {}
func (f *fakeWatchToolsStream) SendMsg(any) error            { return nil }
func (f *fakeWatchToolsStream) RecvMsg(any) error            { return nil }

type fakeControlPlane struct {
	snapshot             domain.ToolSnapshot
	resourcePage         domain.ResourcePage
	promptPage           domain.PromptPage
	callToolErr          error
	readResourceErr      error
	getPromptErr         error
	listToolsErr         error
	registerRegistration domain.ClientRegistration
	registerErr          error
	unregisterErr        error
	watchToolsCh         <-chan domain.ToolSnapshot
}

func (f *fakeControlPlane) Info(_ context.Context) (domain.ControlPlaneInfo, error) {
	return domain.ControlPlaneInfo{}, nil
}

func (f *fakeControlPlane) RegisterClient(_ context.Context, client string, _ int, _ []string, _ string) (domain.ClientRegistration, error) {
	if f.registerErr != nil {
		return domain.ClientRegistration{}, f.registerErr
	}
	if f.registerRegistration.Client == "" {
		return domain.ClientRegistration{Client: client}, nil
	}
	return f.registerRegistration, nil
}

func (f *fakeControlPlane) UnregisterClient(_ context.Context, _ string) error {
	return f.unregisterErr
}

func (f *fakeControlPlane) ListActiveClients(_ context.Context) ([]domain.ActiveClient, error) {
	return nil, nil
}

func (f *fakeControlPlane) WatchActiveClients(_ context.Context) (<-chan domain.ActiveClientSnapshot, error) {
	ch := make(chan domain.ActiveClientSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) ListTools(_ context.Context, _ string) (domain.ToolSnapshot, error) {
	if f.listToolsErr != nil {
		return domain.ToolSnapshot{}, f.listToolsErr
	}
	return f.snapshot, nil
}

func (f *fakeControlPlane) ListToolCatalog(_ context.Context) (domain.ToolCatalogSnapshot, error) {
	if f.listToolsErr != nil {
		return domain.ToolCatalogSnapshot{}, f.listToolsErr
	}
	return domain.ToolCatalogSnapshot{}, nil
}

func (f *fakeControlPlane) WatchTools(_ context.Context, _ string) (<-chan domain.ToolSnapshot, error) {
	if f.watchToolsCh != nil {
		return f.watchToolsCh, nil
	}
	ch := make(chan domain.ToolSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) CallTool(_ context.Context, _, _ string, _ json.RawMessage, _ string) (json.RawMessage, error) {
	if f.callToolErr != nil {
		return nil, f.callToolErr
	}
	return json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`), nil
}

func (f *fakeControlPlane) CallToolAll(_ context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	return f.CallTool(context.TODO(), "", name, args, routingKey)
}

func (f *fakeControlPlane) ListResources(_ context.Context, _ string, _ string) (domain.ResourcePage, error) {
	return f.resourcePage, nil
}

func (f *fakeControlPlane) ListResourcesAll(_ context.Context, _ string) (domain.ResourcePage, error) {
	return f.resourcePage, nil
}

func (f *fakeControlPlane) WatchResources(_ context.Context, _ string) (<-chan domain.ResourceSnapshot, error) {
	ch := make(chan domain.ResourceSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) ReadResource(_ context.Context, _, _ string) (json.RawMessage, error) {
	if f.readResourceErr != nil {
		return nil, f.readResourceErr
	}
	return json.RawMessage(`{"contents":[{"uri":"file:///a","text":"ok"}]}`), nil
}

func (f *fakeControlPlane) ReadResourceAll(_ context.Context, uri string) (json.RawMessage, error) {
	return f.ReadResource(context.TODO(), "", uri)
}

func (f *fakeControlPlane) ListPrompts(_ context.Context, _ string, _ string) (domain.PromptPage, error) {
	return f.promptPage, nil
}

func (f *fakeControlPlane) ListPromptsAll(_ context.Context, _ string) (domain.PromptPage, error) {
	return f.promptPage, nil
}

func (f *fakeControlPlane) WatchPrompts(_ context.Context, _ string) (<-chan domain.PromptSnapshot, error) {
	ch := make(chan domain.PromptSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) GetPrompt(_ context.Context, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	if f.getPromptErr != nil {
		return nil, f.getPromptErr
	}
	return json.RawMessage(`{"messages":[{"role":"user","content":{"type":"text","text":"ok"}}]}`), nil
}

func (f *fakeControlPlane) GetPromptAll(_ context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	return f.GetPrompt(context.TODO(), "", name, args)
}

func (f *fakeControlPlane) StreamLogs(_ context.Context, _ string, _ domain.LogLevel) (<-chan domain.LogEntry, error) {
	ch := make(chan domain.LogEntry)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) StreamLogsAllServers(_ context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return f.StreamLogs(context.TODO(), "", minLevel)
}

func (f *fakeControlPlane) GetCatalog() domain.Catalog {
	return domain.Catalog{}
}

func (f *fakeControlPlane) GetPoolStatus(_ context.Context) ([]domain.PoolInfo, error) {
	return nil, nil
}

func (f *fakeControlPlane) GetServerInitStatus(_ context.Context) ([]domain.ServerInitStatus, error) {
	return nil, nil
}

func (f *fakeControlPlane) GetBootstrapProgress(_ context.Context) (domain.BootstrapProgress, error) {
	return domain.BootstrapProgress{State: domain.BootstrapCompleted}, nil
}

func (f *fakeControlPlane) WatchBootstrapProgress(_ context.Context) (<-chan domain.BootstrapProgress, error) {
	ch := make(chan domain.BootstrapProgress)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) RetryServerInit(_ context.Context, _ string) error {
	return nil
}

func (f *fakeControlPlane) WatchRuntimeStatus(_ context.Context, _ string) (<-chan domain.RuntimeStatusSnapshot, error) {
	ch := make(chan domain.RuntimeStatusSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) WatchRuntimeStatusAllServers(_ context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	return f.WatchRuntimeStatus(context.TODO(), "")
}

func (f *fakeControlPlane) WatchServerInitStatus(_ context.Context, _ string) (<-chan domain.ServerInitStatusSnapshot, error) {
	ch := make(chan domain.ServerInitStatusSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) WatchServerInitStatusAllServers(_ context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	return f.WatchServerInitStatus(context.TODO(), "")
}

func (f *fakeControlPlane) AutomaticMCP(_ context.Context, _ string, _ domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	return domain.AutomaticMCPResult{}, nil
}

func (f *fakeControlPlane) AutomaticEval(_ context.Context, client string, params domain.AutomaticEvalParams) (json.RawMessage, error) {
	return f.CallTool(context.TODO(), client, params.ToolName, params.Arguments, params.RoutingKey)
}

func (f *fakeControlPlane) IsSubAgentEnabled() bool {
	return false
}

func (f *fakeControlPlane) IsSubAgentEnabledForClient(_ string) bool {
	return false
}

func (f *fakeControlPlane) CallToolTask(_ context.Context, _, _ string, _ json.RawMessage, _ string, _ domain.TaskCreateOptions) (domain.Task, error) {
	return domain.Task{}, nil
}

func (f *fakeControlPlane) GetTask(_ context.Context, _, _ string) (domain.Task, error) {
	return domain.Task{}, nil
}

func (f *fakeControlPlane) ListTasks(_ context.Context, _, _ string, _ int) (domain.TaskPage, error) {
	return domain.TaskPage{}, nil
}

func (f *fakeControlPlane) GetTaskResult(_ context.Context, _, _ string) (domain.TaskResult, error) {
	return domain.TaskResult{}, nil
}

func (f *fakeControlPlane) CancelTask(_ context.Context, _, _ string) (domain.Task, error) {
	return domain.Task{}, nil
}
