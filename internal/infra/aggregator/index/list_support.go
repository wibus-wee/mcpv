package index

import (
	"errors"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"

	"mcpv/internal/domain"
)

type listSupportTracker struct {
	mu          sync.RWMutex
	unsupported map[string]struct{}
}

func newListSupportTracker() *listSupportTracker {
	return &listSupportTracker{unsupported: make(map[string]struct{})}
}

func (t *listSupportTracker) IsUnsupported(serverType string) bool {
	if t == nil || serverType == "" {
		return false
	}
	t.mu.RLock()
	_, ok := t.unsupported[serverType]
	t.mu.RUnlock()
	return ok
}

func (t *listSupportTracker) MarkUnsupported(serverType string) {
	if t == nil || serverType == "" {
		return
	}
	t.mu.Lock()
	t.unsupported[serverType] = struct{}{}
	t.mu.Unlock()
}

func (t *listSupportTracker) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.unsupported = make(map[string]struct{})
	t.mu.Unlock()
}

func isListMethodUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrMethodNotAllowed) {
		return true
	}
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeMethodNotFound {
		return true
	}
	return false
}
