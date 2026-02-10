package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/buildinfo"
	"mcpv/internal/domain"
	"mcpv/internal/infra/retry"
	"mcpv/internal/infra/telemetry"
	"mcpv/internal/infra/telemetry/diagnostics"
)

func wrapLifecycleStartError(err error) error {
	if err == nil {
		return nil
	}
	if code, ok := domain.CodeFrom(err); ok {
		return domain.Wrap(code, "lifecycle start", err)
	}
	return domain.Wrap(domain.CodeUnavailable, "lifecycle start", err)
}

func wrapLifecycleStopError(err error) error {
	if err == nil {
		return nil
	}
	if code, ok := domain.CodeFrom(err); ok {
		return domain.Wrap(code, "lifecycle stop", err)
	}
	return domain.Wrap(domain.CodeInternal, "lifecycle stop", err)
}

type Manager struct {
	launcher  domain.Launcher
	transport domain.Transport
	logger    *zap.Logger
	ctx       context.Context
	probe     diagnostics.Probe

	mu    sync.Mutex
	conns map[string]domain.Conn
	stops map[string]domain.StopFn

	samplingHandler    domain.SamplingHandler
	elicitationHandler domain.ElicitationHandler
}

const (
	initializeRetryCount = 3
	initializeRetryDelay = 500 * time.Millisecond
)

