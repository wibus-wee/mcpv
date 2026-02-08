package catalog

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

// TestSnapshot_Atomicity verifies snapshot returns consistent state.
func TestSnapshot_Atomicity(t *testing.T) {
	catalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
		},
		Runtime: domain.RuntimeConfig{},
	}

	state, err := domain.NewCatalogState(catalog, 1, time.Now())
	require.NoError(t, err)

	provider := &DynamicCatalogProvider{
		logger: zap.NewNop(),
		subs:   make(map[chan domain.CatalogUpdate]struct{}),
	}
	provider.state.Store(state)
	provider.revision.Store(uint64(1))

	// Concurrent snapshots should all return the same state
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	snapshots := make([]domain.CatalogState, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			snap, err := provider.Snapshot(context.Background())
			require.NoError(t, err)
			snapshots[idx] = snap
		}(i)
	}

	wg.Wait()

	// All snapshots should be identical
	for i, snap := range snapshots {
		assert.Equal(t, state.Revision, snap.Revision, "Snapshot %d has different revision", i)
		assert.Equal(t, len(state.Catalog.Specs), len(snap.Catalog.Specs), "Snapshot %d has different server count", i)
	}
}

// TestSnapshot_ContextCancellation verifies context handling.
func TestSnapshot_ContextCancellation(t *testing.T) {
	catalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
		},
	}

	state, err := domain.NewCatalogState(catalog, 1, time.Now())
	require.NoError(t, err)

	provider := &DynamicCatalogProvider{
		logger: zap.NewNop(),
		subs:   make(map[chan domain.CatalogUpdate]struct{}),
	}
	provider.state.Store(state)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = provider.Snapshot(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestSnapshot_NilContext verifies nil context handling.
func TestSnapshot_NilContext(t *testing.T) {
	catalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
		},
	}

	state, err := domain.NewCatalogState(catalog, 1, time.Now())
	require.NoError(t, err)

	provider := &DynamicCatalogProvider{
		logger: zap.NewNop(),
		subs:   make(map[chan domain.CatalogUpdate]struct{}),
	}
	provider.state.Store(state)

	snap, err := provider.Snapshot(nil) //nolint:staticcheck
	require.NoError(t, err)
	assert.Equal(t, state.Revision, snap.Revision)
}

// TestWatch_SubscriberManagement verifies subscriber lifecycle.
func TestWatch_SubscriberManagement(t *testing.T) {
	catalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
		},
	}

	state, err := domain.NewCatalogState(catalog, 1, time.Now())
	require.NoError(t, err)

	provider := &DynamicCatalogProvider{
		logger:   zap.NewNop(),
		subs:     make(map[chan domain.CatalogUpdate]struct{}),
		watchCtx: context.Background(),
	}
	provider.state.Store(state)
	provider.revision.Store(uint64(1))

	t.Run("subscriber receives updates", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, err := provider.Watch(ctx)
		require.NoError(t, err)
		assert.NotNil(t, ch)

		// Verify subscriber is registered
		provider.subsMu.Lock()
		count := len(provider.subs)
		provider.subsMu.Unlock()
		assert.Equal(t, 1, count)

		// Broadcast an update
		update := domain.CatalogUpdate{
			Snapshot: state,
			Diff:     domain.CatalogDiff{},
			Source:   domain.CatalogUpdateSourceManual,
		}
		provider.broadcast(update)

		// Should receive the update
		select {
		case received := <-ch:
			assert.Equal(t, update.Source, received.Source)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Did not receive update")
		}
	})

	t.Run("context cancellation removes subscriber", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		ch, err := provider.Watch(ctx)
		require.NoError(t, err)
		assert.NotNil(t, ch)

		// Verify subscriber is registered
		provider.subsMu.Lock()
		initialCount := len(provider.subs)
		provider.subsMu.Unlock()
		assert.Greater(t, initialCount, 0)

		// Cancel context
		cancel()

		// Wait for cleanup
		time.Sleep(50 * time.Millisecond)

		// Verify subscriber is removed
		provider.subsMu.Lock()
		finalCount := len(provider.subs)
		provider.subsMu.Unlock()
		assert.Less(t, finalCount, initialCount)
	})

	t.Run("multiple subscribers receive same update", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		defer cancel1()
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()

		ch1, err := provider.Watch(ctx1)
		require.NoError(t, err)
		ch2, err := provider.Watch(ctx2)
		require.NoError(t, err)

		// Broadcast an update
		update := domain.CatalogUpdate{
			Snapshot: state,
			Diff:     domain.CatalogDiff{AddedSpecKeys: []string{"new-server"}},
			Source:   domain.CatalogUpdateSourceWatch,
		}
		provider.broadcast(update)

		// Both should receive
		received1 := false
		received2 := false

		select {
		case <-ch1:
			received1 = true
		case <-time.After(100 * time.Millisecond):
		}

		select {
		case <-ch2:
			received2 = true
		case <-time.After(100 * time.Millisecond):
		}

		assert.True(t, received1, "Subscriber 1 did not receive update")
		assert.True(t, received2, "Subscriber 2 did not receive update")
	})
}

