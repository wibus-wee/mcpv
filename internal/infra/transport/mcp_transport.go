package transport

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

// MCPTransport connects to MCP servers over IO streams.
type MCPTransport struct {
	logger             *zap.Logger
	listChangeEmitter  domain.ListChangeEmitter
	samplingHandler    domain.SamplingHandler
	elicitationHandler domain.ElicitationHandler
}

// MCPTransportOptions configures the MCP transport.
type MCPTransportOptions struct {
	Logger             *zap.Logger
	ListChangeEmitter  domain.ListChangeEmitter
	SamplingHandler    domain.SamplingHandler
	ElicitationHandler domain.ElicitationHandler
}

// NewMCPTransport creates a new MCP transport.
func NewMCPTransport(opts MCPTransportOptions) *MCPTransport {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MCPTransport{
		logger:             logger,
		listChangeEmitter:  opts.ListChangeEmitter,
		samplingHandler:    opts.SamplingHandler,
		elicitationHandler: opts.ElicitationHandler,
	}
}

// Connect establishes an IO-based MCP connection for the given server spec.
func (t *MCPTransport) Connect(ctx context.Context, specKey string, spec domain.ServerSpec, streams domain.IOStreams) (domain.Conn, error) {
	if streams.Reader == nil || streams.Writer == nil {
		return nil, errors.New("streams are required")
	}

	transport := &mcp.IOTransport{
		Reader: streams.Reader,
		Writer: streams.Writer,
	}
	mcpConn, err := transport.Connect(ctx)
	if err != nil {
		if closeErr := streams.Reader.Close(); closeErr != nil {
			t.logger.Warn("close stream reader failed", zap.Error(closeErr))
		}
		if closeErr := streams.Writer.Close(); closeErr != nil {
			t.logger.Warn("close stream writer failed", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("connect io transport: %w", err)
	}

	return newClientConn(mcpConn, clientConnOptions{
		Logger:             t.logger.Named("mcp_conn"),
		ListChangeEmitter:  t.listChangeEmitter,
		SamplingHandler:    t.samplingHandler,
		ElicitationHandler: t.elicitationHandler,
		ServerType:         spec.Name,
		SpecKey:            specKey,
	}), nil
}
