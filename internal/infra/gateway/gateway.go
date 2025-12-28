package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpd/internal/infra/rpc"
	controlv1 "mcpd/pkg/api/control/v1"
)

type Gateway struct {
	cfg        rpc.ClientConfig
	caller     string
	logger     *zap.Logger
	server     *mcp.Server
	clients    *clientManager
	registry   *toolRegistry
	resources  *resourceRegistry
	prompts    *promptRegistry
	registered atomic.Bool
}

const defaultHeartbeatInterval = 2 * time.Second

func NewGateway(cfg rpc.ClientConfig, caller string, logger *zap.Logger) *Gateway {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Gateway{
		cfg:    cfg,
		caller: caller,
		logger: logger.Named("gateway"),
	}
}

func (g *Gateway) Run(ctx context.Context) error {
	if g.cfg.Address == "" {
		return errors.New("rpc address is required")
	}
	if g.cfg.MaxRecvMsgSize <= 0 {
		return errors.New("rpc max recv message size must be > 0")
	}
	if g.cfg.MaxSendMsgSize <= 0 {
		return errors.New("rpc max send message size must be > 0")
	}
	if g.caller == "" {
		return errors.New("caller is required")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	g.server = mcp.NewServer(&mcp.Implementation{
		Name:    "mcpd-gateway",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		HasTools:     true,
		HasResources: true,
		HasPrompts:   true,
	})

	g.clients = newClientManager(g.cfg)
	g.registry = newToolRegistry(g.server, g.toolHandler, g.logger)
	g.resources = newResourceRegistry(g.server, g.resourceHandler, g.logger)
	g.prompts = newPromptRegistry(g.server, g.promptHandler, g.logger)

	if err := g.registerCaller(runCtx); err != nil {
		return err
	}
	defer func() {
		cancel()
		_ = g.unregisterCaller(context.Background())
	}()

	go g.heartbeat(runCtx)
	go g.syncTools(runCtx)
	go g.syncResources(runCtx)
	go g.syncPrompts(runCtx)
	go newLogBridge(g.server, g.clients, g.caller, g.logger).Run(runCtx)

	g.logger.Info("gateway starting (stdio transport)")
	err := g.server.Run(runCtx, &mcp.StdioTransport{})
	g.clients.close()
	return err
}

func (g *Gateway) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(defaultHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := g.registerCaller(ctx); err != nil {
				g.logger.Warn("caller heartbeat failed", zap.Error(err))
			}
		}
	}
}

func (g *Gateway) registerCaller(ctx context.Context) error {
	client, err := g.clients.get(ctx)
	if err != nil {
		return err
	}
	resp, err := client.Control().RegisterCaller(ctx, &controlv1.RegisterCallerRequest{
		Caller: g.caller,
		Pid:    int64(os.Getpid()),
	})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			g.clients.reset()
		}
		return err
	}
	if !g.registered.Swap(true) && resp != nil && resp.Profile != "" {
		g.logger.Info("caller registered", zap.String("profile", resp.Profile))
	}
	return nil
}

func (g *Gateway) unregisterCaller(ctx context.Context) error {
	client, err := g.clients.get(ctx)
	if err != nil {
		return err
	}
	_, err = client.Control().UnregisterCaller(ctx, &controlv1.UnregisterCallerRequest{
		Caller: g.caller,
	})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			g.clients.reset()
		}
		return err
	}
	return nil
}

func (g *Gateway) toolHandler(name string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := json.RawMessage(req.Params.Arguments)
		resp, err := g.callTool(ctx, name, args)
		if err != nil {
			return nil, err
		}
		var result mcp.CallToolResult
		if err := json.Unmarshal(resp.ResultJson, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}
}

func (g *Gateway) promptHandler(name string) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var args json.RawMessage
		if req != nil && req.Params != nil {
			raw, err := json.Marshal(req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			args = raw
		}
		resp, err := g.getPrompt(ctx, name, args)
		if err != nil {
			return nil, err
		}
		var result mcp.GetPromptResult
		if err := json.Unmarshal(resp.ResultJson, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}
}

