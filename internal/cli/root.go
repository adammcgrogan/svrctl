// Package cli wires svrctl's cobra commands together.
package cli

import (
	"github.com/spf13/cobra"
)

// NewRoot builds the root svrctl command.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "svrctl",
		Short:         "Manage local Minecraft servers",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	server := &cobra.Command{
		Use:   "server",
		Short: "Manage individual Minecraft servers",
	}
	server.AddCommand(
		newServerCreateCmd(),
		newServerListCmd(),
		newServerStartCmd(),
		newServerStopCmd(),
		newServerRestartCmd(),
		newServerConsoleCmd(),
		newServerCmdCmd(),
		newServerLogsCmd(),
		newServerStatusCmd(),
		newServerRemoveCmd(),
	)

	root.AddCommand(server, newHiddenRunCmd())
	return root
}
