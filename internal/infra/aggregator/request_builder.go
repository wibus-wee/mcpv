package aggregator

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

type requestBuilder struct {
	seq atomic.Uint64
}

func (b *requestBuilder) Build(method string, params any) (json.RawMessage, error) {
	seq := b.seq.Add(1)
	id, err := jsonrpc.MakeID(fmt.Sprintf("mcpd-%s-%d", method, seq))
	if err != nil {
		return nil, fmt.Errorf("build request id: %w", err)
	}
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	req := &jsonrpc.Request{ID: id, Method: method, Params: rawParams}
	wire, err := jsonrpc.EncodeMessage(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	return json.RawMessage(wire), nil
}
