package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
	"mcpd/internal/infra/lifecycle"
	"mcpd/internal/infra/router"
	"mcpd/internal/infra/scheduler"
	"mcpd/internal/infra/transport"
)

type App struct {
	logger *zap.Logger
}

type ServeConfig struct {
	ConfigPath string
}

type ValidateConfig struct {
	ConfigPath string
}

func New(logger *zap.Logger) *App {
	return &App{
		logger: logger.Named("app"),
	}
}

func (a *App) Serve(ctx context.Context, cfg ServeConfig) error {
	loader := catalog.NewLoader(a.logger)

	specs, err := loader.Load(ctx, cfg.ConfigPath)
	if err != nil {
		return err
	}

	a.logger.Info("configuration loaded", zap.String("config", cfg.ConfigPath), zap.Int("servers", len(specs)))

	stdioTransport := transport.NewStdioTransport()
	lc := lifecycle.NewManager(stdioTransport, a.logger)
	sched := scheduler.NewBasicScheduler(lc, specs)
	rt := router.NewBasicRouter(sched)

	sched.StartIdleManager(time.Second)
	defer func() {
		sched.StopIdleManager()
		sched.StopAll(context.Background())
	}()

	if err := a.serveStdin(ctx, rt); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

type stdinRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id"`
	Method  string      `json:"method"`
	Params  stdinParams `json:"params"`
}

type stdinParams struct {
	ServerType string          `json:"serverType"`
	RoutingKey string          `json:"routingKey,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}

func (a *App) serveStdin(ctx context.Context, rt *router.BasicRouter) error {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var req stdinRequest
		errCh := make(chan error, 1)
		go func() {
			if err := dec.Decode(&req); err != nil {
				errCh <- err
				return
			}
			errCh <- nil
		}()

		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return fmt.Errorf("decode stdin: %w", err)
			}
		}

		resp := a.handleRequest(ctx, rt, req)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
	}
}

type stdinResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *stdinError     `json:"error,omitempty"`
}

type stdinError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	errInvalidRequest = -32600
	errRouteFailed    = -32001
)

func (a *App) handleRequest(ctx context.Context, rt *router.BasicRouter, req stdinRequest) stdinResponse {
	if req.JSONRPC != "2.0" || req.Method != "route" {
		return stdinResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &stdinError{
				Code:    errInvalidRequest,
				Message: "invalid request",
			},
		}
	}

	resp, err := rt.Route(ctx, req.Params.ServerType, req.Params.RoutingKey, req.Params.Payload)
	if err != nil {
		return errorResponse(req.ID, mapRouterError(err))
	}

	return stdinResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resp,
	}
}

func mapRouterError(err error) *stdinError {
	switch {
	case errors.Is(err, domain.ErrMethodNotAllowed):
		return &stdinError{Code: -32601, Message: "method not allowed"}
	default:
		return &stdinError{Code: errRouteFailed, Message: err.Error()}
	}
}

func errorResponse(id any, err *stdinError) stdinResponse {
	return stdinResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}
}

func (a *App) ValidateConfig(ctx context.Context, cfg ValidateConfig) error {
	loader := catalog.NewLoader(a.logger)

	specs, err := loader.Load(ctx, cfg.ConfigPath)
	if err != nil {
		return err
	}

	a.logger.Info("configuration validated", zap.String("config", cfg.ConfigPath), zap.Int("servers", len(specs)))
	return nil
}