func (g *Gateway) resourceHandler(uri string) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		targetURI := uri
		if req != nil && req.Params != nil && req.Params.URI != "" {
			targetURI = req.Params.URI
		}
		resp, err := g.readResource(ctx, targetURI)
		if err != nil {
			return nil, err
		}
		var result mcp.ReadResourceResult
		if err := json.Unmarshal(resp.ResultJson, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}
}

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
	if resp == nil || len(resp.ResultJson) == 0 {
		return nil, errors.New("empty call tool response")
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
	if resp == nil || len(resp.ResultJson) == 0 {
		return nil, errors.New("empty get prompt response")
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
	if resp == nil || len(resp.ResultJson) == 0 {
		return nil, errors.New("empty read resource response")
	}
	return resp, nil
}

func (g *Gateway) syncTools(ctx context.Context) {
	backoff := newBackoff(1*time.Second, 30*time.Second)
	lastETag := ""

	for {
		if ctx.Err() != nil {
			return
		}

		client, err := g.clients.get(ctx)
		if err != nil {
			g.logger.Warn("rpc connect failed", zap.Error(err))
			backoff.Sleep(ctx)
			continue
		}

		resp, err := client.Control().ListTools(ctx, &controlv1.ListToolsRequest{Caller: g.caller})
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				if regErr := g.registerCaller(ctx); regErr == nil {
					continue
				}
			}
			g.logger.Warn("rpc list tools failed", zap.Error(err))
			g.clients.reset()
			backoff.Sleep(ctx)
			continue
		}
		if resp != nil && resp.Snapshot != nil {
			g.registry.ApplySnapshot(resp.Snapshot)
			lastETag = resp.Snapshot.Etag
		}

		stream, err := client.Control().WatchTools(ctx, &controlv1.WatchToolsRequest{
			Caller:   g.caller,
			LastEtag: lastETag,
		})
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				if regErr := g.registerCaller(ctx); regErr == nil {
					continue
				}
			}
			g.logger.Warn("rpc watch tools failed", zap.Error(err))
			g.clients.reset()
			backoff.Sleep(ctx)
			continue
		}

		backoff.Reset()

		for {
			snapshot, err := stream.Recv()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if status.Code(err) == codes.Canceled {
					return
				}
				g.logger.Warn("rpc tool watch interrupted", zap.Error(err))
				g.clients.reset()
				backoff.Sleep(ctx)
				break
			}
			if snapshot != nil {
				g.registry.ApplySnapshot(snapshot)
				lastETag = snapshot.Etag
			}
		}
	}
}

func (g *Gateway) syncResources(ctx context.Context) {
	backoff := newBackoff(1*time.Second, 30*time.Second)
	lastETag := ""

	for {
		if ctx.Err() != nil {
			return
		}

		client, err := g.clients.get(ctx)
		if err != nil {
			g.logger.Warn("rpc connect failed", zap.Error(err))
			backoff.Sleep(ctx)
			continue
		}

		snapshot, err := g.listAllResources(ctx, client)
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				if regErr := g.registerCaller(ctx); regErr == nil {
					continue
				}
			}
			g.logger.Warn("rpc list resources failed", zap.Error(err))
			g.clients.reset()
			backoff.Sleep(ctx)
			continue
		}
		if snapshot != nil {
			g.resources.ApplySnapshot(snapshot)
			lastETag = snapshot.Etag
		}

		stream, err := client.Control().WatchResources(ctx, &controlv1.WatchResourcesRequest{
			Caller:   g.caller,
			LastEtag: lastETag,
		})
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				if regErr := g.registerCaller(ctx); regErr == nil {
					continue
				}
			}
			g.logger.Warn("rpc watch resources failed", zap.Error(err))
			g.clients.reset()
			backoff.Sleep(ctx)
			continue
		}

		backoff.Reset()

		for {
			snapshot, err := stream.Recv()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if status.Code(err) == codes.Canceled {
					return
				}
				g.logger.Warn("rpc resource watch interrupted", zap.Error(err))
				g.clients.reset()
				backoff.Sleep(ctx)
				break
			}
			if snapshot != nil {
				g.resources.ApplySnapshot(snapshot)
				lastETag = snapshot.Etag
			}
		}
	}
}

