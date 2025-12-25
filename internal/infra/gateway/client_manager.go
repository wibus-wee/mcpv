package gateway

import (
	"context"
	"sync"

	"mcpd/internal/infra/rpc"
)

type clientManager struct {
	cfg rpc.ClientConfig

	mu     sync.Mutex
	client *rpc.Client
}

func newClientManager(cfg rpc.ClientConfig) *clientManager {
	return &clientManager{
		cfg: cfg,
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
		_ = m.client.Close()
		m.client = nil
	}
	m.mu.Unlock()
}

func (m *clientManager) close() {
	m.reset()
}
