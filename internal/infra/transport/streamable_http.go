package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type StreamableHTTPTransport struct {
	logger            *zap.Logger
	listChangeEmitter domain.ListChangeEmitter
}

type StreamableHTTPTransportOptions struct {
	Logger            *zap.Logger
	ListChangeEmitter domain.ListChangeEmitter
}

func NewStreamableHTTPTransport(opts StreamableHTTPTransportOptions) *StreamableHTTPTransport {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StreamableHTTPTransport{
		logger:            logger,
		listChangeEmitter: opts.ListChangeEmitter,
	}
}

func (t *StreamableHTTPTransport) Connect(ctx context.Context, specKey string, spec domain.ServerSpec, streams domain.IOStreams) (domain.Conn, error) {
	if spec.HTTP == nil {
		return nil, errors.New("streamable http config is required")
	}
	endpoint := strings.TrimSpace(spec.HTTP.Endpoint)
	if endpoint == "" {
		return nil, errors.New("streamable http endpoint is required")
	}

	headerTransport, err := buildStreamableHTTPTransport(spec)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: headerTransport,
	}

	maxRetries := spec.HTTP.MaxRetries
	if maxRetries == 0 {
		maxRetries = domain.DefaultStreamableHTTPMaxRetries
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: client,
		MaxRetries: maxRetries,
	}
	mcpConn, err := transport.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect streamable http: %w", err)
	}

	if streams.Reader != nil {
		_ = streams.Reader.Close()
	}
	if streams.Writer != nil {
		_ = streams.Writer.Close()
	}

	return newClientConn(mcpConn, clientConnOptions{
		Logger:            t.logger.Named("mcp_http_conn"),
		ListChangeEmitter: t.listChangeEmitter,
		ServerType:        spec.Name,
		SpecKey:           specKey,
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
			return nil, errors.New("http headers contain empty key")
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
	for key, values := range h.headers {
		req.Header.Del(key)
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return h.base.RoundTrip(req)
}
