// console implements `svrctl console` (attach an interactive session) and
// `svrctl cmd` (fire one command and exit).
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
	"github.com/adammcgrogan/svrctl/internal/tui"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

// seedLines is how much of latest.log to pre-load into the console, so
// attaching shows what just happened instead of an empty screen that only
// fills once the server next says something.
const seedLines = 200

func newConsoleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "console <name>",
		Short: "Attach to a running server's console",
		Long: "Opens the server's console. Type commands as you would in-game;\n" +
			"press esc to detach, which leaves the server running.",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return attachConsole(args[0])
		},
	}
}

// attachConsole opens the console UI, or a plain bidirectional pipe when there
// is no terminal to draw on.
func attachConsole(name string) error {
	v, err := viewServer(name)
	if err != nil {
		return err
	}
	if !v.running() {
		return fmt.Errorf("%q is not running (run `svrctl start %s` first)", name, name)
	}

	conn, err := process.OpenConsole(v.Server.Path)
	if err != nil {
		return err
	}
	defer conn.Close()

	if !ui.Interactive() {
		return pipeConsole(conn)
	}
	return tui.RunConsole(tui.ConsoleOptions{
		Name:   name,
		Detail: fmt.Sprintf("%s %s · port %d", v.Server.Type, v.Server.Version, v.port()),
		Conn:   conn,
		Seed:   tailLog(v.Server.Path, seedLines),
	})
}

// pipeConsole is the non-interactive fallback: stdin in, server output out,
// which is what a pipeline or a `screen` session wants.
func pipeConsole(conn io.ReadWriter) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		io.Copy(os.Stdout, conn)
		close(done)
	}()
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			fmt.Fprintln(conn, scanner.Text())
		}
	}()

	select {
	case <-sigCh:
	case <-done:
	}
	return nil
}

// tailLog returns the last n lines of a server's log, or nothing if it cannot
// be read — a missing log is not worth failing an attach over.
func tailLog(serverDir string, n int) []string {
	path, ok := logFileFor(serverDir)
	if !ok {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func newCmdCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "cmd <name> <command...>",
		Short:             "Send one command to a running server",
		Example:           "  svrctl cmd survival say Server restarting in 5 minutes\n  svrctl cmd survival list",
		Args:              requireMinArgs(2),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			v, err := viewServer(name)
			if err != nil {
				return err
			}
			if !v.running() {
				return fmt.Errorf("%q is not running (run `svrctl start %s` first)", name, name)
			}

			text := strings.Join(args[1:], " ")
			if err := process.SendCommand(v.Server.Path, text); err != nil {
				return err
			}
			// The control socket acknowledges delivery, not the result — the
			// server writes that to its log — so say what actually happened.
			ui.Okf(cmd.OutOrStdout(), "Sent %s", ui.Strong.Render(text))
			ui.Hintf(cmd.OutOrStdout(), "svrctl logs %s --follow", name)
			return nil
		},
	}
}
