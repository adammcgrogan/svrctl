// server_logs implements `svrctl server logs`: prints a server's latest.log,
// optionally following new output with -f.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func newServerLogsCmd() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Show a server's log output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveServer(args[0])
			if err != nil {
				return err
			}
			logPath := filepath.Join(s.Path, "logs", "latest.log")

			f, err := os.Open(logPath)
			if err != nil {
				return fmt.Errorf("opening log file: %w", err)
			}
			defer f.Close()

			if _, err := io.Copy(cmd.OutOrStdout(), f); err != nil {
				return err
			}
			if !follow {
				return nil
			}

			for {
				if _, err := io.Copy(cmd.OutOrStdout(), f); err != nil {
					return err
				}
				time.Sleep(500 * time.Millisecond)
			}
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep streaming new log output")
	return cmd
}
