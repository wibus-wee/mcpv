package ui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type GatewayState string

const (
	GatewayStateStopped  GatewayState = "stopped"
	GatewayStateStarting GatewayState = "starting"
	GatewayStateRunning  GatewayState = "running"
	GatewayStateError    GatewayState = "error"
)

type GatewayProcessConfig struct {
	Enabled       bool
	BinaryPath    string
	Args          []string
	Env           []string
	HealthURL     string
	HealthTimeout time.Duration
	StopTimeout   time.Duration
}

type GatewayProcess struct {
	mu      sync.Mutex
	logger  *zap.Logger
	cfg     GatewayProcessConfig
	cmd     *exec.Cmd
	done    chan struct{}
	managed bool
	state   GatewayState
	started time.Time
	err     error
}

func NewGatewayProcess(logger *zap.Logger, cfg GatewayProcessConfig) *GatewayProcess {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.HealthTimeout <= 0 {
		cfg.HealthTimeout = 300 * time.Millisecond
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 3 * time.Second
	}
	if strings.TrimSpace(cfg.BinaryPath) == "" {
		cfg.BinaryPath = ResolveMcpvmcpPath()
	}
	return &GatewayProcess{
		logger: logger,
		cfg:    cfg,
		state:  GatewayStateStopped,
	}
}

func (g *GatewayProcess) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	g.mu.Lock()
	g.logger = logger
	g.mu.Unlock()
}

func (g *GatewayProcess) Enabled() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cfg.Enabled
}

func (g *GatewayProcess) UpdateConfig(cfg GatewayProcessConfig) {
	g.mu.Lock()
	g.cfg = copyGatewayConfig(cfg)
	g.mu.Unlock()
}

func (g *GatewayProcess) Config() GatewayProcessConfig {
	g.mu.Lock()
	defer g.mu.Unlock()
	return copyGatewayConfig(g.cfg)
}

func (g *GatewayProcess) Start(ctx context.Context) error {
	g.mu.Lock()
	if !g.cfg.Enabled {
		g.mu.Unlock()
		return nil
	}
	if g.state == GatewayStateRunning || g.state == GatewayStateStarting {
		g.mu.Unlock()
		return nil
	}
	g.state = GatewayStateStarting
	g.mu.Unlock()

	if g.cfg.HealthURL != "" {
		if gatewayHealthy(ctx, g.cfg.HealthURL, g.cfg.HealthTimeout) {
			g.mu.Lock()
			g.state = GatewayStateRunning
			g.managed = false
			g.err = nil
			g.mu.Unlock()
			g.logger.Info("gateway already running, skip start", zap.String("health", g.cfg.HealthURL))
			return nil
		}
	}

	args := append([]string(nil), g.cfg.Args...)
	startCtx := ctx
	if startCtx == nil {
		startCtx = context.Background()
	}
	cmd := exec.CommandContext(startCtx, g.cfg.BinaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(g.cfg.Env) > 0 {
		cmd.Env = append(os.Environ(), g.cfg.Env...)
	}

	if err := cmd.Start(); err != nil {
		g.mu.Lock()
		g.state = GatewayStateError
		g.err = err
		g.mu.Unlock()
		return err
	}

	done := make(chan struct{})
	g.mu.Lock()
	g.cmd = cmd
	g.done = done
	g.managed = true
	g.state = GatewayStateRunning
	g.started = time.Now()
	g.err = nil
	g.mu.Unlock()

	g.logger.Info("gateway started", zap.String("path", g.cfg.BinaryPath), zap.Strings("args", args))

	go g.wait(cmd, done)
	return nil
}

func (g *GatewayProcess) Stop(ctx context.Context) error {
	g.mu.Lock()
	state := g.state
	cmd := g.cmd
	done := g.done
	managed := g.managed
	timeout := g.cfg.StopTimeout
	g.mu.Unlock()

	if state != GatewayStateRunning && state != GatewayStateStarting {
		return nil
	}
	if !managed || cmd == nil || cmd.Process == nil {
		g.mu.Lock()
		g.state = GatewayStateStopped
		g.err = nil
		g.managed = false
		g.mu.Unlock()
		return nil
	}

	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	stopCtx := ctx
	if stopCtx == nil {
		stopCtx = context.Background()
	}
	stopCtx, cancel := context.WithTimeout(stopCtx, timeout)
	defer cancel()

	_ = signalGateway(cmd.Process)

	if done == nil {
		return stopCtx.Err()
	}
	select {
	case <-done:
		return nil
	case <-stopCtx.Done():
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
		return stopCtx.Err()
	}
}

func (g *GatewayProcess) State() (GatewayState, time.Duration, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	var uptime time.Duration
	if g.state == GatewayStateRunning && !g.started.IsZero() {
		uptime = time.Since(g.started)
	}
	return g.state, uptime, g.err
}

func (g *GatewayProcess) wait(cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()

	g.mu.Lock()
	g.cmd = nil
	g.done = nil
	g.managed = false
	if err != nil {
		g.state = GatewayStateError
		g.err = err
	} else {
		g.state = GatewayStateStopped
		g.err = nil
	}
	close(done)
	g.mu.Unlock()

	if err != nil && !errors.Is(err, context.Canceled) {
		g.logger.Warn("gateway exited with error", zap.Error(err))
		return
	}
	g.logger.Info("gateway exited")
}

func signalGateway(process *os.Process) error {
	if process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return process.Kill()
	}
	if err := process.Signal(os.Interrupt); err == nil {
		return nil
	}
	return process.Kill()
}

func gatewayHealthy(ctx context.Context, url string, timeout time.Duration) bool {
	if strings.TrimSpace(url) == "" {
		return false
	}
	if timeout <= 0 {
		timeout = 200 * time.Millisecond
	}
	healthCtx := ctx
	if healthCtx == nil {
		healthCtx = context.Background()
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(healthCtx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func copyGatewayConfig(cfg GatewayProcessConfig) GatewayProcessConfig {
	out := cfg
	if len(cfg.Args) > 0 {
		out.Args = append([]string(nil), cfg.Args...)
	}
	if len(cfg.Env) > 0 {
		out.Env = append([]string(nil), cfg.Env...)
	}
	return out
}
