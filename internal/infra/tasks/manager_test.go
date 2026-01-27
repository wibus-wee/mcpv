package tasks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcpd/internal/domain"
)

func TestManagerCreateAndResult(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	payload := json.RawMessage(`{"ok":true}`)
	task, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{}, func(ctx context.Context) (json.RawMessage, *domain.ProtocolError, error) {
		return payload, nil, nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, task.TaskID)

	result, err := manager.Result(ctx, "client-a", task.TaskID)
	require.NoError(t, err)
	require.Equal(t, domain.TaskStatusCompleted, result.Status)
	require.JSONEq(t, string(payload), string(result.Result))
}

func TestManagerCancel(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	task, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{}, func(ctx context.Context) (json.RawMessage, *domain.ProtocolError, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	})
	require.NoError(t, err)

	err = manager.Cancel(ctx, "client-a", task.TaskID)
	require.NoError(t, err)

	result, err := manager.Result(ctx, "client-a", task.TaskID)
	require.NoError(t, err)
	require.Equal(t, domain.TaskStatusCancelled, result.Status)
}

func TestManagerList(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{}, func(ctx context.Context) (json.RawMessage, *domain.ProtocolError, error) {
			return json.RawMessage(`{}`), nil, nil
		})
		require.NoError(t, err)
	}

	page, err := manager.List(ctx, "client-a", "", 2)
	require.NoError(t, err)
	require.Len(t, page.Tasks, 2)
	require.NotEmpty(t, page.NextCursor)

	page2, err := manager.List(ctx, "client-a", page.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, page2.Tasks, 1)
}

func TestManagerTTLExpiry(t *testing.T) {
	manager := NewManager()
	manager.now = func() time.Time { return time.Unix(0, 0) }
	ctx := context.Background()

	ttl := int64(10)
	task, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{TTL: &ttl}, func(ctx context.Context) (json.RawMessage, *domain.ProtocolError, error) {
		return json.RawMessage(`{}`), nil, nil
	})
	require.NoError(t, err)

	manager.now = func() time.Time { return time.Unix(0, 0).Add(20 * time.Millisecond) }
	_, ok := manager.Get(ctx, "client-a", task.TaskID)
	require.False(t, ok)
}
