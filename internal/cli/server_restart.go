// server_restart implements `svrctl server restart`: a graceful stop
// followed by a detached start.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newServerRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := resolveServer(name)
			if err != nil {
				return err
			}

			if err := process.Stop(s.Path, 30*time.Second, false); err != nil {
				return err
			}

			if _, err := resolveJavaPath(s); err != nil {
				return err
			}
			if err := process.Spawn(s.Path, name); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Restarted %q\n", name)
			return nil
		},
	}
	return cmd
}