func NewManager(ctx context.Context, launcher domain.Launcher, transport domain.Transport, probe diagnostics.Probe, logger *zap.Logger) *Manager {
	if transport == nil {
		panic("lifecycle.Manager requires a transport")
	}
	if launcher == nil {
		panic("lifecycle.Manager requires a launcher")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if probe == nil {
		probe = diagnostics.NoopProbe{}
	}
	return &Manager{
		launcher:  launcher,
		transport: transport,
		conns:     make(map[string]domain.Conn),
		stops:     make(map[string]domain.StopFn),
		logger:    logger.Named("lifecycle"),
		ctx:       ctx,
		probe:     probe,
	}
}

// SetSamplingHandler configures the sampling handler for client capabilities.
func (m *Manager) SetSamplingHandler(handler domain.SamplingHandler) {
	m.samplingHandler = handler
}

// SetElicitationHandler configures the elicitation handler for client capabilities.
func (m *Manager) SetElicitationHandler(handler domain.ElicitationHandler) {
	m.elicitationHandler = handler
}

func (m *Manager) StartInstance(ctx context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	baseCtx := m.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	if ctx == nil {
		ctx = baseCtx
	}
	logger := telemetry.LoggerWithRequest(ctx, m.logger)

	started := time.Now()
	var spawnedAt time.Time
	attemptID, _ := diagnostics.AttemptIDFromContext(ctx)
	logger.Info("instance start attempt",
		telemetry.EventField(telemetry.EventStartAttempt),
		telemetry.ServerTypeField(spec.Name),
	)

	transportKind := domain.NormalizeTransport(spec.Transport)
	if !domain.IsSupportedProtocolVersion(transportKind, spec.ProtocolVersion) {
		err := fmt.Errorf("%w: %s", domain.ErrUnsupportedProtocol, spec.ProtocolVersion)
		logger.Error("instance start failed",
			telemetry.EventField(telemetry.EventStartFailure),
			telemetry.ServerTypeField(spec.Name),
			telemetry.DurationField(time.Since(started)),
			zap.Error(err),
		)
		return nil, wrapLifecycleStartError(err)
	}

	startCtx, cancelStart := context.WithCancel(baseCtx)
	cancelOnReturn := cancelStart
	defer func() {
		if cancelOnReturn != nil {
			cancelOnReturn()
		}
	}()
	var detached atomic.Bool
	stopBridge := func() bool { return true }
	if ctx != nil && ctx != baseCtx {
		stopBridge = context.AfterFunc(ctx, func() {
			if detached.Load() {
				return
			}
			cancelStart()
		})
	}
	defer stopBridge()

	var streams domain.IOStreams
	var stop domain.StopFn
	if transportKind == domain.TransportStdio {
		var err error
		streams, stop, err = m.launcher.Start(startCtx, specKey, spec)
		if err != nil {
			cancelStart()
			logger.Error("instance start failed",
				telemetry.EventField(telemetry.EventStartFailure),
				telemetry.ServerTypeField(spec.Name),
				telemetry.DurationField(time.Since(started)),
				zap.Error(err),
			)
			return nil, wrapLifecycleStartError(fmt.Errorf("start launcher: %w", err))
		}
		if streams.Reader == nil || streams.Writer == nil {
			err := errors.New("launcher returned nil streams")
			cancelStart()
			logger.Error("instance start failed",
				telemetry.EventField(telemetry.EventStartFailure),
				telemetry.ServerTypeField(spec.Name),
				telemetry.DurationField(time.Since(started)),
				zap.Error(err),
			)
			if stop != nil {
				_ = stop(ctx)
			}
			return nil, wrapLifecycleStartError(err)
		}
		if stop == nil {
			stop = func(context.Context) error { return nil }
		}
	} else {
		// Streamable HTTP connects to external servers and does not use IO streams.
		streams = domain.IOStreams{}
	}
	stop = wrapStop(stop, cancelStart)
	spawnedAt = time.Now()

	conn, err := m.transport.Connect(startCtx, specKey, spec, streams)
	if err != nil {
		cancelStart()
		logger.Error("instance connect failed",
			telemetry.EventField(telemetry.EventStartFailure),
			telemetry.ServerTypeField(spec.Name),
			telemetry.DurationField(time.Since(started)),
			zap.Error(err),
		)
		if stop != nil {
			_ = stop(ctx)
		}
		return nil, wrapLifecycleStartError(fmt.Errorf("connect transport: %w", err))
	}
	if conn == nil {
		err := errors.New("transport returned nil connection")
		cancelStart()
		logger.Error("instance start failed",
			telemetry.EventField(telemetry.EventStartFailure),
			telemetry.ServerTypeField(spec.Name),
			telemetry.DurationField(time.Since(started)),
			zap.Error(err),
		)
		if stop != nil {
			_ = stop(ctx)
		}
		return nil, wrapLifecycleStartError(err)
	}

	instance := domain.NewInstance(domain.InstanceOptions{
		ID:         m.generateInstanceID(spec),
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateInitializing,
		Conn:       conn,
		SpawnedAt:  spawnedAt,
		LastActive: time.Now(),
	})

	instance.SetState(domain.InstanceStateHandshaking)
	caps, err := m.initializeWithRetry(ctx, conn, specKey, spec, attemptID)
	if err != nil {
		cancelStart()
		logger.Error("instance initialize failed",
			telemetry.EventField(telemetry.EventInitializeFailure),
			telemetry.ServerTypeField(spec.Name),
			telemetry.DurationField(time.Since(started)),
			zap.Error(err),
		)
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("instance close after init failure failed",
				telemetry.ServerTypeField(spec.Name),
				zap.Error(closeErr),
			)
		}
		if stop != nil {
			if stopErr := stop(ctx); stopErr != nil {
				logger.Warn("instance stop after init failure failed",
					telemetry.ServerTypeField(spec.Name),
					zap.Error(stopErr),
				)
			}
		}
		return nil, wrapLifecycleStartError(fmt.Errorf("initialize: %w", err))
	}
	if setter, ok := conn.(interface {
		SetCapabilities(domain.ServerCapabilities)
	}); ok {
		setter.SetCapabilities(caps)
	}
	m.notifyInitialized(ctx, conn, specKey, spec, attemptID)
	instance.SetState(domain.InstanceStateReady)
	instance.SetHandshakedAt(time.Now())
	instance.SetLastHeartbeatAt(instance.HandshakedAt())
	instance.SetCapabilities(caps)

	m.mu.Lock()
	m.conns[instance.ID()] = conn
	m.stops[instance.ID()] = stop
	m.mu.Unlock()

	logger.Info("instance started",
		telemetry.EventField(telemetry.EventStartSuccess),
		telemetry.ServerTypeField(spec.Name),
		telemetry.InstanceIDField(instance.ID()),
		telemetry.StateField(string(instance.State())),
		telemetry.DurationField(time.Since(started)),
	)
	m.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepInstanceReady,
		Phase:      diagnostics.PhaseExit,
		Timestamp:  time.Now(),
		Duration:   time.Since(started),
	})
	detached.Store(true)
	cancelOnReturn = nil
	return instance, nil
}

