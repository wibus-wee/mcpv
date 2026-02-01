package governance

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcpv/internal/domain"
	"mcpv/internal/infra/pipeline"
)

type Executor struct {
	pipeline *pipeline.Engine
}

func NewExecutor(pipe *pipeline.Engine) *Executor {
	return &Executor{pipeline: pipe}
}

func (e *Executor) Request(ctx context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e.pipeline == nil {
		return domain.GovernanceDecision{Continue: true}, nil
	}
	req.Flow = domain.PluginFlowRequest
	return e.pipeline.Handle(ctx, req)
}

func (e *Executor) Response(ctx context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e.pipeline == nil {
		return domain.GovernanceDecision{Continue: true}, nil
	}
	req.Flow = domain.PluginFlowResponse
	return e.pipeline.Handle(ctx, req)
}

func (e *Executor) Execute(ctx context.Context, req domain.GovernanceRequest, next func(context.Context, domain.GovernanceRequest) (json.RawMessage, error)) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e.pipeline == nil {
		return next(ctx, req)
	}

	request := req
	request.Flow = domain.PluginFlowRequest
	decision, err := e.pipeline.Handle(ctx, request)
	if err != nil {
		return nil, err
	}
	if !decision.Continue {
		return handleRejection(req, decision)
	}
	if len(decision.RequestJSON) > 0 {
		request.RequestJSON = decision.RequestJSON
	}

	resp, err := next(ctx, request)
	if err != nil {
		return nil, err
	}

	responseReq := request
	responseReq.Flow = domain.PluginFlowResponse
	responseReq.ResponseJSON = resp

	responseDecision, err := e.pipeline.Handle(ctx, responseReq)
	if err != nil {
		return nil, err
	}
	if !responseDecision.Continue {
		return handleRejection(req, responseDecision)
	}
	if len(responseDecision.ResponseJSON) > 0 {
		resp = responseDecision.ResponseJSON
	}

	return resp, nil
}

func handleRejection(req domain.GovernanceRequest, decision domain.GovernanceDecision) (json.RawMessage, error) {
	if req.Method == "tools/call" {
		return buildToolRejection(decision)
	}
	return nil, domain.GovernanceRejection{
		Category: decision.Category,
		Plugin:   decision.Plugin,
		Code:     decision.RejectCode,
		Message:  decision.RejectMessage,
	}
}

func buildToolRejection(decision domain.GovernanceDecision) (json.RawMessage, error) {
	message := decision.RejectMessage
	if message == "" {
		message = "request rejected"
	}

	structured := map[string]any{
		"code":    decision.RejectCode,
		"message": message,
	}
	result := mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
		StructuredContent: structured,
	}
	payload, err := json.Marshal(&result)
	if err != nil {
		return nil, fmt.Errorf("encode rejection: %w", err)
	}
	return payload, nil
}
