package cmd

import (
	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/bridge"
	"github.com/miltonparedes/kitmux/internal/openlocal"
)

func addBridgeCommand(parent *cobra.Command) {
	var socketPath string

	bridgeCmd := &cobra.Command{
		Use:   "bridge",
		Short: "Local editor bridge (runs on your local machine)",
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the bridge listener on a Unix socket",
		RunE: func(_ *cobra.Command, _ []string) error {
			return bridge.Serve(socketPath)
		},
	}
	serveCmd.Flags().StringVar(&socketPath, "socket", openlocal.ResolveSocketPath(),
		"Unix socket path")

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install macOS LaunchAgent for on-demand bridge",
		RunE: func(_ *cobra.Command, _ []string) error {
			return bridge.InstallLaunchAgent(socketPath)
		},
	}
	installCmd.Flags().StringVar(&socketPath, "socket", openlocal.ResolveSocketPath(),
		"Unix socket path")

	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall macOS LaunchAgent",
		RunE: func(_ *cobra.Command, _ []string) error {
			return bridge.UninstallLaunchAgent()
		},
	}

	bridgeCmd.AddCommand(serveCmd, installCmd, uninstallCmd)
	parent.AddCommand(bridgeCmd)
}
