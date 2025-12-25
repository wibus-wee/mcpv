package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpd/internal/infra/rpc"
	controlv1 "mcpd/pkg/api/control/v1"
)

type Gateway struct {
	cfg      rpc.ClientConfig
	logger   *zap.Logger
	server   *mcp.Server
	clients  *clientManager
	registry *toolRegistry
}

func NewGateway(cfg rpc.ClientConfig, logger *zap.Logger) *Gateway {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Gateway{
		cfg:    cfg,
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

	g.server = mcp.NewServer(&mcp.Implementation{
		Name:    "mcpd-gateway",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		HasTools: true,
	})

	g.clients = newClientManager(g.cfg)
	g.registry = newToolRegistry(g.server, g.toolHandler, g.logger)

	go g.syncTools(ctx)
	go newLogBridge(g.server, g.clients, g.logger).Run(ctx)

	g.logger.Info("gateway starting (stdio transport)")
	err := g.server.Run(ctx, &mcp.StdioTransport{})
	g.clients.close()
	return err
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

func (g *Gateway) callTool(ctx context.Context, name string, args json.RawMessage) (*controlv1.CallToolResponse, error) {
	client, err := g.clients.get(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := client.Control().CallTool(ctx, &controlv1.CallToolRequest{
		Name:          name,
		ArgumentsJson: args,
	})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			g.clients.reset()
		}
		return nil, err
	}
	if resp == nil || len(resp.ResultJson) == 0 {
		return nil, errors.New("empty call tool response")
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

		resp, err := client.Control().ListTools(ctx, &controlv1.ListToolsRequest{})
		if err != nil {
			g.logger.Warn("rpc list tools failed", zap.Error(err))
			g.clients.reset()
			backoff.Sleep(ctx)
			continue
		}
		if resp != nil && resp.Snapshot != nil {
			g.registry.ApplySnapshot(resp.Snapshot)
			lastETag = resp.Snapshot.Etag
		}

		stream, err := client.Control().WatchTools(ctx, &controlv1.WatchToolsRequest{LastEtag: lastETag})
		if err != nil {
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
