package transport

import (
	"context"
	"errors"

	"mcpd/internal/domain"
)

type CompositeTransport struct {
	stdio          domain.Transport
	streamableHTTP domain.Transport
}

type CompositeTransportOptions struct {
	Stdio          domain.Transport
	StreamableHTTP domain.Transport
}

func NewCompositeTransport(opts CompositeTransportOptions) *CompositeTransport {
	if opts.Stdio == nil {
		panic("composite transport requires stdio transport")
	}
	if opts.StreamableHTTP == nil {
		panic("composite transport requires streamable http transport")
	}
	return &CompositeTransport{
		stdio:          opts.Stdio,
		streamableHTTP: opts.StreamableHTTP,
	}
}

func (t *CompositeTransport) Connect(ctx context.Context, specKey string, spec domain.ServerSpec, streams domain.IOStreams) (domain.Conn, error) {
	switch domain.NormalizeTransport(spec.Transport) {
	case domain.TransportStreamableHTTP:
		return t.streamableHTTP.Connect(ctx, specKey, spec, streams)
	case domain.TransportStdio:
		return t.stdio.Connect(ctx, specKey, spec, streams)
	default:
		return nil, errors.New("unknown transport")
	}
}
