package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"

	"mcpd/internal/domain"
	"mcpd/internal/infra/fsutil"
	controlv1 "mcpd/pkg/api/control/v1"
)

type Server struct {
	cfg        domain.RPCConfig
	control    domain.ControlPlane
	logger     *zap.Logger
	grpcServer *grpc.Server
	listener   net.Listener
	health     *health.Server
	network    string
	address    string
}

func NewServer(control domain.ControlPlane, cfg domain.RPCConfig, logger *zap.Logger) *Server {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Server{
		cfg:     cfg,
		control: control,
		logger:  logger.Named("rpc"),
	}
}

func (s *Server) Run(ctx context.Context) error {
	if s.control == nil {
		return errors.New("control plane is nil")
	}

	network, addr, err := parseListenAddress(s.cfg.ListenAddress)
	if err != nil {
		return err
	}
	s.network = network
	s.address = addr

	if network == "unix" {
		if err := os.MkdirAll(filepath.Dir(addr), fsutil.DefaultDirMode); err != nil {
			return fmt.Errorf("create rpc socket dir: %w", err)
		}
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove rpc socket: %w", err)
		}
	}

	lis, err := net.Listen(network, addr)
	if err != nil {
		return fmt.Errorf("listen rpc: %w", err)
	}
	s.listener = lis

	if network == "unix" {
		mode, err := resolveSocketMode(s.cfg.SocketMode)
		if err != nil {
			return err
		}
		if mode != 0 {
			if err := os.Chmod(addr, mode); err != nil {
				return fmt.Errorf("chmod rpc socket: %w", err)
			}
		}
	}

	serverOpts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.ChainStreamInterceptor(otelgrpc.StreamServerInterceptor()),
		grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.cfg.MaxSendMsgSize),
	}

	if s.cfg.KeepaliveTimeSeconds > 0 {
		serverOpts = append(serverOpts, grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    time.Duration(s.cfg.KeepaliveTimeSeconds) * time.Second,
			Timeout: time.Duration(s.cfg.KeepaliveTimeoutSeconds) * time.Second,
		}))
	}

	if s.cfg.TLS.Enabled {
		creds, err := loadServerTLS(s.cfg.TLS)
		if err != nil {
			return err
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
	}

	s.grpcServer = grpc.NewServer(serverOpts...)
	s.health = health.NewServer()
	grpc_health_v1.RegisterHealthServer(s.grpcServer, s.health)
	controlv1.RegisterControlPlaneServiceServer(s.grpcServer, NewControlService(s.control, s.logger))
	s.health.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.grpcServer.Serve(lis)
	}()

	s.logger.Info("rpc server started", zap.String("network", network), zap.String("address", addr))

	select {
	case <-ctx.Done():
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.Stop(stopCtx)
	case err := <-errCh:
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Stop(stopCtx)
		return err
	}
}

func (s *Server) Stop(ctx context.Context) error {
	if s.grpcServer == nil {
		return nil
	}
	if s.health != nil {
		s.health.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	}

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-ctx.Done():
		s.grpcServer.Stop()
		return ctx.Err()
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}
	if s.network == "unix" && s.address != "" {
		_ = os.Remove(s.address)
	}
	return nil
}
