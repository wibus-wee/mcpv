package pipeline

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"mcpv/internal/domain"
)

type fakeHandler struct {
	mu        sync.Mutex
	responses map[string]func(domain.GovernanceRequest) (domain.GovernanceDecision, error)
	seen      map[string][]domain.GovernanceRequest
}

func (f *fakeHandler) Handle(_ context.Context, spec domain.PluginSpec, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
	f.mu.Lock()
	if f.seen == nil {
		f.seen = make(map[string][]domain.GovernanceRequest)
	}
	f.seen[spec.Name] = append(f.seen[spec.Name], req)
	fn := f.responses[spec.Name]
	f.mu.Unlock()
	if fn == nil {
		return domain.GovernanceDecision{Continue: true}, nil
	}
	return fn(req)
}

func TestEngine_ContentMutationFlowsToNextPlugin(t *testing.T) {
	handler := &fakeHandler{
		responses: map[string]func(domain.GovernanceRequest) (domain.GovernanceDecision, error){
			"content": func(_ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				return domain.GovernanceDecision{
					Continue:    true,
					RequestJSON: json.RawMessage(`{"foo":"bar"}`),
				}, nil
			},
			"audit": func(_ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				return domain.GovernanceDecision{Continue: true}, nil
			},
		},
	}

	engine := NewEngine(handler, nil, nil)
	engine.Update([]domain.PluginSpec{
		{Name: "content", Category: domain.PluginCategoryContent, Required: true},
		{Name: "audit", Category: domain.PluginCategoryAudit, Required: true},
	})

	decision, err := engine.Handle(context.Background(), domain.GovernanceRequest{
		Flow:        domain.PluginFlowRequest,
		Method:      "tools/call",
		RequestJSON: json.RawMessage(`{"foo":"old"}`),
	})
	require.NoError(t, err)
	require.True(t, decision.Continue)

	handler.mu.Lock()
	seen := handler.seen["audit"]
	handler.mu.Unlock()
	require.Len(t, seen, 1)
	require.JSONEq(t, `{"foo":"bar"}`, string(seen[0].RequestJSON))
}

func TestEngine_OptionalRejectionBlocksNonObservability(t *testing.T) {
	handler := &fakeHandler{
		responses: map[string]func(domain.GovernanceRequest) (domain.GovernanceDecision, error){
			"authz": func(_ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				return domain.GovernanceDecision{Continue: false, RejectMessage: "nope"}, nil
			},
		},
	}

	engine := NewEngine(handler, nil, nil)
	engine.Update([]domain.PluginSpec{
		{Name: "authz", Category: domain.PluginCategoryAuthorization, Required: false},
	})

	decision, err := engine.Handle(context.Background(), domain.GovernanceRequest{Flow: domain.PluginFlowRequest, Method: "tools/list"})
	require.NoError(t, err)
	require.False(t, decision.Continue)
}

func TestEngine_ObservabilityIgnoresOptionalRejection(t *testing.T) {
	handler := &fakeHandler{
		responses: map[string]func(domain.GovernanceRequest) (domain.GovernanceDecision, error){
			"obs": func(_ domain.GovernanceRequest) (domain.GovernanceDecision, error) {
				return domain.GovernanceDecision{Continue: false, RejectMessage: "ignored"}, nil
			},
		},
	}

	engine := NewEngine(handler, nil, nil)
	engine.Update([]domain.PluginSpec{
		{Name: "obs", Category: domain.PluginCategoryObservability, Required: false},
	})

	decision, err := engine.Handle(context.Background(), domain.GovernanceRequest{Flow: domain.PluginFlowRequest, Method: "tools/list"})
	require.NoError(t, err)
	require.True(t, decision.Continue)
}
