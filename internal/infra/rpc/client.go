package rpc

import (
	"context"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"time"

	"mcpd/internal/domain"
	controlv1 "mcpd/pkg/api/control/v1"
)

type ClientConfig struct {
	Address                 string
	MaxRecvMsgSize          int
	MaxSendMsgSize          int
	KeepaliveTimeSeconds    int
	KeepaliveTimeoutSeconds int
	TLS                     domain.RPCTLSConfig
}

func (c ClientConfig) keepaliveDuration() time.Duration {
	seconds := c.KeepaliveTimeSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func (c ClientConfig) keepaliveTimeout() time.Duration {
	seconds := c.KeepaliveTimeoutSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

type Client struct {
	conn   *grpc.ClientConn
	client controlv1.ControlPlaneServiceClient
}

func Dial(ctx context.Context, cfg ClientConfig) (*Client, error) {
	target, err := normalizeTargetAddress(cfg.Address)
	if err != nil {
		return nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
		grpc.WithBlock(),
	}

	if cfg.MaxRecvMsgSize > 0 || cfg.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(cfg.MaxSendMsgSize),
		))
	}

	if duration := cfg.keepaliveDuration(); duration > 0 {
		timeout := cfg.keepaliveTimeout()
		opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                duration,
			Timeout:             timeout,
			PermitWithoutStream: true,
		}))
	}

	if cfg.TLS.Enabled {
		creds, err := loadClientTLS(cfg.TLS)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		client: controlv1.NewControlPlaneServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) Control() controlv1.ControlPlaneServiceClient {
	if c == nil {
		return nil
	}
	return c.client
}