func (g *Gateway) syncPrompts(ctx context.Context) {
	backoff := newBackoff(1*time.Second, 30*time.Second)
	lastETag := ""

	for {
		if ctx.Err() != nil {
			return
		}

		client, err := g.clients.get(ctx)
		if err != nil {
			g.logger.Warn("rpc connect failed", zap.Error(err))
			backoff.Sleep(ctx)
			continue
		}

		snapshot, err := g.listAllPrompts(ctx, client)
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				if regErr := g.registerCaller(ctx); regErr == nil {
					continue
				}
			}
			g.logger.Warn("rpc list prompts failed", zap.Error(err))
			g.clients.reset()
			backoff.Sleep(ctx)
			continue
		}
		if snapshot != nil {
			g.prompts.ApplySnapshot(snapshot)
			lastETag = snapshot.Etag
		}

		stream, err := client.Control().WatchPrompts(ctx, &controlv1.WatchPromptsRequest{
			Caller:   g.caller,
			LastEtag: lastETag,
		})
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				if regErr := g.registerCaller(ctx); regErr == nil {
					continue
				}
			}
			g.logger.Warn("rpc watch prompts failed", zap.Error(err))
			g.clients.reset()
			backoff.Sleep(ctx)
			continue
		}

		backoff.Reset()

		for {
			snapshot, err := stream.Recv()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if status.Code(err) == codes.Canceled {
					return
				}
				g.logger.Warn("rpc prompt watch interrupted", zap.Error(err))
				g.clients.reset()
				backoff.Sleep(ctx)
				break
			}
			if snapshot != nil {
				g.prompts.ApplySnapshot(snapshot)
				lastETag = snapshot.Etag
			}
		}
	}
}

func (g *Gateway) listAllResources(ctx context.Context, client *rpc.Client) (*controlv1.ResourcesSnapshot, error) {
	cursor := ""
	var combined []*controlv1.ResourceDefinition
	etag := ""
	etagSet := false

	for {
		resp, err := client.Control().ListResources(ctx, &controlv1.ListResourcesRequest{
			Caller: g.caller,
			Cursor: cursor,
		})
		if err != nil {
			return nil, err
		}
		if resp != nil && resp.Snapshot != nil {
			pageETag := resp.Snapshot.Etag
			if !etagSet {
				etag = pageETag
				etagSet = true
			} else if pageETag != etag {
				return nil, errors.New("resource snapshot changed during pagination")
			}
			combined = append(combined, resp.Snapshot.Resources...)
		}
		if resp == nil || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}

	return &controlv1.ResourcesSnapshot{
		Etag:      etag,
		Resources: combined,
	}, nil
}

func (g *Gateway) listAllPrompts(ctx context.Context, client *rpc.Client) (*controlv1.PromptsSnapshot, error) {
	cursor := ""
	var combined []*controlv1.PromptDefinition
	etag := ""
	etagSet := false

	for {
		resp, err := client.Control().ListPrompts(ctx, &controlv1.ListPromptsRequest{
			Caller: g.caller,
			Cursor: cursor,
		})
		if err != nil {
			return nil, err
		}
		if resp != nil && resp.Snapshot != nil {
			pageETag := resp.Snapshot.Etag
			if !etagSet {
				etag = pageETag
				etagSet = true
			} else if pageETag != etag {
				return nil, errors.New("prompt snapshot changed during pagination")
			}
			combined = append(combined, resp.Snapshot.Prompts...)
		}
		if resp == nil || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}

	return &controlv1.PromptsSnapshot{
		Etag:    etag,
		Prompts: combined,
	}, nil
}