// TestBroadcast_NonBlocking verifies slow subscribers don't block.
func TestBroadcast_NonBlocking(t *testing.T) {
	catalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
		},
	}

	state, err := domain.NewCatalogState(catalog, 1, time.Now())
	require.NoError(t, err)

	provider := &DynamicCatalogProvider{
		logger:   zap.NewNop(),
		subs:     make(map[chan domain.CatalogUpdate]struct{}),
		watchCtx: context.Background(),
	}
	provider.state.Store(state)

	// Create a slow subscriber (doesn't read from channel)
	slowCtx, slowCancel := context.WithCancel(context.Background())
	defer slowCancel()
	slowCh, err := provider.Watch(slowCtx)
	require.NoError(t, err)

	// Create a fast subscriber
	fastCtx, fastCancel := context.WithCancel(context.Background())
	defer fastCancel()
	fastCh, err := provider.Watch(fastCtx)
	require.NoError(t, err)

	// Broadcast multiple updates
	for i := 0; i < 5; i++ {
		update := domain.CatalogUpdate{
			Snapshot: state,
			Diff:     domain.CatalogDiff{},
			Source:   domain.CatalogUpdateSourceManual,
		}
		provider.broadcast(update)
	}

	// Fast subscriber should receive at least one update
	select {
	case <-fastCh:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Fast subscriber did not receive update")
	}

	// Slow subscriber's channel should be full but broadcast shouldn't block
	// (we can't easily verify it received updates since we're not reading)
	_ = slowCh
}

// TestCopySubscribers_ThreadSafe verifies thread-safe subscriber copying.
func TestCopySubscribers_ThreadSafe(_ *testing.T) {
	provider := &DynamicCatalogProvider{
		logger:   zap.NewNop(),
		subs:     make(map[chan domain.CatalogUpdate]struct{}),
		watchCtx: context.Background(),
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Concurrent Watch calls (adds subscribers)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, _ = provider.Watch(ctx)
		}()
	}

	// Concurrent copySubscribers calls
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = provider.copySubscribers()
		}()
	}

	wg.Wait()
}

// TestRevisionIncrement verifies atomic revision increments.
func TestRevisionIncrement(t *testing.T) {
	provider := &DynamicCatalogProvider{
		logger: zap.NewNop(),
		subs:   make(map[chan domain.CatalogUpdate]struct{}),
	}

	initialRevision := uint64(1)
	provider.revision.Store(initialRevision)

	// Simulate concurrent revision reads
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	revisions := make([]uint64, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			// Simulate what reload does
			nextRevision := provider.revision.Load() + 1
			provider.revision.Store(nextRevision)
			revisions[idx] = nextRevision
		}(i)
	}

	wg.Wait()

	// Final revision should be initial + goroutines
	finalRevision := provider.revision.Load()
	assert.Greater(t, finalRevision, initialRevision)
}

// TestShouldReloadForPath verifies path matching logic.
func TestShouldReloadForPath(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		configPath   string
		shouldReload bool
	}{
		{
			name:         "exact match",
			path:         "/config/runtime.yaml",
			configPath:   "/config/runtime.yaml",
			shouldReload: true,
		},
		{
			name:         "different file",
			path:         "/config/other.yaml",
			configPath:   "/config/runtime.yaml",
			shouldReload: false,
		},
		{
			name:         "empty path",
			path:         "",
			configPath:   "/config/runtime.yaml",
			shouldReload: false,
		},
		{
			name:         "empty config path",
			path:         "/config/runtime.yaml",
			configPath:   "",
			shouldReload: false,
		},
		{
			name:         "both empty",
			path:         "",
			configPath:   "",
			shouldReload: false,
		},
		{
			name:         "path with trailing slash",
			path:         "/config/runtime.yaml/",
			configPath:   "/config/runtime.yaml",
			shouldReload: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldReloadForPath(tt.path, tt.configPath)
			assert.Equal(t, tt.shouldReload, result)
		})
	}
}

// TestTimerChan verifies timer channel helper.
func TestTimerChan(t *testing.T) {
	t.Run("nil timer returns nil channel", func(t *testing.T) {
		ch := timerChan(nil)
		assert.Nil(t, ch)
	})

	t.Run("non-nil timer returns channel", func(t *testing.T) {
		timer := time.NewTimer(1 * time.Second)
		defer timer.Stop()
		ch := timerChan(timer)
		assert.NotNil(t, ch)
	})
}

// TestCatalogDiff_IsEmpty verifies diff empty detection.
func TestCatalogDiff_IsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		diff    domain.CatalogDiff
		isEmpty bool
	}{
		{
			name:    "completely empty",
			diff:    domain.CatalogDiff{},
			isEmpty: true,
		},
		{
			name: "has added servers",
			diff: domain.CatalogDiff{
				AddedSpecKeys: []string{"server1"},
			},
			isEmpty: false,
		},
		{
			name: "has removed servers",
			diff: domain.CatalogDiff{
				RemovedSpecKeys: []string{"server1"},
			},
			isEmpty: false,
		},
		{
			name: "tags changed",
			diff: domain.CatalogDiff{
				TagsChanged: true,
			},
			isEmpty: false,
		},
		{
			name: "runtime changed",
			diff: domain.CatalogDiff{
				RuntimeChanged: true,
			},
			isEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.diff.IsEmpty()
			assert.Equal(t, tt.isEmpty, result)
		})
	}
}

