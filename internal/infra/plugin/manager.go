package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry"
	pluginv1 "mcpv/pkg/api/plugin/v1"
)

const (
	socketFileName = "plugin.sock"
)

type Manager struct {
	logger    *zap.Logger
	rootDir   string
	mu        sync.RWMutex
	instances map[string]*Instance
}

type ManagerOptions struct {
	RootDir string
	Logger  *zap.Logger
}

type Instance struct {
	spec       domain.PluginSpec
	socketDir  string
	socketPath string
	cmd        *exec.Cmd
	conn       *grpc.ClientConn
	client     pluginv1.PluginServiceClient
	metadata   *pluginv1.PluginMetadata
	stop       func(context.Context) error
}

func NewManager(opts ManagerOptions) (*Manager, error) {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
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
		instances: make(map[string]*Instance),
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
		out = append(out, inst.spec)
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
	existing := make(map[string]*Instance, len(m.instances))
	for name, inst := range m.instances {
		existing[name] = inst
	}
	m.mu.RUnlock()

	var applyErrs []string

	// Start or restart updated plugins.
	for name, spec := range desired {
		inst, ok := existing[name]
		if ok && reflect.DeepEqual(inst.spec, spec) {
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
		if ok {
			_ = inst.stop(context.Background())
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
		if err := inst.stop(context.Background()); err != nil {
			m.logger.Warn("plugin stop failed", zap.String("plugin", name), zap.Error(err))
		}
		m.cleanupInstance(inst)
	}

	if len(applyErrs) > 0 {
		return errors.New(strings.Join(applyErrs, "; "))
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	m.mu.Lock()
	instances := m.instances
	m.instances = make(map[string]*Instance)
	m.mu.Unlock()

	for _, inst := range instances {
		if err := inst.stop(ctx); err != nil {
			m.logger.Warn("plugin stop failed", zap.String("plugin", inst.spec.Name), zap.Error(err))
		}
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
		resp, err = inst.client.HandleResponse(callCtx, grpcReq)
	} else {
		resp, err = inst.client.HandleRequest(callCtx, grpcReq)
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

func (m *Manager) startInstance(ctx context.Context, spec domain.PluginSpec) (*Instance, error) {
	socketDir, socketPath, err := m.prepareSocket(spec.Name)
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
	cleanup := setupProcessHandling(cmd)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("plugin stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
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
		err := waitForProcess(stopCtx, cmd)
		if err != nil && stopCtx.Err() != nil {
			_ = cmd.Process.Kill()
		}
		return err
	}

	conn, client, metadata, err := m.connectAndHandshake(ctx, spec, socketPath)
	if err != nil {
		_ = stopFn(context.Background())
		return nil, err
	}

	return &Instance{
		spec:       spec,
		socketDir:  socketDir,
		socketPath: socketPath,
		cmd:        cmd,
		conn:       conn,
		client:     client,
		metadata:   metadata,
		stop:       stopFn,
	}, nil
}

func (m *Manager) connectAndHandshake(ctx context.Context, spec domain.PluginSpec, socketPath string) (*grpc.ClientConn, pluginv1.PluginServiceClient, *pluginv1.PluginMetadata, error) {
	deadline := time.Duration(domain.DefaultPluginHandshakeTimeoutSeconds) * time.Second
	if spec.TimeoutMs > 0 {
		deadline = time.Duration(spec.TimeoutMs) * time.Millisecond
	}
	if deadline <= 0 {
		deadline = time.Duration(domain.DefaultPluginHandshakeTimeoutSeconds) * time.Second
	}

	// Wait for socket file to appear with retries
	handshakeCtx, handshakeCancel := context.WithTimeout(ctx, deadline)
	defer handshakeCancel()

	var conn *grpc.ClientConn
	var client pluginv1.PluginServiceClient
	var metadata *pluginv1.PluginMetadata
	var lastErr error

	retryInterval := 100 * time.Millisecond
	for {
		select {
		case <-handshakeCtx.Done():
			if lastErr != nil {
				return nil, nil, nil, fmt.Errorf("plugin handshake timeout: %w", lastErr)
			}
			return nil, nil, nil, handshakeCtx.Err()
		default:
		}

		// Try to connect and get metadata
		conn, client, metadata, lastErr = m.tryConnect(handshakeCtx, socketPath)
		if lastErr == nil {
			break
		}

		// Wait before retrying
		select {
		case <-handshakeCtx.Done():
			return nil, nil, nil, fmt.Errorf("plugin handshake timeout: %w", lastErr)
		case <-time.After(retryInterval):
		}
	}
	if err := validateMetadata(spec, metadata); err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}

	cfgCtx, cfgCancel := context.WithTimeout(handshakeCtx, deadline)
	defer cfgCancel()
	_, cfgErr := client.Configure(cfgCtx, &pluginv1.PluginConfigureRequest{ConfigJson: spec.ConfigJSON})
	if cfgErr != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("plugin configure: %w", cfgErr)
	}

	readyCtx, readyCancel := context.WithTimeout(handshakeCtx, deadline)
	defer readyCancel()
	ready, readyErr := client.CheckReady(readyCtx, &emptypb.Empty{})
	if readyErr != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("plugin readiness: %w", readyErr)
	}
	if ready != nil && !ready.GetReady() {
		_ = conn.Close()
		msg := strings.TrimSpace(ready.GetMessage())
		if msg == "" {
			msg = "plugin not ready"
		}
		return nil, nil, nil, errors.New(msg)
	}

	return conn, client, metadata, nil
}

func (m *Manager) tryConnect(ctx context.Context, socketPath string) (*grpc.ClientConn, pluginv1.PluginServiceClient, *pluginv1.PluginMetadata, error) {
	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			dialer := &net.Dialer{}
			return dialer.DialContext(dialCtx, "unix", socketPath)
		}),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("plugin dial: %w", err)
	}

	client := pluginv1.NewPluginServiceClient(conn)

	// Try to get metadata - this will actually establish the connection
	metaCtx, metaCancel := context.WithTimeout(ctx, 2*time.Second)
	defer metaCancel()
	metadata, err := client.GetMetadata(metaCtx, &emptypb.Empty{})
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}

	return conn, client, metadata, nil
}

