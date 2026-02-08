package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/plugin/handshake"
	"mcpv/internal/infra/plugin/instance"
	"mcpv/internal/infra/plugin/process"
	"mcpv/internal/infra/plugin/socket"
	"mcpv/internal/infra/telemetry"
	pluginv1 "mcpv/pkg/api/plugin/v1"
)

type Manager struct {
	logger    *zap.Logger
	rootDir   string
	mu        sync.RWMutex
	instances map[string]*instance.Instance
	metrics   domain.Metrics
}

type Options struct {
	RootDir string
	Logger  *zap.Logger
	Metrics domain.Metrics
}

func NewManager(opts Options) (*Manager, error) {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	metrics := opts.Metrics
	if metrics == nil {
		metrics = telemetry.NewNoopMetrics()
	}
	rootDir := strings.TrimSpace(opts.RootDir)
	if rootDir == "" {
		// Use /tmp directly instead of os.TempDir() to avoid long paths on macOS
		// (/var/folders/...) which can exceed Unix socket path limit (104-108 bytes)
		var err error
		rootDir, err = os.MkdirTemp("/tmp", "mcpv-plug-")
		if err != nil {
			// Fallback to system temp dir if /tmp fails
			rootDir, err = os.MkdirTemp("", "mcpv-plug-")
			if err != nil {
				return nil, fmt.Errorf("create plugin root dir: %w", err)
			}
		}
	} else if err := os.MkdirAll(rootDir, 0o700); err != nil {
		return nil, fmt.Errorf("create plugin root dir: %w", err)
	}

	return &Manager{
		logger:    logger.Named("plugin_manager"),
		rootDir:   rootDir,
		instances: make(map[string]*instance.Instance),
		metrics:   metrics,
	}, nil
}

func (m *Manager) RootDir() string {
	return m.rootDir
}

// Status represents the runtime status of a plugin.
type Status struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// GetStatus returns the runtime status of all configured plugins.
// It compares configured specs against actually running instances.
func (m *Manager) GetStatus(configuredSpecs []domain.PluginSpec) []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make([]Status, 0, len(configuredSpecs))
	for _, spec := range configuredSpecs {
		running := false
		if _, ok := m.instances[spec.Name]; ok {
			running = true
		}
		s := Status{
			Name:    spec.Name,
			Running: running,
		}
		if !running {
			s.Error = "Plugin failed to start or is not running"
		}
		status = append(status, s)
	}
	return status
}

// IsRunning checks if a specific plugin is currently running.
func (m *Manager) IsRunning(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.instances[name]
	return ok
}

func (m *Manager) Snapshot() []domain.PluginSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.PluginSpec, 0, len(m.instances))
	for _, inst := range m.instances {
		out = append(out, inst.Spec)
	}
	return out
}

