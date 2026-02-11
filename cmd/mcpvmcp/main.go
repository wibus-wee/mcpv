package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/gateway"
	"mcpv/internal/infra/rpc"
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
	caller              string
	selectorTags        []string
	selectorServer      string
	launchUIOnFail      bool
	urlScheme           string
	transport           string
	httpAddr            string
	httpPath            string
	httpToken           string
	httpAllowedOrigins  []string
	httpJSONResponse    bool
	httpSessionTimeout  int
	httpTLSEnabled      bool
	httpTLSCertFile     string
	httpTLSKeyFile      string
	httpEventStore      bool
	httpEventStoreBytes int
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
		transport:           "streamable-http",
		httpAddr:            "127.0.0.1:8090",
		httpPath:            "/mcp",
		httpSessionTimeout:  0,
		httpEventStoreBytes: 0,
	}

	root := &cobra.Command{
		Use:   "mcpvmcp [caller]",
		Short: "MCP gateway entrypoint for streamable HTTP and stdio transports",
		Args:  cobra.MaximumNArgs(1),
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			cfg := zap.NewProductionConfig()
			log, err := cfg.Build()
			if err != nil {
				return err
			}
			opts.logger = log
			return nil
		},
		PersistentPostRun: func(_ *cobra.Command, _ []string) {
			_ = opts.logger.Sync()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			applyGatewayFlagBindings(cmd.Flags(), &opts)
			if len(args) > 0 {
				if opts.caller != "" {
					return errors.New("caller cannot be provided both as positional arg and --caller")
				}
				opts.caller = args[0]
			}

			if opts.transport == "streamable-http" {
				if opts.selectorServer != "" || len(opts.selectorTags) > 0 {
					return errors.New("selector flags are only valid for stdio transport")
				}
			}
			if opts.transport == "stdio" {
				if opts.selectorServer != "" && len(opts.selectorTags) > 0 {
					return errors.New("cannot use --selector-server and --selector-tag together")
				}
				if opts.selectorServer == "" && len(opts.selectorTags) == 0 {
					return errors.New("stdio transport requires --selector-server or --selector-tag")
				}
			}
			if opts.caller == "" {
				opts.caller = deriveCallerName()
			}
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

			gatewayTags := append([]string(nil), opts.selectorTags...)
			gatewayServer := strings.TrimSpace(opts.selectorServer)
			if opts.transport == "streamable-http" {
				gatewayTags = nil
				gatewayServer = ""
			}

			gw := gateway.NewGateway(clientCfg, opts.caller, gatewayTags, gatewayServer, opts.logger)
			var err error
			switch opts.transport {
			case "stdio":
				err = gw.Run(ctx)
			case "streamable-http":
				if err := validateHTTPGatewayOptions(opts); err != nil {
					return err
				}
				err = gw.RunStreamableHTTP(ctx, gateway.HTTPOptions{
					Addr:               opts.httpAddr,
					Path:               opts.httpPath,
					Token:              opts.httpToken,
					AllowedOrigins:     opts.httpAllowedOrigins,
					JSONResponse:       opts.httpJSONResponse,
					SessionTimeout:     time.Duration(opts.httpSessionTimeout) * time.Second,
					TLSEnabled:         opts.httpTLSEnabled,
					TLSCertFile:        opts.httpTLSCertFile,
					TLSKeyFile:         opts.httpTLSKeyFile,
					EventStoreEnabled:  opts.httpEventStore,
					EventStoreMaxBytes: opts.httpEventStoreBytes,
				})
			default:
				return fmt.Errorf("unsupported transport: %s", opts.transport)
			}
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil
				}
			}
			if err != nil && opts.launchUIOnFail && isConnectionError(err) {
				opts.logger.Info("failed to connect to mcpv, attempting to launch UI", zap.Error(err))
				if launchErr := launchmcpvUI(opts.urlScheme, opts.logger); launchErr != nil {
					opts.logger.Error("failed to launch UI", zap.Error(launchErr))
					return err // return original error
				}
				opts.logger.Info("UI launch triggered, please start mcpv and retry")
			}
			return err
		},
	}

	root.PersistentFlags().StringVar(&opts.rpcAddress, "rpc", opts.rpcAddress, "rpc address for core (unix:///tmp/mcpv.sock or host:port)")
	root.PersistentFlags().IntVar(&opts.rpcMaxRecvMsgSize, "rpc-max-recv", opts.rpcMaxRecvMsgSize, "max gRPC receive message size in bytes")
	root.PersistentFlags().IntVar(&opts.rpcMaxSendMsgSize, "rpc-max-send", opts.rpcMaxSendMsgSize, "max gRPC send message size in bytes")
	root.PersistentFlags().IntVar(&opts.rpcKeepaliveTime, "rpc-keepalive-time", opts.rpcKeepaliveTime, "gRPC keepalive time in seconds")
	root.PersistentFlags().IntVar(&opts.rpcKeepaliveTimeout, "rpc-keepalive-timeout", opts.rpcKeepaliveTimeout, "gRPC keepalive timeout in seconds")
	root.PersistentFlags().BoolVar(&opts.rpcTLSEnabled, "rpc-tls", false, "enable TLS for RPC connection")
	root.PersistentFlags().StringVar(&opts.rpcTLSCertFile, "rpc-tls-cert", "", "client TLS certificate file")
	root.PersistentFlags().StringVar(&opts.rpcTLSKeyFile, "rpc-tls-key", "", "client TLS key file")
	root.PersistentFlags().StringVar(&opts.rpcTLSCAFile, "rpc-tls-ca", "", "RPC CA file")
	root.PersistentFlags().StringVar(&opts.caller, "caller", "", "explicit caller name (optional)")
	root.PersistentFlags().StringVar(&opts.selectorServer, "selector-server", "", "server selector for stdio transport")
	root.PersistentFlags().StringArrayVar(&opts.selectorTags, "selector-tag", nil, "tag selector for stdio transport (repeatable)")
	root.PersistentFlags().BoolVar(&opts.launchUIOnFail, "launch-ui-on-fail", false, "attempt to launch mcpv UI if connection fails")
	root.PersistentFlags().StringVar(&opts.urlScheme, "url-scheme", "mcpv", "URL scheme to use for launching UI (mcpv or mcpvev)")
	root.PersistentFlags().StringVar(&opts.transport, "transport", opts.transport, "gateway transport (stdio or streamable-http)")
	root.PersistentFlags().StringVar(&opts.httpAddr, "http-addr", opts.httpAddr, "streamable HTTP listen address")
	root.PersistentFlags().StringVar(&opts.httpPath, "http-path", opts.httpPath, "streamable HTTP endpoint path")
	root.PersistentFlags().StringVar(&opts.httpToken, "http-token", "", "streamable HTTP bearer token (required for non-localhost)")
	root.PersistentFlags().StringArrayVar(&opts.httpAllowedOrigins, "http-allowed-origin", nil, "allowed CORS origin (repeatable or *)")
	root.PersistentFlags().BoolVar(&opts.httpJSONResponse, "http-json-response", false, "use application/json responses instead of SSE")
	root.PersistentFlags().IntVar(&opts.httpSessionTimeout, "http-session-timeout", opts.httpSessionTimeout, "streamable HTTP session idle timeout in seconds (0 disables)")
	root.PersistentFlags().BoolVar(&opts.httpTLSEnabled, "http-tls", false, "enable TLS for streamable HTTP listener")
	root.PersistentFlags().StringVar(&opts.httpTLSCertFile, "http-tls-cert", "", "TLS certificate file for streamable HTTP")
	root.PersistentFlags().StringVar(&opts.httpTLSKeyFile, "http-tls-key", "", "TLS key file for streamable HTTP")
	root.PersistentFlags().BoolVar(&opts.httpEventStore, "http-event-store", false, "enable in-memory event store for streamable HTTP replay")
	root.PersistentFlags().IntVar(&opts.httpEventStoreBytes, "http-event-store-bytes", opts.httpEventStoreBytes, "max bytes for in-memory event store (0 uses default)")

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
		case "caller":
			opts.caller, _ = flags.GetString("caller")
		case "selector-server":
			opts.selectorServer, _ = flags.GetString("selector-server")
		case "selector-tag":
			opts.selectorTags, _ = flags.GetStringArray("selector-tag")
		case "launch-ui-on-fail":
			opts.launchUIOnFail, _ = flags.GetBool("launch-ui-on-fail")
		case "url-scheme":
			opts.urlScheme, _ = flags.GetString("url-scheme")
		case "transport":
			opts.transport, _ = flags.GetString("transport")
		case "http-addr":
			opts.httpAddr, _ = flags.GetString("http-addr")
		case "http-path":
			opts.httpPath, _ = flags.GetString("http-path")
		case "http-token":
			opts.httpToken, _ = flags.GetString("http-token")
		case "http-allowed-origin":
			opts.httpAllowedOrigins, _ = flags.GetStringArray("http-allowed-origin")
		case "http-json-response":
			opts.httpJSONResponse, _ = flags.GetBool("http-json-response")
		case "http-session-timeout":
			opts.httpSessionTimeout, _ = flags.GetInt("http-session-timeout")
		case "http-tls":
			opts.httpTLSEnabled, _ = flags.GetBool("http-tls")
		case "http-tls-cert":
			opts.httpTLSCertFile, _ = flags.GetString("http-tls-cert")
		case "http-tls-key":
			opts.httpTLSKeyFile, _ = flags.GetString("http-tls-key")
		case "http-event-store":
			opts.httpEventStore, _ = flags.GetBool("http-event-store")
		case "http-event-store-bytes":
			opts.httpEventStoreBytes, _ = flags.GetInt("http-event-store-bytes")
		}
	})
}

