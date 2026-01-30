package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

// StreamableHTTPTransport connects to MCP servers over streamable HTTP.
type StreamableHTTPTransport struct {
	logger             *zap.Logger
	listChangeEmitter  domain.ListChangeEmitter
	samplingHandler    domain.SamplingHandler
	elicitationHandler domain.ElicitationHandler
}

// StreamableHTTPTransportOptions configures the streamable HTTP transport.
type StreamableHTTPTransportOptions struct {
	Logger             *zap.Logger
	ListChangeEmitter  domain.ListChangeEmitter
	SamplingHandler    domain.SamplingHandler
	ElicitationHandler domain.ElicitationHandler
}

// NewStreamableHTTPTransport creates a streamable HTTP transport for MCP.
func NewStreamableHTTPTransport(opts StreamableHTTPTransportOptions) *StreamableHTTPTransport {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StreamableHTTPTransport{
		logger:             logger,
		listChangeEmitter:  opts.ListChangeEmitter,
		samplingHandler:    opts.SamplingHandler,
		elicitationHandler: opts.ElicitationHandler,
	}
}

// Connect establishes a streamable HTTP connection for the given server spec.
func (t *StreamableHTTPTransport) Connect(ctx context.Context, specKey string, spec domain.ServerSpec, streams domain.IOStreams) (domain.Conn, error) {
	if spec.HTTP == nil {
		return nil, fmt.Errorf("server %s: streamable http config is required", spec.Name)
	}
	endpoint := strings.TrimSpace(spec.HTTP.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("server %s: streamable http endpoint is required", spec.Name)
	}

	headerTransport, err := buildStreamableHTTPTransport(spec)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: headerTransport,
	}

	// Loader defaults MaxRetries, but keep a defensive fallback for unnormalized specs.
	maxRetries := effectiveMaxRetries(spec.HTTP.MaxRetries)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: client,
		MaxRetries: maxRetries,
	}
	mcpConn, err := transport.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect streamable http: %w", err)
	}

	if streams.Reader != nil || streams.Writer != nil {
		t.logger.Warn("streamable http transport ignores IO streams",
			zap.String("server", spec.Name),
		)
	}
	if streams.Reader != nil {
		if err := streams.Reader.Close(); err != nil {
			t.logger.Warn("close stream reader failed", zap.Error(err))
		}
	}
	if streams.Writer != nil {
		if err := streams.Writer.Close(); err != nil {
			t.logger.Warn("close stream writer failed", zap.Error(err))
		}
	}

	return newClientConn(mcpConn, clientConnOptions{
		Logger:             t.logger.Named("mcp_http_conn"),
		ListChangeEmitter:  t.listChangeEmitter,
		SamplingHandler:    t.samplingHandler,
		ElicitationHandler: t.elicitationHandler,
		ServerType:         spec.Name,
		SpecKey:            specKey,
	}), nil
}

func buildStreamableHTTPTransport(spec domain.ServerSpec) (http.RoundTripper, error) {
	headers := http.Header{}
	if spec.ProtocolVersion != "" {
		headers.Set("Mcp-Protocol-Version", spec.ProtocolVersion)
	}
	for key, value := range spec.HTTP.Headers {
		name := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if name == "" {
			return nil, fmt.Errorf("server %s: http headers contain empty key", spec.Name)
		}
		headers.Set(name, value)
	}

	base := http.DefaultTransport
	if base == nil {
		return nil, errors.New("default http transport is nil")
	}

	return &headerRoundTripper{
		base:    base,
		headers: headers,
	}, nil
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers http.Header
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	// Explicitly overwrite any existing values with configured headers.
	for key, values := range h.headers {
		cloned.Header.Del(key)
		for _, value := range values {
			cloned.Header.Add(key, value)
		}
	}
	return h.base.RoundTrip(cloned)
}

func effectiveMaxRetries(value int) int {
	if value == 0 {
		return domain.DefaultStreamableHTTPMaxRetries
	}
	return value
}