func (m *Manager) Apply(ctx context.Context, specs []domain.PluginSpec) error {
	if ctx == nil {
		ctx = context.Background()
	}
	desired := make(map[string]domain.PluginSpec, len(specs))
	for _, spec := range specs {
		if spec.Name == "" {
			continue
		}
		desired[spec.Name] = spec
	}

	m.mu.RLock()
	existing := make(map[string]*instance.Instance, len(m.instances))
	for name, inst := range m.instances {
		existing[name] = inst
	}
	m.mu.RUnlock()

	var applyErrs []string

	// Start or restart updated plugins.
	for name, spec := range desired {
		inst, ok := existing[name]
		if ok && reflect.DeepEqual(inst.Spec, spec) {
			continue
		}
		newInst, err := m.startInstance(ctx, spec)
		if err != nil {
			if spec.Required {
				applyErrs = append(applyErrs, fmt.Sprintf("plugin %q start failed: %v", name, err))
				continue
			}
			m.logger.Warn("optional plugin start failed", zap.String("plugin", name), zap.Error(err))
			continue
		}
		m.mu.Lock()
		m.instances[name] = newInst
		m.mu.Unlock()
		m.setPluginRunning(spec, true)
		if ok {
			_ = inst.Stop(context.Background())
			m.cleanupInstance(inst)
		}
	}

	// Stop removed plugins.
	for name, inst := range existing {
		if _, ok := desired[name]; ok {
			continue
		}
		m.mu.Lock()
		delete(m.instances, name)
		m.mu.Unlock()
		if err := inst.Stop(context.Background()); err != nil {
			m.logger.Warn("plugin stop failed", zap.String("plugin", name), zap.Error(err))
		}
		m.setPluginRunning(inst.Spec, false)
		m.cleanupInstance(inst)
	}

	if len(applyErrs) > 0 {
		return domain.Wrap(domain.CodeFailedPrecond, "plugin apply", errors.New(strings.Join(applyErrs, "; ")))
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	m.mu.Lock()
	instances := m.instances
	m.instances = make(map[string]*instance.Instance)
	m.mu.Unlock()

	for _, inst := range instances {
		if err := inst.Stop(ctx); err != nil {
			m.logger.Warn("plugin stop failed", zap.String("plugin", inst.Spec.Name), zap.Error(err))
		}
		m.setPluginRunning(inst.Spec, false)
		m.cleanupInstance(inst)
	}
}

func (m *Manager) Handle(ctx context.Context, spec domain.PluginSpec, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
	m.mu.RLock()
	inst, ok := m.instances[spec.Name]
	m.mu.RUnlock()
	if !ok {
		return domain.GovernanceDecision{}, fmt.Errorf("plugin %q not available", spec.Name)
	}

	flow := req.Flow
	if flow == "" {
		flow = domain.PluginFlowRequest
	}

	deadline := time.Duration(domain.DefaultPluginCallTimeoutMs) * time.Millisecond
	if spec.TimeoutMs > 0 {
		deadline = time.Duration(spec.TimeoutMs) * time.Millisecond
	}
	callCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	grpcReq := &pluginv1.PluginHandleRequest{
		Flow:         string(flow),
		Method:       req.Method,
		Caller:       req.Caller,
		Server:       req.Server,
		ToolName:     req.ToolName,
		ResourceUri:  req.ResourceURI,
		PromptName:   req.PromptName,
		RoutingKey:   req.RoutingKey,
		RequestJson:  req.RequestJSON,
		ResponseJson: req.ResponseJSON,
		Metadata:     req.Metadata,
	}

	var resp *pluginv1.PluginHandleResponse
	var err error
	if flow == domain.PluginFlowResponse {
		resp, err = inst.Client.HandleResponse(callCtx, grpcReq)
	} else {
		resp, err = inst.Client.HandleRequest(callCtx, grpcReq)
	}
	if err != nil {
		return domain.GovernanceDecision{}, err
	}
	if resp == nil {
		return domain.GovernanceDecision{}, errors.New("plugin returned empty response")
	}

	return domain.GovernanceDecision{
		Continue:      resp.GetContinue(),
		RequestJSON:   resp.GetRequestJson(),
		ResponseJSON:  resp.GetResponseJson(),
		RejectCode:    resp.GetRejectCode(),
		RejectMessage: resp.GetRejectMessage(),
	}, nil
}

func (m *Manager) startInstance(ctx context.Context, spec domain.PluginSpec) (*instance.Instance, error) {
	startTime := time.Now()
	socketDir, socketPath, err := socket.Prepare(m.rootDir, spec.Name)
	if err != nil {
		return nil, err
	}

	logger := m.logger.With(
		zap.String("plugin", spec.Name),
		zap.String("category", string(spec.Category)),
	)

	startCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(startCtx, spec.Cmd[0], spec.Cmd[1:]...)
	cmd.Dir = strings.TrimSpace(spec.Cwd)
	cmd.Env = buildEnv(spec.Env, map[string]string{
		"MCPV_PLUGIN_SOCKET":   socketPath,
		"MCPD_PLUGIN_SOCKET":   socketPath,
		"MCPV_PLUGIN_NAME":     spec.Name,
		"MCPV_PLUGIN_CATEGORY": string(spec.Category),
	})
	cleanup := process.Setup(cmd)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("plugin stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		m.recordPluginStart(spec, time.Since(startTime), false)
		cancel()
		if cleanup != nil {
			cleanup()
		}
		return nil, fmt.Errorf("plugin start: %w", err)
	}

	go mirrorStderr(stderr, logger.With(
		zap.String(telemetry.FieldLogSource, telemetry.LogSourceDownstream),
		zap.String(telemetry.FieldLogStream, "stderr"),
	))

	stopFn := func(stopCtx context.Context) error {
		if stopCtx == nil {
			stopCtx = context.Background()
		}
		cancel()
		if cleanup != nil {
			cleanup()
		}
		err := process.Wait(stopCtx, cmd)
		if err != nil && stopCtx.Err() != nil {
			_ = cmd.Process.Kill()
		}
		return err
	}

	handshakeStart := time.Now()
	conn, client, metadata, err := handshake.Connect(ctx, spec, socketPath)
	m.recordPluginHandshake(spec, time.Since(handshakeStart), err == nil)
	if err != nil {
		m.recordPluginStart(spec, time.Since(startTime), false)
		_ = stopFn(context.Background())
		return nil, err
	}

	m.recordPluginStart(spec, time.Since(startTime), true)

	return &instance.Instance{
		Spec:       spec,
		SocketDir:  socketDir,
		SocketPath: socketPath,
		Cmd:        cmd,
		Conn:       conn,
		Client:     client,
		Metadata:   metadata,
		Stop:       stopFn,
	}, nil
}
func (m *Manager) cleanupInstance(inst *instance.Instance) {
	if inst == nil {
		return
	}
	if inst.Conn != nil {
		_ = inst.Conn.Close()
	}
	if inst.SocketDir != "" {
		_ = os.RemoveAll(inst.SocketDir)
	}
}

func buildEnv(extra map[string]string, overrides map[string]string) []string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		env[parts[0]] = parts[1]
	}
	for key, value := range extra {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		env[key] = value
	}
	for key, value := range overrides {
		env[key] = value
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, fmt.Sprintf("%s=%s", key, env[key]))
	}
	return out
}

func mirrorStderr(reader io.ReadCloser, logger *zap.Logger) {
	defer func() {
		_ = reader.Close()
	}()
	buf := make([]byte, 8*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			line := strings.TrimRight(string(buf[:n]), "\r\n")
			if line != "" {
				logger.Info(line)
			}
		}
		if err != nil {
			return
		}
	}
}

func (m *Manager) recordPluginStart(spec domain.PluginSpec, duration time.Duration, success bool) {
	if m.metrics == nil || spec.Name == "" {
		return
	}
	m.metrics.RecordPluginStart(domain.PluginStartMetric{
		Category: spec.Category,
		Plugin:   spec.Name,
		Duration: duration,
		Success:  success,
	})
}

func (m *Manager) recordPluginHandshake(spec domain.PluginSpec, duration time.Duration, succeeded bool) {
	if m.metrics == nil || spec.Name == "" {
		return
	}
	m.metrics.RecordPluginHandshake(domain.PluginHandshakeMetric{
		Category:  spec.Category,
		Plugin:    spec.Name,
		Duration:  duration,
		Succeeded: succeeded,
	})
}

func (m *Manager) setPluginRunning(spec domain.PluginSpec, running bool) {
	if m.metrics == nil || spec.Name == "" {
		return
	}
	m.metrics.SetPluginRunning(spec.Category, spec.Name, running)
}
