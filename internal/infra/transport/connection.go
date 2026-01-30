package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

type clientConn struct {
	conn        mcp.Connection
	pending     map[string]chan callResult
	emitter     domain.ListChangeEmitter
	sampling    domain.SamplingHandler
	elicitation domain.ElicitationHandler
	serverType  string
	specKey     string
	logger      *zap.Logger

	mu        sync.Mutex
	capsMu    sync.RWMutex
	caps      domain.ServerCapabilities
	capsSet   bool
	closeOnce sync.Once
	cancel    context.CancelFunc
	closed    chan struct{}
}

type clientConnOptions struct {
	Logger             *zap.Logger
	ListChangeEmitter  domain.ListChangeEmitter
	SamplingHandler    domain.SamplingHandler
	ElicitationHandler domain.ElicitationHandler
	ServerType         string
	SpecKey            string
}

type callResult struct {
	resp *jsonrpc.Response
	err  error
}

func newClientConn(conn mcp.Connection, opts clientConnOptions) *clientConn {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &clientConn{
		conn:        conn,
		pending:     make(map[string]chan callResult),
		emitter:     opts.ListChangeEmitter,
		sampling:    opts.SamplingHandler,
		elicitation: opts.ElicitationHandler,
		serverType:  opts.ServerType,
		specKey:     opts.SpecKey,
		logger:      logger,
		cancel:      cancel,
		closed:      make(chan struct{}),
	}
	go c.readLoop(ctx)
	return c
}

func (c *clientConn) Call(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	if c.isClosed() {
		return nil, domain.ErrConnectionClosed
	}
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok || !req.ID.IsValid() {
		return nil, errors.New("request id is required")
	}
	key, err := idKey(req.ID)
	if err != nil {
		return nil, err
	}

	resultCh := make(chan callResult, 1)
	c.mu.Lock()
	if c.pending == nil {
		c.mu.Unlock()
		return nil, domain.ErrConnectionClosed
	}
	c.pending[key] = resultCh
	c.mu.Unlock()

	if err := c.conn.Write(ctx, req); err != nil {
		c.removePending(key)
		return nil, fmt.Errorf("write request: %w", err)
	}

	select {
	case result := <-resultCh:
		if result.err != nil {
			return nil, result.err
		}
		wire, err := jsonrpc.EncodeMessage(result.resp)
		if err != nil {
			return nil, fmt.Errorf("encode response: %w", err)
		}
		return json.RawMessage(wire), nil
	case <-ctx.Done():
		c.removePending(key)
		return nil, ctx.Err()
	}
}

func (c *clientConn) Notify(ctx context.Context, method string, params json.RawMessage) error {
	if c.isClosed() {
		return domain.ErrConnectionClosed
	}
	if strings.TrimSpace(method) == "" {
		return errors.New("method is required")
	}
	req := &jsonrpc.Request{
		Method: method,
		Params: params,
	}
	if err := c.conn.Write(ctx, req); err != nil {
		return fmt.Errorf("write notification: %w", err)
	}
	return nil
}

func (c *clientConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		c.cancel()
		err = c.conn.Close()
		c.failPending(domain.ErrConnectionClosed)
	})
	return err
}

func (c *clientConn) SetCapabilities(caps domain.ServerCapabilities) {
	c.capsMu.Lock()
	c.caps = caps
	c.capsSet = true
	c.capsMu.Unlock()
}

func (c *clientConn) readLoop(ctx context.Context) {
	for {
		msg, err := c.conn.Read(ctx)
		if err != nil {
			c.failPending(fmt.Errorf("read: %w", err))
			return
		}
		switch typed := msg.(type) {
		case *jsonrpc.Response:
			c.dispatchResponse(typed)
		case *jsonrpc.Request:
			if typed.ID.IsValid() {
				c.handleServerCall(ctx, typed)
				continue
			}
			c.handleNotification(typed)
		}
	}
}

func (c *clientConn) dispatchResponse(resp *jsonrpc.Response) {
	key, err := idKey(resp.ID)
	if err != nil {
		c.logger.Debug("drop response with invalid id", zap.Error(err))
		return
	}
	c.mu.Lock()
	ch := c.pending[key]
	delete(c.pending, key)
	c.mu.Unlock()
	if ch == nil {
		c.logger.Debug("drop response with no pending call", zap.String("id", key))
		return
	}
	ch <- callResult{resp: resp}
}

func (c *clientConn) handleServerCall(ctx context.Context, req *jsonrpc.Request) {
	var resp *jsonrpc.Response
	switch req.Method {
	case "sampling/createMessage":
		resp = c.handleSamplingCall(ctx, req)
	case "elicitation/create":
		resp = c.handleElicitationCall(ctx, req)
	default:
		resp = newMethodNotFoundResponse(req.ID)
	}
	if err := c.conn.Write(ctx, resp); err != nil {
		c.logger.Warn("respond to server call failed", zap.String("method", req.Method), zap.Error(err))
	}
}