func validateMetadata(spec domain.PluginSpec, metadata *pluginv1.PluginMetadata) error {
	if metadata == nil {
		return errors.New("plugin metadata missing")
	}
	if name := strings.TrimSpace(metadata.GetName()); name != "" && name != spec.Name {
		return fmt.Errorf("plugin name mismatch: expected %q got %q", spec.Name, name)
	}
	if category := strings.TrimSpace(metadata.GetCategory()); category != "" && category != string(spec.Category) {
		return fmt.Errorf("plugin category mismatch: expected %q got %q", spec.Category, category)
	}
	if spec.CommitHash != "" {
		if metadata.GetCommitHash() != spec.CommitHash {
			return fmt.Errorf("plugin commit hash mismatch: expected %q got %q", spec.CommitHash, metadata.GetCommitHash())
		}
	}
	if len(metadata.GetFlows()) > 0 && len(spec.Flows) > 0 {
		allowed := map[string]struct{}{}
		for _, flow := range metadata.GetFlows() {
			allowed[strings.ToLower(flow)] = struct{}{}
		}
		for _, flow := range spec.Flows {
			if _, ok := allowed[string(flow)]; !ok {
				return fmt.Errorf("plugin flow %q not supported", flow)
			}
		}
	}
	return nil
}

func (m *Manager) prepareSocket(name string) (string, string, error) {
	prefix := sanitizeName(name)
	if prefix == "" {
		prefix = "p"
	}
	// Truncate prefix to 8 chars to keep socket path short
	// (Unix socket paths have ~104-108 byte limit on many systems)
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	addrDir, err := os.MkdirTemp(m.rootDir, prefix+"-")
	if err != nil {
		return "", "", fmt.Errorf("create plugin socket dir: %w", err)
	}
	path := filepath.Join(addrDir, socketFileName)
	if err := os.RemoveAll(path); err != nil {
		return "", "", fmt.Errorf("cleanup plugin socket: %w", err)
	}
	return addrDir, path, nil
}

func (m *Manager) cleanupInstance(inst *Instance) {
	if inst == nil {
		return
	}
	if inst.conn != nil {
		_ = inst.conn.Close()
	}
	if inst.socketDir != "" {
		_ = os.RemoveAll(inst.socketDir)
	}
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, string(os.PathSeparator), "-")
	return name
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
