package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type Manager struct {
	transport domain.Transport
	logger    *zap.Logger

	mu    sync.Mutex
	conns map[string]domain.Conn
	stops map[string]domain.StopFn
}

func NewManager(transport domain.Transport, logger *zap.Logger) *Manager {
	if transport == nil {
		panic("lifecycle.Manager requires a transport")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		transport: transport,
		conns:     make(map[string]domain.Conn),
		stops:     make(map[string]domain.StopFn),
		logger:    logger.Named("lifecycle"),
	}
}

func (m *Manager) StartInstance(ctx context.Context, spec domain.ServerSpec) (*domain.Instance, error) {
	conn, stop, err := m.transport.Start(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("start transport: %w", err)
	}
	if conn == nil {
		return nil, errors.New("transport returned nil connection")
	}
	if stop == nil {
		stop = func(context.Context) error { return nil }
	}

	if spec.ProtocolVersion != domain.DefaultProtocolVersion {
		_ = conn.Close()
		_ = stop(ctx)
		return nil, fmt.Errorf("unsupported protocol version: %s", spec.ProtocolVersion)
	}

	if err := m.initialize(ctx, conn); err != nil {
		_ = conn.Close()
		_ = stop(ctx)
		return nil, fmt.Errorf("initialize: %w", err)
	}

	instance := &domain.Instance{
		ID:         m.generateInstanceID(spec),
		Spec:       spec,
		State:      domain.InstanceStateReady,
		BusyCount:  0,
		LastActive: time.Now(),
		Conn:       conn,
	}

	m.mu.Lock()
	m.conns[instance.ID] = conn
	m.stops[instance.ID] = stop
	m.mu.Unlock()

	m.logger.Info("instance started", zap.String("serverType", spec.Name), zap.String("instanceID", instance.ID))
	return instance, nil
}

func (m *Manager) initialize(ctx context.Context, conn domain.Conn) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	initParams := &mcp.InitializeParams{
		ProtocolVersion: domain.DefaultProtocolVersion,
		ClientInfo: &mcp.Implementation{
			Name:    "mcpd",
			Version: "0.1.0",
		},
		Capabilities: &mcp.ClientCapabilities{},
	}

	id, err := jsonrpc.MakeID("mcpd-init")
	if err != nil {
		return fmt.Errorf("build initialize id: %w", err)
	}
	rawParams, err := json.Marshal(initParams)
	if err != nil {
		return fmt.Errorf("marshal initialize params: %w", err)
	}
	wireMsg := &jsonrpc.Request{
		ID:     id,
		Method: "initialize",
		Params: rawParams,
	}
	wire, err := jsonrpc.EncodeMessage(wireMsg)
	if err != nil {
		return fmt.Errorf("encode initialize: %w", err)
	}

	if err := conn.Send(pingCtx, wire); err != nil {
		return fmt.Errorf("send initialize: %w", err)
	}

	rawResp, err := conn.Recv(pingCtx)
	if err != nil {
		return fmt.Errorf("recv initialize: %w", err)
	}

	return m.validateInitializeResponse(rawResp)
}

func (m *Manager) StopInstance(ctx context.Context, instance *domain.Instance, reason string) error {
	if instance == nil {
		return errors.New("instance is nil")
	}

	m.mu.Lock()
	conn := m.conns[instance.ID]
	stop := m.stops[instance.ID]
	delete(m.conns, instance.ID)
	delete(m.stops, instance.ID)
	m.mu.Unlock()

	if conn == nil && stop == nil {
		return fmt.Errorf("unknown instance: %s", instance.ID)
	}

	if conn != nil {
		_ = conn.Close()
	}
	if stop != nil {
		if err := stop(ctx); err != nil {
			return fmt.Errorf("stop instance %s: %w", instance.ID, err)
		}
	}

	instance.State = domain.InstanceStateStopped
	m.logger.Info("instance stopped", zap.String("serverType", instance.Spec.Name), zap.String("instanceID", instance.ID), zap.String("reason", reason))
	return nil
}

func (m *Manager) generateInstanceID(spec domain.ServerSpec) string {
	return fmt.Sprintf("%s-%d-%d", spec.Name, time.Now().UnixNano(), rand.Int63())
}

func (m *Manager) validateInitializeResponse(raw json.RawMessage) error {
	respMsg, err := jsonrpc.DecodeMessage(raw)
	if err != nil {
		return fmt.Errorf("decode initialize response: %w", err)
	}

	resp, ok := respMsg.(*jsonrpc.Response)
	if !ok {
		return errors.New("initialize response is not a response message")
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %w", resp.Error)
	}

	if len(resp.Result) == 0 {
		return errors.New("initialize response missing result")
	}

	var initResult mcp.InitializeResult
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		return fmt.Errorf("decode initialize result: %w", err)
	}

	if initResult.ProtocolVersion != domain.DefaultProtocolVersion {
		return fmt.Errorf("protocolVersion mismatch: %s", initResult.ProtocolVersion)
	}
	if initResult.ServerInfo == nil || initResult.ServerInfo.Name == "" {
		return errors.New("missing serverInfo")
	}
	if initResult.Capabilities == nil {
		return errors.New("missing capabilities")
	}

	return nil
}
