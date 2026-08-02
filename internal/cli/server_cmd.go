// server_cmd implements `svrctl server cmd`: sends a single console command
// to a running server without attaching an interactive session.
package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newServerCmdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cmd <name> <command...>",
		Short: "Send a single command to a running server",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveServer(args[0])
			if err != nil {
				return err
			}
			return process.SendCommand(s.Path, strings.Join(args[1:], " "))
		},
	}
}