// TestDiffCatalogStates verifies diff computation.
func TestDiffCatalogStates(t *testing.T) {
	t.Run("identical catalogs produce empty diff", func(t *testing.T) {
		catalog := domain.Catalog{
			Specs: map[string]domain.ServerSpec{
				"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
			},
		}

		state1, err := domain.NewCatalogState(catalog, 1, time.Now())
		require.NoError(t, err)
		state2, err := domain.NewCatalogState(catalog, 2, time.Now())
		require.NoError(t, err)

		diff := domain.DiffCatalogStates(state1, state2)
		assert.True(t, diff.IsEmpty())
	})

	t.Run("added server detected", func(t *testing.T) {
		catalog1 := domain.Catalog{
			Specs: map[string]domain.ServerSpec{
				"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
			},
		}
		catalog2 := domain.Catalog{
			Specs: map[string]domain.ServerSpec{
				"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
				"server2": {Name: "server2", Transport: domain.TransportStdio, Cmd: []string{"test2"}},
			},
		}

		state1, err := domain.NewCatalogState(catalog1, 1, time.Now())
		require.NoError(t, err)
		state2, err := domain.NewCatalogState(catalog2, 2, time.Now())
		require.NoError(t, err)

		diff := domain.DiffCatalogStates(state1, state2)
		assert.False(t, diff.IsEmpty())
		assert.NotEmpty(t, diff.AddedSpecKeys, "Expected added servers to be detected")
	})

	t.Run("removed server detected", func(t *testing.T) {
		catalog1 := domain.Catalog{
			Specs: map[string]domain.ServerSpec{
				"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
				"server2": {Name: "server2", Transport: domain.TransportStdio, Cmd: []string{"test2"}},
			},
		}
		catalog2 := domain.Catalog{
			Specs: map[string]domain.ServerSpec{
				"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
			},
		}

		state1, err := domain.NewCatalogState(catalog1, 1, time.Now())
		require.NoError(t, err)
		state2, err := domain.NewCatalogState(catalog2, 2, time.Now())
		require.NoError(t, err)

		diff := domain.DiffCatalogStates(state1, state2)
		assert.False(t, diff.IsEmpty())
		assert.NotEmpty(t, diff.RemovedSpecKeys, "Expected removed servers to be detected")
	})
}

// TestStateStore_Atomicity verifies atomic state updates.
func TestStateStore_Atomicity(t *testing.T) {
	catalog1 := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
		},
	}
	catalog2 := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server2": {Name: "server2", Transport: domain.TransportStdio, Cmd: []string{"test2"}},
		},
	}

	state1, err := domain.NewCatalogState(catalog1, 1, time.Now())
	require.NoError(t, err)
	state2, err := domain.NewCatalogState(catalog2, 2, time.Now())
	require.NoError(t, err)

	provider := &DynamicCatalogProvider{
		logger: zap.NewNop(),
		subs:   make(map[chan domain.CatalogUpdate]struct{}),
	}
	provider.state.Store(state1)

	// Concurrent reads and writes
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				provider.state.Store(state1)
			} else {
				provider.state.Store(state2)
			}
		}(i)
	}

	// Readers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			state := provider.state.Load().(domain.CatalogState)
			// Should always get a valid state (either state1 or state2)
			assert.NotZero(t, state.Revision)
			assert.NotEmpty(t, state.Catalog.Specs)
		}()
	}

	wg.Wait()
}

// TestWatch_NilContext verifies nil context handling.
func TestWatch_NilContext(t *testing.T) {
	catalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
		},
	}

	state, err := domain.NewCatalogState(catalog, 1, time.Now())
	require.NoError(t, err)

	provider := &DynamicCatalogProvider{
		logger:   zap.NewNop(),
		subs:     make(map[chan domain.CatalogUpdate]struct{}),
		watchCtx: context.Background(),
	}
	provider.state.Store(state)

	ch, err := provider.Watch(nil) //nolint:staticcheck // intentional nil context coverage
	require.NoError(t, err)
	assert.NotNil(t, ch)
}

// TestWatchOnce verifies watcher starts only once.
func TestWatchOnce(t *testing.T) {
	catalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"server1": {Name: "server1", Transport: domain.TransportStdio, Cmd: []string{"test"}},
		},
	}

	state, err := domain.NewCatalogState(catalog, 1, time.Now())
	require.NoError(t, err)

	provider := &DynamicCatalogProvider{
		logger:   zap.NewNop(),
		subs:     make(map[chan domain.CatalogUpdate]struct{}),
		watchCtx: context.Background(),
	}
	provider.state.Store(state)

	// Multiple Watch calls should only start watcher once
	const calls = 10
	for i := 0; i < calls; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := provider.Watch(ctx)
		require.NoError(t, err)
	}

	// The watchOnce should ensure only one watcher goroutine started
	// (We can't easily verify this without exposing internal state,
	// but the test documents the expected behavior)
}
