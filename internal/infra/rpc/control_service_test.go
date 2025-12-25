package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpd/internal/domain"
	controlv1 "mcpd/pkg/api/control/v1"
)

func TestControlService_CallToolNotFound(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		callToolErr: domain.ErrToolNotFound,
	}, nil)

	_, err := svc.CallTool(context.Background(), &controlv1.CallToolRequest{
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

func TestControlService_ListTools(t *testing.T) {
	svc := NewControlService(&fakeControlPlane{
		snapshot: domain.ToolSnapshot{
			ETag: "v1",
			Tools: []domain.ToolDefinition{
				{Name: "echo.echo", ToolJSON: json.RawMessage(`{"name":"echo.echo","inputSchema":{"type":"object"}}`)},
			},
		},
	}, nil)

	resp, err := svc.ListTools(context.Background(), &controlv1.ListToolsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Snapshot)
	require.Equal(t, "v1", resp.Snapshot.Etag)
	require.Len(t, resp.Snapshot.Tools, 1)
	require.Equal(t, "echo.echo", resp.Snapshot.Tools[0].Name)
}

type fakeControlPlane struct {
	snapshot    domain.ToolSnapshot
	callToolErr error
}

func (f *fakeControlPlane) Info(ctx context.Context) (domain.ControlPlaneInfo, error) {
	return domain.ControlPlaneInfo{}, nil
}

func (f *fakeControlPlane) ListTools(ctx context.Context) (domain.ToolSnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeControlPlane) WatchTools(ctx context.Context) (<-chan domain.ToolSnapshot, error) {
	ch := make(chan domain.ToolSnapshot)
	close(ch)
	return ch, nil
}

func (f *fakeControlPlane) CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	if f.callToolErr != nil {
		return nil, f.callToolErr
	}
	return json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`), nil
}

func (f *fakeControlPlane) StreamLogs(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	ch := make(chan domain.LogEntry)
	close(ch)
	return ch, nil
}
