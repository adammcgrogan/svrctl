// server_start implements `svrctl server start`: launches a server either
// detached (default) or attached to the current terminal (--attach).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newServerStartCmd() *cobra.Command {
	var attach bool

	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := resolveServer(name)
			if err != nil {
				return err
			}

			if _, ok := process.IsRunning(s.Path); ok {
				return fmt.Errorf("server %q is already running", name)
			}

			javaPath, err := resolveJavaPath(s)
			if err != nil {
				return err
			}

			if attach {
				return runAttached(s.Path, javaPath, s.Memory)
			}

			if err := process.Spawn(s.Path, name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Started %q. Use `svrctl server console %s` to attach.\n", name, name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&attach, "attach", false, "run in the foreground with console attached to this terminal")
	return cmd
}
