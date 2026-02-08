package governance

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcpv/internal/domain"
)

// mockPolicy implements Policy for testing.
type mockPolicy struct {
	name         string
	requestFunc  func(ctx context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error)
	responseFunc func(ctx context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error)
}

func (m *mockPolicy) Request(ctx context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
	if m.requestFunc != nil {
		return m.requestFunc(ctx, req)
	}
	return domain.GovernanceDecision{Continue: true}, nil
}

func (m *mockPolicy) Response(ctx context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
	if m.responseFunc != nil {
		return m.responseFunc(ctx, req)
	}
	return domain.GovernanceDecision{Continue: true}, nil
}

// TestChain_ExecutionOrder verifies chain execution order.
func TestChain_ExecutionOrder(t *testing.T) {
	var executionOrder []string
	var mu sync.Mutex

	policy1 := &mockPolicy{
		name: "policy1",
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "p1-request")
			mu.Unlock()
			return domain.GovernanceDecision{Continue: true}, nil
		},
		responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "p1-response")
			mu.Unlock()
			return domain.GovernanceDecision{Continue: true}, nil
		},
	}

	policy2 := &mockPolicy{
		name: "policy2",
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "p2-request")
			mu.Unlock()
			return domain.GovernanceDecision{Continue: true}, nil
		},
		responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "p2-response")
			mu.Unlock()
			return domain.GovernanceDecision{Continue: true}, nil
		},
	}

	policy3 := &mockPolicy{
		name: "policy3",
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "p3-request")
			mu.Unlock()
			return domain.GovernanceDecision{Continue: true}, nil
		},
		responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "p3-response")
			mu.Unlock()
			return domain.GovernanceDecision{Continue: true}, nil
		},
	}

	chain := NewChain(policy1, policy2, policy3)
	req := domain.GovernanceRequest{Method: "test"}

	t.Run("request policies execute forward", func(t *testing.T) {
		executionOrder = nil
		_, err := chain.Request(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, []string{"p1-request", "p2-request", "p3-request"}, executionOrder)
	})

	t.Run("response policies execute backward", func(t *testing.T) {
		executionOrder = nil
		_, err := chain.Response(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, []string{"p3-response", "p2-response", "p1-response"}, executionOrder)
	})

	t.Run("execute runs request forward then response backward", func(t *testing.T) {
		executionOrder = nil
		nextCalled := false
		next := func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "next")
			nextCalled = true
			mu.Unlock()
			return json.RawMessage(`{"result":"ok"}`), nil
		}

		_, err := chain.Execute(context.Background(), req, next)
		require.NoError(t, err)
		assert.True(t, nextCalled)
		assert.Equal(t, []string{
			"p1-request", "p2-request", "p3-request",
			"next",
			"p3-response", "p2-response", "p1-response",
		}, executionOrder)
	})
}

// TestChain_EarlyRejection verifies early rejection stops chain.
func TestChain_EarlyRejection(t *testing.T) {
	tests := []struct {
		name           string
		rejectAt       int // which policy rejects (0-indexed)
		totalPolicies  int
		calledPolicies []string
	}{
		{
			name:           "first policy rejects, others not called",
			rejectAt:       0,
			totalPolicies:  3,
			calledPolicies: []string{"p0"},
		},
		{
			name:           "middle policy rejects, later not called",
			rejectAt:       1,
			totalPolicies:  3,
			calledPolicies: []string{"p0", "p1"},
		},
		{
			name:           "last policy rejects",
			rejectAt:       2,
			totalPolicies:  3,
			calledPolicies: []string{"p0", "p1", "p2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calledPolicies []string
			var mu sync.Mutex

			policies := make([]Policy, tt.totalPolicies)
			for i := 0; i < tt.totalPolicies; i++ {
				idx := i
				policies[i] = &mockPolicy{
					name: "p" + string(rune('0'+i)),
					requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
						mu.Lock()
						calledPolicies = append(calledPolicies, "p"+string(rune('0'+idx)))
						mu.Unlock()

						if idx == tt.rejectAt {
							return domain.GovernanceDecision{
								Continue:      false,
								RejectCode:    "REJECTED",
								RejectMessage: "Policy rejected",
							}, nil
						}
						return domain.GovernanceDecision{Continue: true}, nil
					},
				}
			}

			chain := NewChain(policies...)
			req := domain.GovernanceRequest{Method: "test"}

			decision, err := chain.Request(context.Background(), req)
			require.NoError(t, err)
			assert.False(t, decision.Continue)
			assert.Equal(t, "REJECTED", decision.RejectCode)
			assert.Equal(t, tt.calledPolicies, calledPolicies)
		})
	}
}

