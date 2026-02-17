package main

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

type cliOptions struct {
	rpcAddress          string
	rpcMaxRecvMsgSize   int
	rpcMaxSendMsgSize   int
	rpcKeepaliveTime    int
	rpcKeepaliveTimeout int
	rpcTLSEnabled       bool
	rpcTLSCertFile      string
	rpcTLSKeyFile       string
	rpcTLSCAFile        string
	rpcToken            string
	rpcTokenEnv         string
	caller              string
	tags                []string
	server              string
	noRegister          bool
	jsonOutput          bool
	logger              *zap.Logger
}

func newRootCommand() *cobra.Command {
	opts := cliOptions{
		rpcAddress:          domain.DefaultRPCListenAddress,
		rpcMaxRecvMsgSize:   domain.DefaultRPCMaxRecvMsgSize,
		rpcMaxSendMsgSize:   domain.DefaultRPCMaxSendMsgSize,
		rpcKeepaliveTime:    domain.DefaultRPCKeepaliveTimeSeconds,
		rpcKeepaliveTimeout: domain.DefaultRPCKeepaliveTimeoutSeconds,
		logger:              zap.NewNop(),
	}

	root := &cobra.Command{
		Use:   "mcpvctl",
		Short: "CLI client for the mcpv control plane",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			applyRootFlagBindings(cmd, &opts)
			return validateSelectorFlags(&opts)
		},
		PersistentPostRun: func(_ *cobra.Command, _ []string) {
			_ = opts.logger.Sync()
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
	root.PersistentFlags().StringVar(&opts.rpcToken, "rpc-token", "", "RPC bearer token (token auth)")
	root.PersistentFlags().StringVar(&opts.rpcTokenEnv, "rpc-token-env", "", "RPC bearer token env var (token auth)")
	root.PersistentFlags().StringVar(&opts.caller, "caller", "", "explicit caller name (optional)")
	root.PersistentFlags().StringArrayVar(&opts.tags, "tag", nil, "tag selector (repeatable)")
	root.PersistentFlags().StringVar(&opts.server, "server", "", "server selector (mutually exclusive with --tag)")
	root.PersistentFlags().BoolVar(&opts.noRegister, "no-register", false, "skip auto register/unregister")
	root.PersistentFlags().BoolVar(&opts.jsonOutput, "json", false, "output JSON")

	root.AddCommand(
		newInfoCmd(&opts),
		newRegisterCmd(&opts),
		newUnregisterCmd(&opts),
		newDaemonCmd(&opts),
		newToolsCmd(&opts),
		newTasksCmd(&opts),
		newResourcesCmd(&opts),
		newPromptsCmd(&opts),
		newLogsCmd(&opts),
		newRuntimeCmd(&opts),
		newInitCmd(&opts),
		newSubAgentCmd(&opts),
	)

	return root
}

func applyRootFlagBindings(cmd *cobra.Command, opts *cliOptions) {
	flags := cmd.Flags()
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
		case "rpc-token":
			opts.rpcToken, _ = flags.GetString("rpc-token")
		case "rpc-token-env":
			opts.rpcTokenEnv, _ = flags.GetString("rpc-token-env")
		case "caller":
			opts.caller, _ = flags.GetString("caller")
		case "tag":
			opts.tags, _ = flags.GetStringArray("tag")
		case "server":
			opts.server, _ = flags.GetString("server")
		case "no-register":
			opts.noRegister, _ = flags.GetBool("no-register")
		case "json":
			opts.jsonOutput, _ = flags.GetBool("json")
		}
	})
}

func validateSelectorFlags(opts *cliOptions) error {
	if strings.TrimSpace(opts.server) != "" && len(opts.tags) > 0 {
		return errors.New("--server and --tag are mutually exclusive")
	}
	return nil
}
