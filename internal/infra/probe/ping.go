package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"

	"mcpd/internal/domain"
)

var pingIDSeq atomic.Uint64

type PingProbe struct {
	Timeout time.Duration
}

func (p *PingProbe) Ping(ctx context.Context, conn domain.Conn) error {
	if conn == nil {
		return errors.New("connection is nil")
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	seq := pingIDSeq.Add(1)
	id, err := jsonrpc.MakeID(fmt.Sprintf("ping-%d", seq))
	if err != nil {
		return fmt.Errorf("build ping id: %w", err)
	}

	wireMsg := &jsonrpc.Request{
		ID:     id,
		Method: "ping",
		Params: json.RawMessage(`{}`),
	}

	wire, err := jsonrpc.EncodeMessage(wireMsg)
	if err != nil {
		return fmt.Errorf("encode ping: %w", err)
	}

	if err := conn.Send(pingCtx, wire); err != nil {
		return fmt.Errorf("send ping: %w", err)
	}

	rawResp, err := conn.Recv(pingCtx)
	if err != nil {
		return fmt.Errorf("recv ping: %w", err)
	}

	respMsg, err := jsonrpc.DecodeMessage(rawResp)
	if err != nil {
		return fmt.Errorf("decode ping response: %w", err)
	}

	resp, ok := respMsg.(*jsonrpc.Response)
	if !ok {
		return errors.New("ping response is not a response message")
	}
	if resp.Error != nil {
		return fmt.Errorf("ping error: %w", resp.Error)
	}

	return nil
}