// TestChain_RequestMutation verifies request mutation propagation.
func TestChain_RequestMutation(t *testing.T) {
	policy1 := &mockPolicy{
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{
				Continue:    true,
				RequestJSON: json.RawMessage(`{"step":1}`),
			}, nil
		},
	}

	policy2 := &mockPolicy{
		requestFunc: func(_ context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			// Verify we received the mutation from policy1
			assert.Equal(t, json.RawMessage(`{"step":1}`), req.RequestJSON)
			return domain.GovernanceDecision{
				Continue:    true,
				RequestJSON: json.RawMessage(`{"step":2}`),
			}, nil
		},
	}

	policy3 := &mockPolicy{
		requestFunc: func(_ context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			// Verify we received the mutation from policy2
			assert.Equal(t, json.RawMessage(`{"step":2}`), req.RequestJSON)
			// Return a decision with RequestJSON to verify it's in the final decision
			return domain.GovernanceDecision{
				Continue:    true,
				RequestJSON: json.RawMessage(`{"step":3}`),
			}, nil
		},
	}

	chain := NewChain(policy1, policy2, policy3)
	req := domain.GovernanceRequest{
		Method:      "test",
		RequestJSON: json.RawMessage(`{"step":0}`),
	}

	decision, err := chain.Request(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, decision.Continue)
	// The final decision should have the RequestJSON from the last policy
	assert.Equal(t, json.RawMessage(`{"step":3}`), decision.RequestJSON)
}

// TestChain_ResponseMutation verifies response mutation propagation.
func TestChain_ResponseMutation(t *testing.T) {
	policy1 := &mockPolicy{
		responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			// First to execute in response chain (last in list)
			return domain.GovernanceDecision{
				Continue:     true,
				ResponseJSON: json.RawMessage(`{"step":1}`),
			}, nil
		},
	}

	policy2 := &mockPolicy{
		responseFunc: func(_ context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			// Verify we received the mutation from policy1
			assert.Equal(t, json.RawMessage(`{"step":1}`), req.ResponseJSON)
			return domain.GovernanceDecision{
				Continue:     true,
				ResponseJSON: json.RawMessage(`{"step":2}`),
			}, nil
		},
	}

	chain := NewChain(policy2, policy1) // Note: reversed order for response
	req := domain.GovernanceRequest{
		Method:       "test",
		ResponseJSON: json.RawMessage(`{"step":0}`),
	}

	decision, err := chain.Response(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, decision.Continue)
	assert.Equal(t, json.RawMessage(`{"step":2}`), decision.ResponseJSON)
}

