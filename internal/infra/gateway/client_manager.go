package gateway

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"mcpv/internal/infra/rpc"
)

type clientManager struct {
	cfg    rpc.ClientConfig
	logger *zap.Logger

	mu     sync.Mutex
	client *rpc.Client
}

func newClientManager(cfg rpc.ClientConfig, logger *zap.Logger) *clientManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &clientManager{
		cfg:    cfg,
		logger: logger.Named("gateway_client"),
	}
}

func (m *clientManager) get(ctx context.Context) (*rpc.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		return m.client, nil
	}
	client, err := rpc.Dial(ctx, m.cfg)
	if err != nil {
		return nil, err
	}
	m.client = client
	return client, nil
}

func (m *clientManager) reset() {
	m.mu.Lock()
	if m.client != nil {
		if err := m.client.Close(); err != nil {
			m.logger.Warn("rpc client close failed", zap.Error(err))
		}
		m.client = nil
	}
	m.mu.Unlock()
}

func (m *clientManager) close() {
	m.reset()
}
