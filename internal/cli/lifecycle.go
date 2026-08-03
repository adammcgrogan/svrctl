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
	var group string

	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a server in the background",
		Long: "Starts a server in the background.\n\n" +
			"Pass --group instead of a name to start every server tagged with it.",
		Example:           "  svrctl start survival\n  svrctl start survival --attach\n  svrctl start --group network",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := targetNames(args, group)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			runOne := func(name string) error {
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
			}

			if len(names) == 1 {
				return runOne(names[0])
			}
			if attach {
				return fmt.Errorf("--attach only works for a single server, not --group")
			}
			return forEachTarget(names, runOne)
		},
	}

	cmd.Flags().BoolVar(&attach, "attach", false, "run in the foreground, tied to this terminal (stops when you do)")
	cmd.Flags().StringVar(&group, "group", "", "start every server in this group instead of one by name")
	return cmd
}

func newStopCmd() *cobra.Command {
	var force bool
	var timeout time.Duration
	var group string

	cmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running server",
		Long: "Stops a running server.\n\n" +
			"Pass --group instead of a name to stop every server tagged with it.",
		Example:           "  svrctl stop survival\n  svrctl stop --group network",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := targetNames(args, group)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			runOne := func(name string) error {
				v, err := viewServer(name)
				if err != nil {
					return err
				}
				if !v.running() {
					ui.Warnf(out, "%s is already stopped", name)
					return nil
				}
				if !force {
					ui.Stepf(out, "Asking %s to save and shut down…", name)
				}
				if err := process.Stop(v.Server.Path, timeout, force); err != nil {
					return err
				}
				ui.Okf(out, "Stopped %s", ui.Strong.Render(name))
				return nil
			}

			if len(names) == 1 {
				return runOne(names[0])
			}
			return forEachTarget(names, runOne)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "kill it immediately, without letting it save first")
	cmd.Flags().DurationVar(&timeout, "timeout", defaultStopTimeout, "how long to wait for a graceful shutdown")
	cmd.Flags().StringVar(&group, "group", "", "stop every server in this group instead of one by name")
	return cmd
}

func newRestartCmd() *cobra.Command {
	var group string

	cmd := &cobra.Command{
		Use:   "restart <name>",
		Short: "Stop and start a server",
		Long: "Stops and starts a server.\n\n" +
			"Pass --group instead of a name to restart every server tagged with it.",
		Example:           "  svrctl restart survival\n  svrctl restart --group network",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := targetNames(args, group)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			runOne := func(name string) error {
				if err := restartServer(out, name); err != nil {
					return err
				}
				ui.Okf(out, "Restarted %s", ui.Strong.Render(name))
				return nil
			}

			if len(names) == 1 {
				return runOne(names[0])
			}
			return forEachTarget(names, runOne)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "restart every server in this group instead of one by name")
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