// TestChain_NilHandling verifies nil policy/chain handling.
func TestChain_NilHandling(t *testing.T) {
	t.Run("nil chain returns Continue", func(t *testing.T) {
		var chain *Chain
		req := domain.GovernanceRequest{Method: "test"}

		decision, err := chain.Request(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)

		decision, err = chain.Response(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
	})

	t.Run("empty chain returns Continue", func(t *testing.T) {
		chain := NewChain()
		req := domain.GovernanceRequest{Method: "test"}

		decision, err := chain.Request(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)

		decision, err = chain.Response(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
	})

	t.Run("chain with nil policies filters them out", func(t *testing.T) {
		policy := &mockPolicy{
			requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				return domain.GovernanceDecision{
					Continue:    true,
					RequestJSON: json.RawMessage(`{"called":true}`),
				}, nil
			},
		}

		chain := NewChain(nil, policy, nil)
		req := domain.GovernanceRequest{Method: "test"}

		decision, err := chain.Request(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
		assert.Equal(t, json.RawMessage(`{"called":true}`), decision.RequestJSON)
	})

	t.Run("nil context uses Background", func(t *testing.T) {
		policy := &mockPolicy{
			requestFunc: func(ctx context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				assert.NotNil(t, ctx)
				return domain.GovernanceDecision{Continue: true}, nil
			},
		}

		chain := NewChain(policy)
		req := domain.GovernanceRequest{Method: "test"}

		_, err := chain.Request(nil, req) //nolint:staticcheck
		require.NoError(t, err)
	})
}

// TestChain_ErrorHandling verifies error propagation.
func TestChain_ErrorHandling(t *testing.T) {
	testErr := errors.New("policy error")

	t.Run("request error stops chain", func(t *testing.T) {
		policy1 := &mockPolicy{
			requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				return domain.GovernanceDecision{}, testErr
			},
		}

		policy2 := &mockPolicy{
			requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				t.Fatal("policy2 should not be called")
				return domain.GovernanceDecision{Continue: true}, nil
			},
		}

		chain := NewChain(policy1, policy2)
		req := domain.GovernanceRequest{Method: "test"}

		_, err := chain.Request(context.Background(), req)
		assert.ErrorIs(t, err, testErr)
	})

	t.Run("response error stops chain", func(t *testing.T) {
		policy1 := &mockPolicy{
			responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				return domain.GovernanceDecision{}, testErr
			},
		}

		policy2 := &mockPolicy{
			responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				t.Fatal("policy2 should not be called")
				return domain.GovernanceDecision{Continue: true}, nil
			},
		}

		chain := NewChain(policy2, policy1) // policy1 executes first in response
		req := domain.GovernanceRequest{Method: "test"}

		_, err := chain.Response(context.Background(), req)
		assert.ErrorIs(t, err, testErr)
	})
}

// TestChain_ConcurrentExecute verifies thread-safe execution.
func TestChain_ConcurrentExecute(t *testing.T) {
	const goroutines = 100

	policy := &mockPolicy{
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{Continue: true}, nil
		},
		responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{Continue: true}, nil
		},
	}

	chain := NewChain(policy)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	errors := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			req := domain.GovernanceRequest{Method: "test"}
			next := func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
				return json.RawMessage(`{"ok":true}`), nil
			}
			_, errors[idx] = chain.Execute(context.Background(), req, next)
		}(i)
	}

	wg.Wait()

	for i, err := range errors {
		assert.NoError(t, err, "Execution %d failed", i)
	}
}

// TestHandleRejection_ToolCall verifies tool call rejection formatting.
func TestHandleRejection_ToolCall(t *testing.T) {
	req := domain.GovernanceRequest{
		Method: "tools/call",
	}

	decision := domain.GovernanceDecision{
		Continue:      false,
		RejectCode:    "FORBIDDEN",
		RejectMessage: "Tool access denied",
	}

	result, err := handleRejection(req, decision)
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Verify it's a valid CallToolResult
	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)
	assert.True(t, parsed["isError"].(bool))
	assert.NotNil(t, parsed["content"])
	assert.NotNil(t, parsed["structuredContent"])
}

// TestHandleRejection_OtherMethods verifies non-tool rejection formatting.
func TestHandleRejection_OtherMethods(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"resources/read", "resources/read"},
		{"prompts/get", "prompts/get"},
		{"custom/method", "custom/method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := domain.GovernanceRequest{
				Method: tt.method,
			}

			decision := domain.GovernanceDecision{
				Continue:      false,
				RejectCode:    "FORBIDDEN",
				RejectMessage: "Access denied",
			}

			result, err := handleRejection(req, decision)
			assert.Nil(t, result)
			assert.Error(t, err)

			var govErr domain.GovernanceRejection
			assert.ErrorAs(t, err, &govErr)
			assert.Equal(t, "FORBIDDEN", govErr.Code)
			assert.Equal(t, "Access denied", govErr.Message)
		})
	}
}