func (c *clientConn) handleSamplingCall(ctx context.Context, req *jsonrpc.Request) *jsonrpc.Response {
	if c.sampling == nil {
		return newMethodNotFoundResponse(req.ID)
	}
	var params domain.SamplingRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonrpc.Response{ID: req.ID, Error: fmt.Errorf("decode sampling params: %w", err)}
	}
	result, err := c.sampling.CreateMessage(ctx, &params)
	if err != nil {
		return &jsonrpc.Response{ID: req.ID, Error: err}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return &jsonrpc.Response{ID: req.ID, Error: fmt.Errorf("encode sampling result: %w", err)}
	}
	return &jsonrpc.Response{ID: req.ID, Result: raw}
}

func (c *clientConn) handleElicitationCall(ctx context.Context, req *jsonrpc.Request) *jsonrpc.Response {
	if c.elicitation == nil {
		return newMethodNotFoundResponse(req.ID)
	}
	var params domain.ElicitationRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonrpc.Response{ID: req.ID, Error: fmt.Errorf("decode elicitation params: %w", err)}
	}
	result, err := c.elicitation.Elicit(ctx, &params)
	if err != nil {
		return &jsonrpc.Response{ID: req.ID, Error: err}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return &jsonrpc.Response{ID: req.ID, Error: fmt.Errorf("encode elicitation result: %w", err)}
	}
	return &jsonrpc.Response{ID: req.ID, Result: raw}
}

func (c *clientConn) handleNotification(req *jsonrpc.Request) {
	switch req.Method {
	case "notifications/tools/list_changed":
		c.emitListChange(domain.ListChangeTools)
	case "notifications/resources/list_changed":
		c.emitListChange(domain.ListChangeResources)
	case "notifications/prompts/list_changed":
		c.emitListChange(domain.ListChangePrompts)
	}
}

func (c *clientConn) emitListChange(kind domain.ListChangeKind) {
	if c.emitter == nil {
		return
	}
	if !c.listChangeAllowed(kind) {
		return
	}
	c.emitter.EmitListChange(domain.ListChangeEvent{
		Kind:       kind,
		ServerType: c.serverType,
		SpecKey:    c.specKey,
	})
}

func (c *clientConn) listChangeAllowed(kind domain.ListChangeKind) bool {
	c.capsMu.RLock()
	caps := c.caps
	capsSet := c.capsSet
	c.capsMu.RUnlock()

	if !capsSet {
		return true
	}

	switch kind {
	case domain.ListChangeTools:
		return caps.Tools != nil && caps.Tools.ListChanged
	case domain.ListChangeResources:
		return caps.Resources != nil && caps.Resources.ListChanged
	case domain.ListChangePrompts:
		return caps.Prompts != nil && caps.Prompts.ListChanged
	default:
		return false
	}
}

func (c *clientConn) failPending(err error) {
	c.mu.Lock()
	pending := c.pending
	c.pending = nil
	c.mu.Unlock()
	for _, ch := range pending {
		ch <- callResult{err: err}
	}
}

func (c *clientConn) removePending(key string) {
	c.mu.Lock()
	if c.pending != nil {
		delete(c.pending, key)
	}
	c.mu.Unlock()
}

func (c *clientConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

func idKey(id jsonrpc.ID) (string, error) {
	if !id.IsValid() {
		return "", errors.New("missing request id")
	}
	raw := id.Raw()
	switch typed := raw.(type) {
	case string:
		return "s:" + typed, nil
	case float64:
		return fmt.Sprintf("n:%v", typed), nil
	case int:
		return fmt.Sprintf("n:%v", typed), nil
	case int64:
		return fmt.Sprintf("n:%v", typed), nil
	case json.Number:
		return "n:" + typed.String(), nil
	default:
		return "", fmt.Errorf("unsupported id type %T", raw)
	}
}

func newMethodNotFoundResponse(id jsonrpc.ID) *jsonrpc.Response {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id.Raw(),
		"error": map[string]any{
			"code":    -32601,
			"message": "method not found",
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return &jsonrpc.Response{ID: id, Error: errors.New("method not found")}
	}
	msg, err := jsonrpc.DecodeMessage(raw)
	if err != nil {
		return &jsonrpc.Response{ID: id, Error: errors.New("method not found")}
	}
	resp, ok := msg.(*jsonrpc.Response)
	if !ok {
		return &jsonrpc.Response{ID: id, Error: errors.New("method not found")}
	}
	return resp
}
