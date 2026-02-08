package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConn implements domain.Conn for testing.
type mockConn struct {
	callFunc func(ctx context.Context, msg json.RawMessage) (json.RawMessage, error)
}

func (m *mockConn) Call(ctx context.Context, msg json.RawMessage) (json.RawMessage, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, msg)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConn) Close() error {
	return nil
}

// TestPingProbe_SuccessfulPing verifies successful ping responses.
func TestPingProbe_SuccessfulPing(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
	}{
		{
			name: "valid response with empty result",
			response: mustEncodeResponse(t, &jsonrpc.Response{
				ID:     mustMakeID(t, "ping-1"),
				Result: json.RawMessage(`{}`),
			}),
		},
		{
			name: "valid response with result data",
			response: mustEncodeResponse(t, &jsonrpc.Response{
				ID:     mustMakeID(t, "ping-1"),
				Result: json.RawMessage(`{"status": "ok"}`),
			}),
		},
		{
			name: "valid response with null result",
			response: mustEncodeResponse(t, &jsonrpc.Response{
				ID:     mustMakeID(t, "ping-1"),
				Result: json.RawMessage(`null`),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockConn{
				callFunc: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
					return tt.response, nil
				},
			}

			probe := &PingProbe{Timeout: 2 * time.Second}
			err := probe.Ping(context.Background(), conn)
			assert.NoError(t, err)
		})
	}
}

// TestPingProbe_TimeoutHandling verifies timeout behavior.
func TestPingProbe_TimeoutHandling(t *testing.T) {
	tests := []struct {
		name          string
		timeout       time.Duration
		responseDelay time.Duration
		expectTimeout bool
	}{
		{
			name:          "fast response succeeds",
			timeout:       2 * time.Second,
			responseDelay: 10 * time.Millisecond,
			expectTimeout: false,
		},
		{
			name:          "slow response times out",
			timeout:       50 * time.Millisecond,
			responseDelay: 200 * time.Millisecond,
			expectTimeout: true,
		},
		{
			name:          "zero timeout uses default 2s",
			timeout:       0,
			responseDelay: 10 * time.Millisecond,
			expectTimeout: false,
		},
		{
			name:          "negative timeout uses default 2s",
			timeout:       -1 * time.Second,
			responseDelay: 10 * time.Millisecond,
			expectTimeout: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockConn{
				callFunc: func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
					select {
					case <-time.After(tt.responseDelay):
						return mustEncodeResponse(t, &jsonrpc.Response{
							ID:     mustMakeID(t, "ping-1"),
							Result: json.RawMessage(`{}`),
						}), nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				},
			}

			probe := &PingProbe{Timeout: tt.timeout}
			err := probe.Ping(context.Background(), conn)

			if tt.expectTimeout {
				require.Error(t, err)
				assert.ErrorIs(t, err, context.DeadlineExceeded)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestPingProbe_ErrorResponses verifies error handling.
func TestPingProbe_ErrorResponses(t *testing.T) {
	tests := []struct {
		name        string
		setupConn   func() *mockConn
		expectError string
	}{
		{
			name: "connection error",
			setupConn: func() *mockConn {
				return &mockConn{
					callFunc: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
						return nil, errors.New("connection failed")
					},
				}
			},
			expectError: "call ping: connection failed",
		},
		{
			name: "decode error",
			setupConn: func() *mockConn {
				return &mockConn{
					callFunc: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
						return []byte("invalid json"), nil
					},
				}
			},
			expectError: "decode ping response",
		},
		{
			name: "response error",
			setupConn: func() *mockConn {
				return &mockConn{
					callFunc: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
						return mustEncodeResponse(t, &jsonrpc.Response{
							ID: mustMakeID(t, "ping-1"),
							Error: &jsonrpc.Error{
								Code:    -32600,
								Message: "Invalid Request",
							},
						}), nil
					},
				}
			},
			expectError: "ping error",
		},
		{
			name: "wrong message type (notification)",
			setupConn: func() *mockConn {
				return &mockConn{
					callFunc: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
						// Return a properly formatted JSON-RPC 2.0 message but without an ID (notification)
						return json.RawMessage(`{"jsonrpc":"2.0","method":"ping","params":{}}`), nil
					},
				}
			},
			expectError: "ping response is not a response message",
		},
		{
			name: "wrong message type (request)",
			setupConn: func() *mockConn {
				return &mockConn{
					callFunc: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
						request := &jsonrpc.Request{
							ID:     mustMakeID(t, "ping-1"),
							Method: "ping",
							Params: json.RawMessage(`{}`),
						}
						data, _ := jsonrpc.EncodeMessage(request)
						return data, nil
					},
				}
			},
			expectError: "ping response is not a response message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := tt.setupConn()
			probe := &PingProbe{Timeout: 2 * time.Second}
			err := probe.Ping(context.Background(), conn)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestPingProbe_NilConnection verifies nil connection handling.
func TestPingProbe_NilConnection(t *testing.T) {
	probe := &PingProbe{Timeout: 2 * time.Second}
	err := probe.Ping(context.Background(), nil)

	require.Error(t, err)
	assert.Equal(t, "connection is nil", err.Error())
}

// TestPingProbe_ConcurrentPing verifies thread-safe concurrent pings.
func TestPingProbe_ConcurrentPing(t *testing.T) {
	const goroutines = 100

	conn := &mockConn{
		callFunc: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
			return mustEncodeResponse(t, &jsonrpc.Response{
				ID:     mustMakeID(t, "ping-1"),
				Result: json.RawMessage(`{}`),
			}), nil
		},
	}

	probe := &PingProbe{Timeout: 2 * time.Second}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	errors := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			errors[idx] = probe.Ping(context.Background(), conn)
		}(i)
	}

	wg.Wait()

	// All pings should succeed
	for i, err := range errors {
		assert.NoError(t, err, "Ping %d failed", i)
	}
}