// TestExecutor_NilChain verifies executor with nil chain.
func TestExecutor_NilChain(t *testing.T) {
	executor := &Executor{}
	req := domain.GovernanceRequest{Method: "test"}

	t.Run("request returns Continue", func(t *testing.T) {
		decision, err := executor.Request(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
	})

	t.Run("response returns Continue", func(t *testing.T) {
		decision, err := executor.Response(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
	})

	t.Run("execute calls next", func(t *testing.T) {
		nextCalled := false
		next := func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
			nextCalled = true
			return json.RawMessage(`{"ok":true}`), nil
		}

		result, err := executor.Execute(context.Background(), req, next)
		require.NoError(t, err)
		assert.True(t, nextCalled)
		assert.Equal(t, json.RawMessage(`{"ok":true}`), result)
	})
}

// TestNewExecutor verifies executor construction.
func TestNewExecutor(t *testing.T) {
	t.Run("nil pipeline creates empty executor", func(t *testing.T) {
		executor := NewExecutor(nil)
		assert.NotNil(t, executor)
		assert.Nil(t, executor.chain)
	})

	t.Run("with policies creates executor with chain", func(t *testing.T) {
		policy := &mockPolicy{}
		executor := NewExecutorWithPolicies(policy)
		assert.NotNil(t, executor)
		assert.NotNil(t, executor.chain)
	})
}

// TestExecute_FullFlow verifies complete execute flow with mutations.
func TestExecute_FullFlow(t *testing.T) {
	policy1 := &mockPolicy{
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{
				Continue:    true,
				RequestJSON: json.RawMessage(`{"modified":"request"}`),
			}, nil
		},
		responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{
				Continue:     true,
				ResponseJSON: json.RawMessage(`{"modified":"response"}`),
			}, nil
		},
	}

	chain := NewChain(policy1)
	req := domain.GovernanceRequest{
		Method:      "test",
		RequestJSON: json.RawMessage(`{"original":"request"}`),
	}

	nextCalled := false
	var capturedRequest domain.GovernanceRequest
	next := func(_ context.Context, req domain.GovernanceRequest) (json.RawMessage, error) {
		nextCalled = true
		capturedRequest = req
		return json.RawMessage(`{"original":"response"}`), nil
	}

	result, err := chain.Execute(context.Background(), req, next)
	require.NoError(t, err)
	assert.True(t, nextCalled)
	// Verify request was modified before calling next
	assert.Equal(t, json.RawMessage(`{"modified":"request"}`), capturedRequest.RequestJSON)
	// Verify response was modified
	assert.Equal(t, json.RawMessage(`{"modified":"response"}`), result)
}

// TestExecute_RequestRejection verifies rejection during request phase.
func TestExecute_RequestRejection(t *testing.T) {
	policy := &mockPolicy{
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{
				Continue:      false,
				RejectCode:    "BLOCKED",
				RejectMessage: "Request blocked",
			}, nil
		},
	}

	chain := NewChain(policy)
	req := domain.GovernanceRequest{Method: "resources/read"}

	next := func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
		t.Fatal("next should not be called")
		return nil, nil
	}

	result, err := chain.Execute(context.Background(), req, next)
	assert.Nil(t, result)
	assert.Error(t, err)

	var govErr domain.GovernanceRejection
	assert.ErrorAs(t, err, &govErr)
	assert.Equal(t, "BLOCKED", govErr.Code)
}

