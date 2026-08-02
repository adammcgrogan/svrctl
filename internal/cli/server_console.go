// server_console implements `svrctl server console`: attaches an interactive
// session to a running server's control socket, streaming its log output and
// forwarding typed lines back as commands until the user detaches (Ctrl+C).
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newServerConsoleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "console <name>",
		Short: "Attach an interactive console to a running server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveServer(args[0])
			if err != nil {
				return err
			}

			conn, err := process.OpenConsole(s.Path)
			if err != nil {
				return err
			}
			defer conn.Close()

			fmt.Fprintln(cmd.OutOrStdout(), "Attached. Press Ctrl+C to detach (the server keeps running).")

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			done := make(chan struct{})

			go func() {
				io.Copy(cmd.OutOrStdout(), conn)
				close(done)
			}()

			go func() {
				scanner := bufio.NewScanner(cmd.InOrStdin())
				for scanner.Scan() {
					fmt.Fprintln(conn, scanner.Text())
				}
			}()

			select {
			case <-sigCh:
				fmt.Fprintln(cmd.OutOrStdout(), "\nDetached.")
			case <-done:
				fmt.Fprintln(cmd.OutOrStdout(), "\nServer connection closed.")
			}
			return nil
		},
	}
}
