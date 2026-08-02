// lifecycle implements start, stop, and restart.
package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/fetch"
	"github.com/adammcgrogan/svrctl/internal/process"
	"github.com/adammcgrogan/svrctl/internal/registry"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

// defaultStopTimeout is how long a graceful shutdown gets before the process
// is killed. Saving a large world can take a while, so this is generous.
const defaultStopTimeout = 30 * time.Second

func newStartCmd() *cobra.Command {
	var attach bool

	cmd := &cobra.Command{
		Use:               "start <name>",
		Short:             "Start a server in the background",
		Example:           "  svrctl start survival\n  svrctl start survival --attach",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			out := cmd.OutOrStdout()

			s, err := resolveServer(name)
			if err != nil {
				return err
			}
			if _, ok := process.IsRunning(s.Path); ok {
				return fmt.Errorf("%q is already running (run `svrctl console %s` to attach)", name, name)
			}

			javaPath, err := ensureJava(out, s)
			if err != nil {
				return err
			}
			if attach {
				return runAttached(s.Path, javaPath, s.Memory)
			}
			if err := process.Spawn(s.Path, name); err != nil {
				return err
			}

			ui.Okf(out, "Started %s", ui.Strong.Render(name))
			ui.Hintf(out, "svrctl console %s", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&attach, "attach", false, "run in the foreground, tied to this terminal (stops when you do)")
	return cmd
}

func newStopCmd() *cobra.Command {
	var force bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:               "stop <name>",
		Short:             "Stop a running server",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			v, err := viewServer(name)
			if err != nil {
				return err
			}
			if !v.running() {
				ui.Warnf(cmd.OutOrStdout(), "%s is already stopped", name)
				return nil
			}
			if !force {
				ui.Stepf(cmd.OutOrStdout(), "Asking %s to save and shut down…", name)
			}
			if err := process.Stop(v.Server.Path, timeout, force); err != nil {
				return err
			}
			ui.Okf(cmd.OutOrStdout(), "Stopped %s", ui.Strong.Render(name))
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "kill it immediately, without letting it save first")
	cmd.Flags().DurationVar(&timeout, "timeout", defaultStopTimeout, "how long to wait for a graceful shutdown")
	return cmd
}

func newRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "restart <name>",
		Short:             "Stop and start a server",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			out := cmd.OutOrStdout()
			if err := restartServer(out, name); err != nil {
				return err
			}
			ui.Okf(out, "Restarted %s", ui.Strong.Render(name))
			return nil
		},
	}
	return cmd
}

// restartServer is the shared stop-then-start used by the command and the
// dashboard's "r" key.
func restartServer(out io.Writer, name string) error {
	s, err := resolveServer(name)
	if err != nil {
		return err
	}
	if err := process.Stop(s.Path, defaultStopTimeout, false); err != nil {
		return err
	}
	if _, err := ensureJava(out, s); err != nil {
		return err
	}
	return process.Spawn(s.Path, name)
}

// startServer is the shared start used by the command and the dashboard.
func startServer(out io.Writer, name string) error {
	s, err := resolveServer(name)
	if err != nil {
		return err
	}
	if _, ok := process.IsRunning(s.Path); ok {
		return fmt.Errorf("%q is already running", name)
	}
	if _, err := ensureJava(out, s); err != nil {
		return err
	}
	return process.Spawn(s.Path, name)
}

// stopServer is the shared stop used by the command and the dashboard.
func stopServer(name string) error {
	s, err := resolveServer(name)
	if err != nil {
		return err
	}
	return process.Stop(s.Path, defaultStopTimeout, false)
}

// ensureJava resolves the server's JDK, reporting progress if it has to be
// downloaded. Starting a server usually hits a warm cache and prints nothing;
// the one time it does not, this is a multi-minute wait and the user gets to
// see why rather than staring at a dead prompt.
func ensureJava(out io.Writer, s registry.Server) (string, error) {
	var render func(fetch.Progress)
	if ui.IsTerminal(os.Stdout) {
		render = plainProgress(out)
	}

	announced := false
	report := func(p fetch.Progress) {
		if !announced {
			announced = true
			ui.Stepf(out, "Downloading the Java runtime this server needs (one time only)…")
		}
		if render != nil {
			render(p)
		}
	}
	return resolveJavaPath(s, report)
}