// TestExecute_ResponseRejection verifies rejection during response phase.
func TestExecute_ResponseRejection(t *testing.T) {
	policy := &mockPolicy{
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{Continue: true}, nil
		},
		responseFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{
				Continue:      false,
				RejectCode:    "BLOCKED",
				RejectMessage: "Response blocked",
			}, nil
		},
	}

	chain := NewChain(policy)
	req := domain.GovernanceRequest{Method: "tools/call"}

	nextCalled := false
	next := func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
		nextCalled = true
		return json.RawMessage(`{"result":"ok"}`), nil
	}

	result, err := chain.Execute(context.Background(), req, next)
	assert.True(t, nextCalled) // next is called before response rejection
	assert.NotNil(t, result)   // tool call returns structured error
	assert.NoError(t, err)     // tool call rejection is not an error

	// Verify it's a CallToolResult with error
	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)
	assert.True(t, parsed["isError"].(bool))
}

// TestExecute_NextError verifies error from next function.
func TestExecute_NextError(t *testing.T) {
	policy := &mockPolicy{
		requestFunc: func(_ context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			return domain.GovernanceDecision{Continue: true}, nil
		},
	}

	chain := NewChain(policy)
	req := domain.GovernanceRequest{Method: "test"}

	testErr := errors.New("next error")
	next := func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
		return nil, testErr
	}

	result, err := chain.Execute(context.Background(), req, next)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, testErr)
}

// TestExecute_NilChain verifies execute with nil chain.
func TestExecute_NilChain(t *testing.T) {
	var chain *Chain
	req := domain.GovernanceRequest{Method: "test"}

	nextCalled := false
	next := func(_ context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
		nextCalled = true
		return json.RawMessage(`{"ok":true}`), nil
	}

	result, err := chain.Execute(context.Background(), req, next)
	require.NoError(t, err)
	assert.True(t, nextCalled)
	assert.Equal(t, json.RawMessage(`{"ok":true}`), result)
}

// TestBuildToolRejection_EmptyMessage verifies default message.
func TestBuildToolRejection_EmptyMessage(t *testing.T) {
	decision := domain.GovernanceDecision{
		Continue:      false,
		RejectCode:    "ERROR",
		RejectMessage: "", // Empty message
	}

	result, err := buildToolRejection(decision)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	// Verify default message is used
	content := parsed["content"].([]any)[0].(map[string]any)
	assert.Equal(t, "request rejected", content["text"])
}

// TestPipelinePolicy_NilHandling verifies nil pipeline handling.
func TestPipelinePolicy_NilHandling(t *testing.T) {
	t.Run("nil policy returns Continue", func(t *testing.T) {
		var policy *PipelinePolicy
		req := domain.GovernanceRequest{Method: "test"}

		decision, err := policy.Request(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)

		decision, err = policy.Response(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
	})

	t.Run("policy with nil pipeline returns Continue", func(t *testing.T) {
		policy := &PipelinePolicy{}
		req := domain.GovernanceRequest{Method: "test"}

		decision, err := policy.Request(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)

		decision, err = policy.Response(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, decision.Continue)
	})

	t.Run("nil context uses Background", func(t *testing.T) {
		policy := &PipelinePolicy{}
		req := domain.GovernanceRequest{Method: "test"}

		_, err := policy.Request(nil, req) //nolint:staticcheck
		require.NoError(t, err)

		_, err = policy.Response(context.Background(), req)
		require.NoError(t, err)
	})
}

// TestExecutor_NilContext verifies nil context handling.
func TestExecutor_NilContext(t *testing.T) {
	policy := &mockPolicy{
		requestFunc: func(ctx context.Context, _ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
			assert.NotNil(t, ctx)
			return domain.GovernanceDecision{Continue: true}, nil
		},
	}

	executor := NewExecutorWithPolicies(policy)
	req := domain.GovernanceRequest{Method: "test"}

	_, err := executor.Request(nil, req) //nolint:staticcheck
	require.NoError(t, err)

	_, err = executor.Response(nil, req) //nolint:staticcheck
	require.NoError(t, err)

	next := func(ctx context.Context, _ domain.GovernanceRequest) (json.RawMessage, error) {
		assert.NotNil(t, ctx)
		return json.RawMessage(`{"ok":true}`), nil
	}
	_, err = executor.Execute(nil, req, next) //nolint:staticcheck
	require.NoError(t, err)
}
