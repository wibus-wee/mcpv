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
	"mcpd/internal/infra/telemetry"
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
	started := time.Now()
	m.logger.Info("instance start attempt",
		telemetry.EventField(telemetry.EventStartAttempt),
		telemetry.ServerTypeField(spec.Name),
	)

	conn, stop, err := m.transport.Start(ctx, spec)
	if err != nil {
		m.logger.Error("instance start failed",
			telemetry.EventField(telemetry.EventStartFailure),
			telemetry.ServerTypeField(spec.Name),
			telemetry.DurationField(time.Since(started)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("start transport: %w", err)
	}
	if conn == nil {
		err := errors.New("transport returned nil connection")
		m.logger.Error("instance start failed",
			telemetry.EventField(telemetry.EventStartFailure),
			telemetry.ServerTypeField(spec.Name),
			telemetry.DurationField(time.Since(started)),
			zap.Error(err),
		)
		return nil, err
	}
	if stop == nil {
		stop = func(context.Context) error { return nil }
	}

	if spec.ProtocolVersion != domain.DefaultProtocolVersion {
		err := fmt.Errorf("unsupported protocol version: %s", spec.ProtocolVersion)
		m.logger.Error("instance start failed",
			telemetry.EventField(telemetry.EventStartFailure),
			telemetry.ServerTypeField(spec.Name),
			telemetry.DurationField(time.Since(started)),
			zap.Error(err),
		)
		_ = conn.Close()
		_ = stop(ctx)
		return nil, err
	}

	caps, err := m.initialize(ctx, conn)
	if err != nil {
		m.logger.Error("instance initialize failed",
			telemetry.EventField(telemetry.EventInitializeFailure),
			telemetry.ServerTypeField(spec.Name),
			telemetry.DurationField(time.Since(started)),
			zap.Error(err),
		)
		_ = conn.Close()
		_ = stop(ctx)
		return nil, fmt.Errorf("initialize: %w", err)
	}

	instance := &domain.Instance{
		ID:           m.generateInstanceID(spec),
		Spec:         spec,
		State:        domain.InstanceStateReady,
		BusyCount:    0,
		LastActive:   time.Now(),
		Conn:         conn,
		Capabilities: caps,
	}

	m.mu.Lock()
	m.conns[instance.ID] = conn
	m.stops[instance.ID] = stop
	m.mu.Unlock()

	m.logger.Info("instance started",
		telemetry.EventField(telemetry.EventStartSuccess),
		telemetry.ServerTypeField(spec.Name),
		telemetry.InstanceIDField(instance.ID),
		telemetry.StateField(string(instance.State)),
		telemetry.DurationField(time.Since(started)),
	)
	return instance, nil
}

func (m *Manager) initialize(ctx context.Context, conn domain.Conn) (domain.ServerCapabilities, error) {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	initParams := &mcp.InitializeParams{
		ProtocolVersion: domain.DefaultProtocolVersion,
		ClientInfo: &mcp.Implementation{
			Name:    "mcpd",
			Version: "0.1.0",
		},
		Capabilities: &mcp.ClientCapabilities{
			Sampling: &mcp.SamplingCapabilities{},
		},
	}

	id, err := jsonrpc.MakeID("mcpd-init")
	if err != nil {
		return domain.ServerCapabilities{}, fmt.Errorf("build initialize id: %w", err)
	}
	rawParams, err := json.Marshal(initParams)
	if err != nil {
		return domain.ServerCapabilities{}, fmt.Errorf("marshal initialize params: %w", err)
	}
	wireMsg := &jsonrpc.Request{
		ID:     id,
		Method: "initialize",
		Params: rawParams,
	}
	wire, err := jsonrpc.EncodeMessage(wireMsg)
	if err != nil {
		return domain.ServerCapabilities{}, fmt.Errorf("encode initialize: %w", err)
	}

	if err := conn.Send(pingCtx, wire); err != nil {
		return domain.ServerCapabilities{}, fmt.Errorf("send initialize: %w", err)
	}

	rawResp, err := conn.Recv(pingCtx)
	if err != nil {
		return domain.ServerCapabilities{}, fmt.Errorf("recv initialize: %w", err)
	}

	return m.validateInitializeResponse(rawResp)
}

func (m *Manager) StopInstance(ctx context.Context, instance *domain.Instance, reason string) error {
	if instance == nil {
		return errors.New("instance is nil")
	}

	started := time.Now()

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
			m.logger.Error("instance stop failed",
				telemetry.EventField(telemetry.EventStopFailure),
				telemetry.ServerTypeField(instance.Spec.Name),
				telemetry.InstanceIDField(instance.ID),
				telemetry.DurationField(time.Since(started)),
				zap.Error(err),
			)
			return fmt.Errorf("stop instance %s: %w", instance.ID, err)
		}
	}

	instance.State = domain.InstanceStateStopped
	m.logger.Info("instance stopped",
		telemetry.EventField(telemetry.EventStopSuccess),
		telemetry.ServerTypeField(instance.Spec.Name),
		telemetry.InstanceIDField(instance.ID),
		telemetry.StateField(string(instance.State)),
		telemetry.DurationField(time.Since(started)),
		zap.String("reason", reason),
	)
	return nil
}

