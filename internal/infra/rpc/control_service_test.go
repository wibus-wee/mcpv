package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpd/internal/domain"
	"mcpd/internal/infra/scheduler"
	controlv1 "mcpd/pkg/api/control/v1"
)

func TestControlService_CallToolNotFound(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		callToolErr: domain.ErrToolNotFound,
	}, nil)

	_, err := svc.CallTool(context.Background(), &controlv1.CallToolRequest{
		Caller:        "caller",
		Name:          "missing",
		ArgumentsJson: json.RawMessage(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestControlService_CallToolMissingName(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{}, nil)

	_, err := svc.CallTool(context.Background(), &controlv1.CallToolRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestControlService_GetPromptMissingName(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{}, nil)

	_, err := svc.GetPrompt(context.Background(), &controlv1.GetPromptRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestControlService_ReadResourceMissingURI(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{}, nil)

	_, err := svc.ReadResource(context.Background(), &controlv1.ReadResourceRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestControlService_CallToolDeadlineExceeded(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		callToolErr: context.DeadlineExceeded,
	}, nil)

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
	}, nil)

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
	}, nil)

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
				{Name: "echo.echo", ToolJSON: json.RawMessage(`{"name":"echo.echo","inputSchema":{"type":"object"}}`)},
			},
		},
	}, nil)

	resp, err := svc.ListTools(context.Background(), &controlv1.ListToolsRequest{Caller: "caller"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Snapshot)
	require.Equal(t, "v1", resp.Snapshot.Etag)
	require.Len(t, resp.Snapshot.Tools, 1)
	require.Equal(t, "echo.echo", resp.Snapshot.Tools[0].Name)
}

func TestControlService_RegisterCaller(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		registerProfile: "default",
	}, nil)

	resp, err := svc.RegisterCaller(context.Background(), &controlv1.RegisterCallerRequest{
		Caller: "caller",
		Pid:    1234,
	})
	require.NoError(t, err)
	require.Equal(t, "default", resp.Profile)
}

func TestControlService_ListToolsRequiresCaller(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		listToolsErr: domain.ErrCallerNotRegistered,
	}, nil)

	_, err := svc.ListTools(context.Background(), &controlv1.ListToolsRequest{Caller: "caller"})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

type fakeControlPlane struct {
	snapshot        domain.ToolSnapshot
	resourcePage    domain.ResourcePage
	promptPage      domain.PromptPage
	callToolErr     error
	readResourceErr error
	getPromptErr    error
	listToolsErr    error
	registerProfile string
	registerErr     error
	unregisterErr   error
}

func (f *fakeControlPlane) Info(ctx context.Context) (domain.ControlPlaneInfo, error) {
	return domain.ControlPlaneInfo{}, nil
}

func (f *fakeControlPlane) RegisterCaller(ctx context.Context, caller string, pid int) (string, error) {
	if f.registerErr != nil {
		return "", f.registerErr
	}
	if f.registerProfile == "" {
		return "default", nil
	}
	return f.registerProfile, nil
}

func (f *fakeControlPlane) UnregisterCaller(ctx context.Context, caller string) error {
	return f.unregisterErr
}

func (f *fakeControlPlane) ListTools(ctx context.Context, caller string) (domain.ToolSnapshot, error) {
	if f.listToolsErr != nil {
		return domain.ToolSnapshot{}, f.listToolsErr
	}
	return f.snapshot, nil
}

func (f *fakeControlPlane) WatchTools(ctx context.Context, caller string) (<-chan domain.ToolSnapshot, error) {
	ch := make(chan domain.ToolSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) CallTool(ctx context.Context, caller, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	if f.callToolErr != nil {
		return nil, f.callToolErr
	}
	return json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`), nil
}

func (f *fakeControlPlane) ListResources(ctx context.Context, caller string, cursor string) (domain.ResourcePage, error) {
	return f.resourcePage, nil
}

func (f *fakeControlPlane) WatchResources(ctx context.Context, caller string) (<-chan domain.ResourceSnapshot, error) {
	ch := make(chan domain.ResourceSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) ReadResource(ctx context.Context, caller, uri string) (json.RawMessage, error) {
	if f.readResourceErr != nil {
		return nil, f.readResourceErr
	}
	return json.RawMessage(`{"contents":[{"uri":"file:///a","text":"ok"}]}`), nil
}

func (f *fakeControlPlane) ListPrompts(ctx context.Context, caller string, cursor string) (domain.PromptPage, error) {
	return f.promptPage, nil
}

func (f *fakeControlPlane) WatchPrompts(ctx context.Context, caller string) (<-chan domain.PromptSnapshot, error) {
	ch := make(chan domain.PromptSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) GetPrompt(ctx context.Context, caller, name string, args json.RawMessage) (json.RawMessage, error) {
	if f.getPromptErr != nil {
		return nil, f.getPromptErr
	}
	return json.RawMessage(`{"messages":[{"role":"user","content":{"type":"text","text":"ok"}}]}`), nil
}

func (f *fakeControlPlane) StreamLogs(ctx context.Context, caller string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	ch := make(chan domain.LogEntry)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) GetProfileStore() domain.ProfileStore {
	return domain.ProfileStore{}
}

func (f *fakeControlPlane) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	return nil, nil
}