func (m *Manager) initializeWithRetry(ctx context.Context, conn domain.Conn, specKey string, spec domain.ServerSpec, attemptID string) (domain.ServerCapabilities, error) {
	var caps domain.ServerCapabilities
	var lastErr error
	attempt := 0
	policy := retry.Policy{
		BaseDelay:  initializeRetryDelay,
		MaxDelay:   initializeRetryDelay,
		Factor:     1,
		MaxRetries: initializeRetryCount,
	}

	logger := telemetry.LoggerWithRequest(ctx, m.logger)
	err := retry.Retry(ctx, policy, func(ctx context.Context) error {
		attempt++
		result, initErr := m.initialize(ctx, conn, specKey, spec, attemptID, attempt)
		if initErr == nil {
			caps = result
			return nil
		}
		lastErr = initErr
		if policy.MaxRetries < 0 || attempt <= policy.MaxRetries {
			logger.Debug("initialize retry failed",
				zap.String("server", spec.Name),
				zap.Int("attempt", attempt),
				zap.Error(initErr),
			)
		}
		return initErr
	})
	if err != nil {
		if ctx.Err() != nil {
			return domain.ServerCapabilities{}, ctx.Err()
		}
		if lastErr != nil {
			return domain.ServerCapabilities{}, lastErr
		}
		return domain.ServerCapabilities{}, err
	}
	return caps, nil
}

func (m *Manager) initialize(ctx context.Context, conn domain.Conn, specKey string, spec domain.ServerSpec, attemptID string, attempt int) (domain.ServerCapabilities, error) {
	// Allow longer timeout for slow-starting servers (e.g., npx downloads)
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	initParams := &mcp.InitializeParams{
		ProtocolVersion: spec.ProtocolVersion,
		ClientInfo: &mcp.Implementation{
			Name:    "mcpv",
			Version: buildinfo.Version,
		},
		Capabilities: &mcp.ClientCapabilities{},
	}
	if m.samplingHandler != nil {
		initParams.Capabilities.Sampling = &mcp.SamplingCapabilities{}
	}
	if m.elicitationHandler != nil {
		initParams.Capabilities.Elicitation = &mcp.ElicitationCapabilities{}
	}

	attrs := map[string]string{
		"attempt":         strconv.Itoa(attempt),
		"protocolVersion": spec.ProtocolVersion,
	}

	id, err := jsonrpc.MakeID("mcpv-init")
	if err != nil {
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepInitializeCall,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Error:      fmt.Errorf("build initialize id: %w", err).Error(),
			Attributes: cloneStringMap(attrs),
		})
		return domain.ServerCapabilities{}, fmt.Errorf("build initialize id: %w", err)
	}
	rawParams, err := json.Marshal(initParams)
	if err != nil {
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepInitializeCall,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Error:      fmt.Errorf("marshal initialize params: %w", err).Error(),
			Attributes: cloneStringMap(attrs),
		})
		return domain.ServerCapabilities{}, fmt.Errorf("marshal initialize params: %w", err)
	}
	wireMsg := &jsonrpc.Request{
		ID:     id,
		Method: "initialize",
		Params: rawParams,
	}
	wire, err := jsonrpc.EncodeMessage(wireMsg)
	if err != nil {
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepInitializeCall,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Error:      fmt.Errorf("encode initialize: %w", err).Error(),
			Attributes: map[string]string{
				"attempt": strconv.Itoa(attempt),
			},
		})
		return domain.ServerCapabilities{}, fmt.Errorf("encode initialize: %w", err)
	}

	started := time.Now()
	attrs["requestSizeBytes"] = strconv.Itoa(len(rawParams))
	attrs["requestHash"] = diagnostics.HashBytes(rawParams)
	sensitive := map[string]string{}
	if m.captureSensitive() {
		sensitive["request"] = diagnostics.TruncateString(string(rawParams), 2048)
	}
	m.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepInitializeCall,
		Phase:      diagnostics.PhaseEnter,
		Timestamp:  started,
		Attributes: cloneStringMap(attrs),
		Sensitive:  sensitive,
	})
	rawResp, err := conn.Call(pingCtx, wire)
	if err != nil {
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepInitializeCall,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("call initialize: %w", err).Error(),
			Attributes: cloneStringMap(attrs),
			Sensitive:  sensitive,
		})
		return domain.ServerCapabilities{}, fmt.Errorf("call initialize: %w", err)
	}

	attrs["responseSizeBytes"] = strconv.Itoa(len(rawResp))
	attrs["responseHash"] = diagnostics.HashBytes(rawResp)
	caps, err := m.validateInitializeResponse(rawResp)
	if err != nil {
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepInitializeResponse,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      err.Error(),
			Attributes: cloneStringMap(attrs),
			Sensitive:  sensitive,
		})
		return domain.ServerCapabilities{}, err
	}
	m.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepInitializeCall,
		Phase:      diagnostics.PhaseExit,
		Timestamp:  time.Now(),
		Duration:   time.Since(started),
		Attributes: cloneStringMap(attrs),
		Sensitive:  sensitive,
	})
	return caps, nil
}