func (m *Manager) generateInstanceID(spec domain.ServerSpec) string {
	return fmt.Sprintf("%s-%d-%d", spec.Name, time.Now().UnixNano(), rand.Int63())
}

func (m *Manager) validateInitializeResponse(raw json.RawMessage) (domain.ServerCapabilities, error) {
	respMsg, err := jsonrpc.DecodeMessage(raw)
	if err != nil {
		return domain.ServerCapabilities{}, fmt.Errorf("decode initialize response: %w", err)
	}

	resp, ok := respMsg.(*jsonrpc.Response)
	if !ok {
		return domain.ServerCapabilities{}, errors.New("initialize response is not a response message")
	}
	if resp.Error != nil {
		return domain.ServerCapabilities{}, fmt.Errorf("initialize error: %w", resp.Error)
	}

	if len(resp.Result) == 0 {
		return domain.ServerCapabilities{}, errors.New("initialize response missing result")
	}

	var initResult mcp.InitializeResult
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		return domain.ServerCapabilities{}, fmt.Errorf("decode initialize result: %w", err)
	}

	if initResult.ProtocolVersion != domain.DefaultProtocolVersion {
		return domain.ServerCapabilities{}, fmt.Errorf("protocolVersion mismatch: %s", initResult.ProtocolVersion)
	}
	if initResult.ServerInfo == nil || initResult.ServerInfo.Name == "" {
		return domain.ServerCapabilities{}, errors.New("missing serverInfo")
	}
	if initResult.Capabilities == nil {
		return domain.ServerCapabilities{}, errors.New("missing capabilities")
	}

	return mapCapabilities(initResult.Capabilities), nil
}

func mapCapabilities(caps *mcp.ServerCapabilities) domain.ServerCapabilities {
	out := domain.ServerCapabilities{}
	if caps == nil {
		return out
	}
	if caps.Tools != nil {
		out.Tools = &domain.ToolsCapability{
			ListChanged: caps.Tools.ListChanged,
		}
	}
	if caps.Resources != nil {
		out.Resources = &domain.ResourcesCapability{
			Subscribe:   caps.Resources.Subscribe,
			ListChanged: caps.Resources.ListChanged,
		}
	}
	if caps.Prompts != nil {
		out.Prompts = &domain.PromptsCapability{
			ListChanged: caps.Prompts.ListChanged,
		}
	}
	if caps.Logging != nil {
		out.Logging = &domain.LoggingCapability{}
	}
	if caps.Completions != nil {
		out.Completions = &domain.CompletionsCapability{}
	}
	if len(caps.Experimental) > 0 {
		out.Experimental = caps.Experimental
	}
	return out
}
