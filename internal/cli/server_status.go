// server_status implements `svrctl server status`: reports whether a single
// named server is running.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newServerStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show whether a server is running",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveServer(args[0])
			if err != nil {
				return err
			}
			if st, ok := process.IsRunning(s.Path); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: running (pid %d)\n", args[0], st.PID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: stopped\n", args[0])
			}
			return nil
		},
	}
}
