package main

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"mcpv/internal/infra/daemon"
)

type daemonArgs struct {
	configPath string
	logPath    string
	binaryPath string
}

func newDaemonCmd(opts *cliOptions) *cobra.Command {
	args := &daemonArgs{
		configPath: "runtime.yaml",
	}
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the mcpv system service",
	}
	cmd.PersistentFlags().StringVar(&args.configPath, "config", args.configPath, "path to config file")
	cmd.PersistentFlags().StringVar(&args.logPath, "log-file", "", "path to daemon log file (optional)")
	cmd.PersistentFlags().StringVar(&args.binaryPath, "mcpv-binary", "", "path to mcpv binary (optional)")

	cmd.AddCommand(
		newDaemonInstallCmd(opts, args),
		newDaemonUninstallCmd(opts, args),
		newDaemonStartCmd(opts, args),
		newDaemonStopCmd(opts, args),
		newDaemonStatusCmd(opts, args),
		newDaemonRestartCmd(opts, args),
	)
	return cmd
}

func newDaemonInstallCmd(opts *cliOptions, args *daemonArgs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the mcpv system service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := newDaemonManager(opts, args)
			if err != nil {
				return err
			}
			status, err := manager.Install(cmd.Context())
			if err != nil {
				return err
			}
			return printDaemonAction("installed", status, opts.jsonOutput)
		},
	}
	return cmd
}

func newDaemonUninstallCmd(opts *cliOptions, args *daemonArgs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the mcpv system service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := newDaemonManager(opts, args)
			if err != nil {
				return err
			}
			status, err := manager.Uninstall(cmd.Context())
			if err != nil {
				return err
			}
			return printDaemonAction("uninstalled", status, opts.jsonOutput)
		},
	}
	return cmd
}

func newDaemonStartCmd(opts *cliOptions, args *daemonArgs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the mcpv system service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := newDaemonManager(opts, args)
			if err != nil {
				return err
			}
			status, err := manager.Start(cmd.Context())
			if err != nil {
				return err
			}
			return printDaemonAction("started", status, opts.jsonOutput)
		},
	}
	return cmd
}

func newDaemonStopCmd(opts *cliOptions, args *daemonArgs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the mcpv system service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := newDaemonManager(opts, args)
			if err != nil {
				return err
			}
			status, err := manager.Stop(cmd.Context())
			if err != nil {
				if errors.Is(err, daemon.ErrNotInstalled) {
					if fallback, statusErr := manager.Status(cmd.Context()); statusErr == nil {
						status = fallback
					}
					_ = printDaemonAction("not installed", status, opts.jsonOutput)
					return exitSilent(4)
				}
				return err
			}
			return printDaemonAction("stopped", status, opts.jsonOutput)
		},
	}
	return cmd
}

func newDaemonStatusCmd(opts *cliOptions, args *daemonArgs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show mcpv system service status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := newDaemonManager(opts, args)
			if err != nil {
				return err
			}
			status, err := manager.Status(cmd.Context())
			if err != nil {
				return err
			}
			if err := printDaemonStatus(status, opts.jsonOutput); err != nil {
				return err
			}
			if !status.Installed {
				return exitSilent(4)
			}
			if !status.Running {
				return exitSilent(3)
			}
			return nil
		},
	}
	return cmd
}

func newDaemonRestartCmd(opts *cliOptions, args *daemonArgs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the mcpv system service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := newDaemonManager(opts, args)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if _, err := manager.Stop(ctx); err != nil && !errors.Is(err, daemon.ErrNotInstalled) {
				return err
			}
			status, err := manager.Start(ctx)
			if err != nil {
				return err
			}
			return printDaemonAction("restarted", status, opts.jsonOutput)
		},
	}
	return cmd
}

func newDaemonManager(opts *cliOptions, args *daemonArgs) (*daemon.Manager, error) {
	return daemon.NewManager(daemon.Options{
		BinaryPath: strings.TrimSpace(args.binaryPath),
		ConfigPath: strings.TrimSpace(args.configPath),
		RPCAddress: strings.TrimSpace(opts.rpcAddress),
		LogPath:    strings.TrimSpace(args.logPath),
	})
}