func deriveCallerName() string {
	base := "mcpvmcp"
	pid := os.Getpid()
	if pid > 0 {
		return fmt.Sprintf("%s-%d", base, pid)
	}
	return base
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

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connect: no such file or directory") ||
		strings.Contains(msg, "failed to connect") ||
		strings.Contains(msg, "Unavailable")
}

func validateHTTPGatewayOptions(opts gatewayOptions) error {
	if strings.TrimSpace(opts.httpAddr) == "" {
		return errors.New("http address is required")
	}
	if !isLocalhostAddr(opts.httpAddr) && strings.TrimSpace(opts.httpToken) == "" {
		return errors.New("http token is required when binding to non-localhost address")
	}
	if opts.httpTLSEnabled {
		if strings.TrimSpace(opts.httpTLSCertFile) == "" || strings.TrimSpace(opts.httpTLSKeyFile) == "" {
			return errors.New("http tls cert and key are required when http tls is enabled")
		}
	}
	return nil
}

func isLocalhostAddr(addr string) bool {
	host := addr
	if strings.Contains(addr, ":") {
		if h, _, err := net.SplitHostPort(addr); err == nil {
			host = h
		}
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func launchmcpvUI(scheme string, logger *zap.Logger) error {
	if scheme == "" {
		scheme = "mcpv"
	}

	// Validate scheme
	if scheme != "mcpv" && scheme != "mcpvev" {
		return fmt.Errorf("invalid URL scheme: %s (must be mcpv or mcpvev)", scheme)
	}

	url := fmt.Sprintf("%s://", scheme)
	logger.Info("launching mcpv UI", zap.String("url", url))

	// Use 'open' command on macOS, 'xdg-open' on Linux, 'start' on Windows
	var cmd *exec.Cmd
	switch {
	case strings.Contains(strings.ToLower(os.Getenv("OS")), "windows"):
		cmd = exec.CommandContext(context.Background(), "cmd", "/c", "start", url)
	case fileExists("/usr/bin/open"): // macOS
		cmd = exec.CommandContext(context.Background(), "open", url)
	default: // Linux
		cmd = exec.CommandContext(context.Background(), "xdg-open", url)
	}

	return cmd.Start()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
