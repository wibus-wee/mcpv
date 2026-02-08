package gateway

import (
	"context"
	"encoding/json"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (g *Gateway) callTool(ctx context.Context, name string, args json.RawMessage) (*controlv1.CallToolResponse, error) {
	client, err := g.clients.get(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := client.Control().CallTool(ctx, &controlv1.CallToolRequest{
		Caller:        g.caller,
		Name:          name,
		ArgumentsJson: args,
	})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			if regErr := g.registerCaller(ctx); regErr == nil {
				resp, err = client.Control().CallTool(ctx, &controlv1.CallToolRequest{
					Caller:        g.caller,
					Name:          name,
					ArgumentsJson: args,
				})
			}
		}
		if err != nil {
			if status.Code(err) == codes.Unavailable {
				g.clients.reset()
			}
			return nil, err
		}
	}
	if resp == nil || len(resp.GetResultJson()) == 0 {
		return nil, domain.Wrap(domain.CodeInternal, "gateway call tool", errors.New("empty call tool response"))
	}
	return resp, nil
}

func (g *Gateway) getPrompt(ctx context.Context, name string, args json.RawMessage) (*controlv1.GetPromptResponse, error) {
	client, err := g.clients.get(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := client.Control().GetPrompt(ctx, &controlv1.GetPromptRequest{
		Caller:        g.caller,
		Name:          name,
		ArgumentsJson: args,
	})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			if regErr := g.registerCaller(ctx); regErr == nil {
				resp, err = client.Control().GetPrompt(ctx, &controlv1.GetPromptRequest{
					Caller:        g.caller,
					Name:          name,
					ArgumentsJson: args,
				})
			}
		}
		if err != nil {
			if status.Code(err) == codes.Unavailable {
				g.clients.reset()
			}
			return nil, err
		}
	}
	if resp == nil || len(resp.GetResultJson()) == 0 {
		return nil, domain.Wrap(domain.CodeInternal, "gateway get prompt", errors.New("empty get prompt response"))
	}
	return resp, nil
}

func (g *Gateway) readResource(ctx context.Context, uri string) (*controlv1.ReadResourceResponse, error) {
	client, err := g.clients.get(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := client.Control().ReadResource(ctx, &controlv1.ReadResourceRequest{
		Caller: g.caller,
		Uri:    uri,
	})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			if regErr := g.registerCaller(ctx); regErr == nil {
				resp, err = client.Control().ReadResource(ctx, &controlv1.ReadResourceRequest{
					Caller: g.caller,
					Uri:    uri,
				})
			}
		}
		if err != nil {
			if status.Code(err) == codes.Unavailable {
				g.clients.reset()
			}
			return nil, err
		}
	}
	if resp == nil || len(resp.GetResultJson()) == 0 {
		return nil, domain.Wrap(domain.CodeInternal, "gateway read resource", errors.New("empty read resource response"))
	}
	return resp, nil
}
