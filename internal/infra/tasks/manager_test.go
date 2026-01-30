package tasks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcpv/internal/domain"
)

func TestManagerCreateAndResult(t *testing.T) {
	manager := NewManager()
	defer manager.Stop()
	ctx := context.Background()

	payload := json.RawMessage(`{"ok":true}`)
	task, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{}, func(_ context.Context) (domain.TaskRunResult, error) {
		return domain.TaskRunResult{Result: payload}, nil
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
	defer manager.Stop()
	ctx := context.Background()

	task, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{}, func(ctx context.Context) (domain.TaskRunResult, error) {
		<-ctx.Done()
		return domain.TaskRunResult{}, ctx.Err()
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
	defer manager.Stop()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{}, func(_ context.Context) (domain.TaskRunResult, error) {
			return domain.TaskRunResult{Result: json.RawMessage(`{}`)}, nil
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
	defer manager.Stop()
	manager.now = func() time.Time { return time.Unix(0, 0) }
	ctx := context.Background()

	ttl := int64(10)
	task, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{TTL: &ttl}, func(_ context.Context) (domain.TaskRunResult, error) {
		return domain.TaskRunResult{Result: json.RawMessage(`{}`)}, nil
	})
	require.NoError(t, err)

	manager.now = func() time.Time { return time.Unix(0, 0).Add(20 * time.Millisecond) }
	_, err = manager.Get(ctx, "client-a", task.TaskID)
	require.ErrorIs(t, err, domain.ErrTaskNotFound)
}

func TestManagerBackgroundPurge(t *testing.T) {
	manager := NewManager()
	defer manager.Stop()

	// Use a counter to generate unique task IDs
	counter := int64(0)
	manager.now = func() time.Time {
		counter++
		return time.Unix(1000+counter, 0)
	}
	ctx := context.Background()

	// Create tasks with short TTL that block to keep tasks alive
	ttl := int64(100) // 100ms
	blockChan := make(chan struct{})
	defer close(blockChan)

	for i := 0; i < 5; i++ {
		_, err := manager.Create(ctx, "client-a", domain.TaskCreateOptions{TTL: &ttl}, func(ctx context.Context) (domain.TaskRunResult, error) {
			// Block until test is done or context cancelled
			select {
			case <-ctx.Done():
				return domain.TaskRunResult{}, ctx.Err()
			case <-blockChan:
				return domain.TaskRunResult{Result: json.RawMessage(`{}`)}, nil
			}
		})
		require.NoError(t, err)
	}

	// Give tasks a moment to start
	time.Sleep(10 * time.Millisecond)

	// Verify tasks exist
	manager.mu.Lock()
	require.Len(t, manager.tasks, 5)
	manager.mu.Unlock()

	// Advance time beyond TTL
	manager.now = func() time.Time { return time.Unix(2000, 0) }

	// Trigger purge manually to simulate background purge
	manager.mu.Lock()
	manager.purgeExpiredLocked()
	taskCount := len(manager.tasks)
	manager.mu.Unlock()

	// All tasks should be purged
	require.Equal(t, 0, taskCount, "expired tasks should be purged")
}

func TestManagerStop(t *testing.T) {
	manager := NewManager()

	// Should not block
	done := make(chan struct{})
	go func() {
		manager.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() should not block")
	}
}