func (m *Manager) notifyInitialized(ctx context.Context, conn domain.Conn, specKey string, spec domain.ServerSpec, attemptID string) {
	notifier, ok := conn.(interface {
		Notify(context.Context, string, json.RawMessage) error
	})
	if !ok {
		return
	}
	logger := telemetry.LoggerWithRequest(ctx, m.logger)
	started := time.Now()
	m.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepNotifyInitialized,
		Phase:      diagnostics.PhaseEnter,
		Timestamp:  started,
	})
	notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := notifier.Notify(notifyCtx, "notifications/initialized", json.RawMessage(`{}`)); err != nil {
		m.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepNotifyInitialized,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      err.Error(),
		})
		logger.Debug("send initialized notification failed",
			telemetry.ServerTypeField(spec.Name),
			zap.Error(err),
		)
		return
	}
	m.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepNotifyInitialized,
		Phase:      diagnostics.PhaseExit,
		Timestamp:  time.Now(),
		Duration:   time.Since(started),
	})
}

func (m *Manager) StopInstance(ctx context.Context, instance *domain.Instance, reason string) error {
	if instance == nil {
		return wrapLifecycleStopError(errors.New("instance is nil"))
	}
	logger := telemetry.LoggerWithRequest(ctx, m.logger)

	started := time.Now()
	instanceID := instance.ID()
	spec := instance.Spec()

	m.mu.Lock()
	conn := m.conns[instanceID]
	stop := m.stops[instanceID]
	delete(m.conns, instanceID)
	delete(m.stops, instanceID)
	m.mu.Unlock()

	if conn == nil && stop == nil {
		return wrapLifecycleStopError(fmt.Errorf("unknown instance: %s", instanceID))
	}

	var closeErr error
	if conn != nil {
		if err := conn.Close(); err != nil {
			closeErr = err
			logger.Warn("instance close failed",
				telemetry.ServerTypeField(spec.Name),
				telemetry.InstanceIDField(instanceID),
				zap.Error(err),
			)
		}
	}
	var stopErr error
	if stop != nil {
		if err := stop(ctx); err != nil {
			stopErr = err
			logger.Error("instance stop failed",
				telemetry.EventField(telemetry.EventStopFailure),
				telemetry.ServerTypeField(spec.Name),
				telemetry.InstanceIDField(instanceID),
				telemetry.DurationField(time.Since(started)),
				zap.Error(err),
			)
		}
	}
	if stopErr != nil {
		return wrapLifecycleStopError(fmt.Errorf("stop instance %s: %w", instanceID, errors.Join(stopErr, closeErr)))
	}
	if closeErr != nil {
		return wrapLifecycleStopError(fmt.Errorf("close instance %s: %w", instanceID, closeErr))
	}

	instance.SetState(domain.InstanceStateStopped)
	logger.Info("instance stopped",
		telemetry.EventField(telemetry.EventStopSuccess),
		telemetry.ServerTypeField(spec.Name),
		telemetry.InstanceIDField(instanceID),
		telemetry.StateField(string(instance.State())),
		telemetry.DurationField(time.Since(started)),
		zap.String("reason", reason),
	)
	return nil
}

func (m *Manager) generateInstanceID(spec domain.ServerSpec) string {
	return fmt.Sprintf("%s-%d-%d", spec.Name, time.Now().UnixNano(), rand.Int63())
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func (m *Manager) recordEvent(event diagnostics.Event) {
	if m == nil || m.probe == nil {
		return
	}
	if len(event.Sensitive) == 0 {
		event.Sensitive = nil
	}
	m.probe.Record(event)
}

func (m *Manager) captureSensitive() bool {
	if m == nil || m.probe == nil {
		return false
	}
	if probe, ok := m.probe.(diagnostics.SensitiveProbe); ok {
		return probe.CaptureSensitive()
	}
	return false
}

func wrapStop(stop domain.StopFn, cancel context.CancelFunc) domain.StopFn {
	return func(ctx context.Context) error {
		if cancel != nil {
			defer cancel()
		}
		if stop == nil {
			return nil
		}
		return stop(ctx)
	}
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

	// if initResult.ProtocolVersion != domain.DefaultProtocolVersion {
	// 	return domain.ServerCapabilities{}, fmt.Errorf("protocolVersion mismatch: %s", initResult.ProtocolVersion)
	// }
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