// TestPingProbe_AtomicIDGeneration verifies unique ID generation.
func TestPingProbe_AtomicIDGeneration(t *testing.T) {
	const goroutines = 100

	seenIDs := make(map[string]bool)
	var mu sync.Mutex

	conn := &mockConn{
		callFunc: func(_ context.Context, msg json.RawMessage) (json.RawMessage, error) {
			// Decode the request to extract the ID
			decoded, err := jsonrpc.DecodeMessage(msg)
			if err != nil {
				return nil, err
			}

			req, ok := decoded.(*jsonrpc.Request)
			if !ok {
				return nil, errors.New("not a request")
			}

			// Record the ID - use fmt.Sprintf to get string representation
			mu.Lock()
			idStr := fmt.Sprintf("%v", req.ID)
			if seenIDs[idStr] {
				mu.Unlock()
				return nil, errors.New("duplicate ID detected: " + idStr)
			}
			seenIDs[idStr] = true
			mu.Unlock()

			// Return success response with the same ID
			return mustEncodeResponse(t, &jsonrpc.Response{
				ID:     req.ID,
				Result: json.RawMessage(`{}`),
			}), nil
		},
	}

	probe := &PingProbe{Timeout: 2 * time.Second}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	errors := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			errors[idx] = probe.Ping(context.Background(), conn)
		}(i)
	}

	wg.Wait()

	// All pings should succeed (no duplicate IDs)
	for i, err := range errors {
		assert.NoError(t, err, "Ping %d failed", i)
	}

	// All IDs should be unique
	assert.Equal(t, goroutines, len(seenIDs), "Expected all IDs to be unique")
}

// TestPingProbe_ContextCancellation verifies context cancellation handling.
func TestPingProbe_ContextCancellation(t *testing.T) {
	conn := &mockConn{
		callFunc: func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
			// Simulate slow response
			select {
			case <-time.After(1 * time.Second):
				return mustEncodeResponse(t, &jsonrpc.Response{
					ID:     mustMakeID(t, "ping-1"),
					Result: json.RawMessage(`{}`),
				}), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	probe := &PingProbe{Timeout: 5 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := probe.Ping(ctx, conn)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestPingProbe_RequestFormat verifies ping request format.
func TestPingProbe_RequestFormat(t *testing.T) {
	var capturedRequest json.RawMessage

	conn := &mockConn{
		callFunc: func(_ context.Context, msg json.RawMessage) (json.RawMessage, error) {
			capturedRequest = msg
			return mustEncodeResponse(t, &jsonrpc.Response{
				ID:     mustMakeID(t, "ping-1"),
				Result: json.RawMessage(`{}`),
			}), nil
		},
	}

	probe := &PingProbe{Timeout: 2 * time.Second}
	err := probe.Ping(context.Background(), conn)
	require.NoError(t, err)

	// Verify request format
	decoded, err := jsonrpc.DecodeMessage(capturedRequest)
	require.NoError(t, err)

	req, ok := decoded.(*jsonrpc.Request)
	require.True(t, ok, "Expected request message")
	assert.Equal(t, "ping", req.Method)
	assert.NotNil(t, req.ID)
	assert.Equal(t, json.RawMessage(`{}`), req.Params)
}

// Helper functions

func mustMakeID(t *testing.T, id string) jsonrpc.ID {
	t.Helper()
	result, err := jsonrpc.MakeID(id)
	require.NoError(t, err)
	return result
}

func mustEncodeResponse(t *testing.T, resp *jsonrpc.Response) []byte {
	t.Helper()
	data, err := jsonrpc.EncodeMessage(resp)
	require.NoError(t, err)
	return data
}
