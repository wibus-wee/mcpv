package transport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry/diagnostics"
)

// MCPTransport connects to MCP servers over IO streams.
type MCPTransport struct {
	logger             *zap.Logger
	listChangeEmitter  domain.ListChangeEmitter
	samplingHandler    domain.SamplingHandler
	elicitationHandler domain.ElicitationHandler
	probe              diagnostics.Probe
}

// MCPTransportOptions configures the MCP transport.
type MCPTransportOptions struct {
	Logger             *zap.Logger
	ListChangeEmitter  domain.ListChangeEmitter
	SamplingHandler    domain.SamplingHandler
	ElicitationHandler domain.ElicitationHandler
	Probe              diagnostics.Probe
}

// NewMCPTransport creates a new MCP transport.
func NewMCPTransport(opts MCPTransportOptions) *MCPTransport {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	probe := opts.Probe
	if probe == nil {
		probe = diagnostics.NoopProbe{}
	}
	return &MCPTransport{
		logger:             logger,
		listChangeEmitter:  opts.ListChangeEmitter,
		samplingHandler:    opts.SamplingHandler,
		elicitationHandler: opts.ElicitationHandler,
		probe:              probe,
	}
}

// Connect establishes an IO-based MCP connection for the given server spec.
func (t *MCPTransport) Connect(ctx context.Context, specKey string, spec domain.ServerSpec, streams domain.IOStreams) (domain.Conn, error) {
	started := time.Now()
	attemptID, _ := diagnostics.AttemptIDFromContext(ctx)
	attrs := map[string]string{
		"transport": string(domain.TransportStdio),
	}
	if streams.Reader == nil {
		attrs["reader"] = "nil"
	}
	if streams.Writer == nil {
		attrs["writer"] = "nil"
	}
	t.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepTransportConnect,
		Phase:      diagnostics.PhaseEnter,
		Timestamp:  started,
		Attributes: attrs,
	})
	if streams.Reader == nil || streams.Writer == nil {
		t.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepTransportConnect,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      "streams are required",
			Attributes: attrs,
		})
		return nil, errors.New("streams are required")
	}

	transport := &mcp.IOTransport{
		Reader: streams.Reader,
		Writer: streams.Writer,
	}
	mcpConn, err := transport.Connect(ctx)
	if err != nil {
		t.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepTransportConnect,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("connect io transport: %w", err).Error(),
			Attributes: attrs,
		})
		if closeErr := streams.Reader.Close(); closeErr != nil {
			t.logger.Warn("close stream reader failed", zap.Error(closeErr))
		}
		if closeErr := streams.Writer.Close(); closeErr != nil {
			t.logger.Warn("close stream writer failed", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("connect io transport: %w", err)
	}
	t.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepTransportConnect,
		Phase:      diagnostics.PhaseExit,
		Timestamp:  time.Now(),
		Duration:   time.Since(started),
		Attributes: attrs,
	})

	return newClientConn(mcpConn, clientConnOptions{
		Logger:             t.logger.Named("mcp_conn"),
		ListChangeEmitter:  t.listChangeEmitter,
		SamplingHandler:    t.samplingHandler,
		ElicitationHandler: t.elicitationHandler,
		ServerType:         spec.Name,
		SpecKey:            specKey,
	}), nil
}

func (t *MCPTransport) recordEvent(event diagnostics.Event) {
	if t == nil || t.probe == nil {
		return
	}
	t.probe.Record(event)
}
