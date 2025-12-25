package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/gateway"
	"mcpd/internal/infra/rpc"
)

type gatewayOptions struct {
	rpcAddress          string
	rpcMaxRecvMsgSize   int
	rpcMaxSendMsgSize   int
	rpcKeepaliveTime    int
	rpcKeepaliveTimeout int
	rpcTLSEnabled       bool
	rpcTLSCertFile      string
	rpcTLSKeyFile       string
	rpcTLSCAFile        string
	logger              *zap.Logger
}

func main() {
	opts := gatewayOptions{
		rpcAddress:          domain.DefaultRPCListenAddress,
		rpcMaxRecvMsgSize:   domain.DefaultRPCMaxRecvMsgSize,
		rpcMaxSendMsgSize:   domain.DefaultRPCMaxSendMsgSize,
		rpcKeepaliveTime:    domain.DefaultRPCKeepaliveTimeSeconds,
		rpcKeepaliveTimeout: domain.DefaultRPCKeepaliveTimeoutSeconds,
		logger:              zap.NewNop(),
	}

	root := &cobra.Command{
		Use:   "mcpd-gateway",
		Short: "MCP gateway bridge for mcpd control plane",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg := zap.NewProductionConfig()
			log, err := cfg.Build()
			if err != nil {
				return err
			}
			opts.logger = log
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			_ = opts.logger.Sync()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			applyGatewayFlagBindings(cmd.Flags(), &opts)
			ctx, cancel := signalAwareContext(cmd.Context())
			defer cancel()

			clientCfg := rpc.ClientConfig{
				Address:                 opts.rpcAddress,
				MaxRecvMsgSize:          opts.rpcMaxRecvMsgSize,
				MaxSendMsgSize:          opts.rpcMaxSendMsgSize,
				KeepaliveTimeSeconds:    opts.rpcKeepaliveTime,
				KeepaliveTimeoutSeconds: opts.rpcKeepaliveTimeout,
				TLS: domain.RPCTLSConfig{
					Enabled:  opts.rpcTLSEnabled,
					CertFile: opts.rpcTLSCertFile,
					KeyFile:  opts.rpcTLSKeyFile,
					CAFile:   opts.rpcTLSCAFile,
				},
			}

			gw := gateway.NewGateway(clientCfg, opts.logger)
			return gw.Run(ctx)
		},
	}

	root.PersistentFlags().StringVar(&opts.rpcAddress, "rpc", opts.rpcAddress, "rpc address for core (unix:///tmp/mcpd.sock or host:port)")
	root.PersistentFlags().IntVar(&opts.rpcMaxRecvMsgSize, "rpc-max-recv", opts.rpcMaxRecvMsgSize, "max gRPC receive message size in bytes")
	root.PersistentFlags().IntVar(&opts.rpcMaxSendMsgSize, "rpc-max-send", opts.rpcMaxSendMsgSize, "max gRPC send message size in bytes")
	root.PersistentFlags().IntVar(&opts.rpcKeepaliveTime, "rpc-keepalive-time", opts.rpcKeepaliveTime, "gRPC keepalive time in seconds")
	root.PersistentFlags().IntVar(&opts.rpcKeepaliveTimeout, "rpc-keepalive-timeout", opts.rpcKeepaliveTimeout, "gRPC keepalive timeout in seconds")
	root.PersistentFlags().BoolVar(&opts.rpcTLSEnabled, "rpc-tls", false, "enable TLS for RPC connection")
	root.PersistentFlags().StringVar(&opts.rpcTLSCertFile, "rpc-tls-cert", "", "client TLS certificate file")
	root.PersistentFlags().StringVar(&opts.rpcTLSKeyFile, "rpc-tls-key", "", "client TLS key file")
	root.PersistentFlags().StringVar(&opts.rpcTLSCAFile, "rpc-tls-ca", "", "RPC CA file")

	if err := root.Execute(); err != nil {
		opts.logger.Fatal("command failed", zap.Error(err))
	}
}

func applyGatewayFlagBindings(flags *pflag.FlagSet, opts *gatewayOptions) {
	flags.Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "rpc":
			opts.rpcAddress, _ = flags.GetString("rpc")
		case "rpc-max-recv":
			opts.rpcMaxRecvMsgSize, _ = flags.GetInt("rpc-max-recv")
		case "rpc-max-send":
			opts.rpcMaxSendMsgSize, _ = flags.GetInt("rpc-max-send")
		case "rpc-keepalive-time":
			opts.rpcKeepaliveTime, _ = flags.GetInt("rpc-keepalive-time")
		case "rpc-keepalive-timeout":
			opts.rpcKeepaliveTimeout, _ = flags.GetInt("rpc-keepalive-timeout")
		case "rpc-tls":
			opts.rpcTLSEnabled, _ = flags.GetBool("rpc-tls")
		case "rpc-tls-cert":
			opts.rpcTLSCertFile, _ = flags.GetString("rpc-tls-cert")
		case "rpc-tls-key":
			opts.rpcTLSKeyFile, _ = flags.GetString("rpc-tls-key")
		case "rpc-tls-ca":
			opts.rpcTLSCAFile, _ = flags.GetString("rpc-tls-ca")
		}
	})
}

func signalAwareContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer signal.Stop(signals)
		select {
		case <-signals:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
