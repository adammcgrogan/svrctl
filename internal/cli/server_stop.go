// server_stop implements `svrctl server stop`: graceful shutdown with a
// timeout, or an immediate kill via --force.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newServerStopCmd() *cobra.Command {
	var force bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveServer(args[0])
			if err != nil {
				return err
			}
			if err := process.Stop(s.Path, timeout, force); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stopped %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "kill the process immediately instead of asking it to shut down")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "how long to wait for graceful shutdown before failing")
	return cmd
}
