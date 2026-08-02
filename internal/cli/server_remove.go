// server_remove implements `svrctl server remove`: unregisters a server,
// optionally deleting its files with --purge.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newServerRemoveCmd() *cobra.Command {
	var purge bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Unregister a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			reg, regPath, err := loadRegistry()
			if err != nil {
				return err
			}
			s, ok := reg.Get(name)
			if !ok {
				return fmt.Errorf("no server named %q", name)
			}

			if _, running := process.IsRunning(s.Path); running {
				return fmt.Errorf("server %q is running; stop it first with `svrctl server stop %s`", name, name)
			}

			reg.Remove(name)
			if err := reg.Save(regPath); err != nil {
				return err
			}

			if purge {
				if err := os.RemoveAll(s.Path); err != nil {
					return fmt.Errorf("deleting server files: %w", err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false, "also delete the server's files on disk")
	return cmd
}
