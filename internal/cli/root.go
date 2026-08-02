// Package cli wires svrctl's cobra commands together.
//
// The commands are flat: `svrctl start survival`, not `svrctl server start
// survival`. There is only one kind of noun in this tool, so naming it in
// every invocation was pure ceremony. The old `server ...` forms still work as
// hidden aliases so existing scripts and muscle memory keep working.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/tui"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

// NewRoot builds the root svrctl command.
func NewRoot() *cobra.Command {
	var plain bool

	root := &cobra.Command{
		Use:   "svrctl",
		Short: "Manage local Minecraft servers",
		Long: "svrctl creates, runs, and monitors Minecraft servers on this machine.\n\n" +
			"Run it with no arguments to open the dashboard, where you can start,\n" +
			"stop, and attach to servers without memorising any of the commands below.",
		Example: "  svrctl                    open the dashboard\n" +
			"  svrctl create             set up a new server, step by step\n" +
			"  svrctl start survival     start a server in the background\n" +
			"  svrctl console survival   attach to its console",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			ui.SetPlain(plain)
		},
		// With no subcommand, open the dashboard on a terminal and print help
		// anywhere else, so `svrctl | cat` and CI runs still behave sensibly.
		RunE: func(cmd *cobra.Command, args []string) error {
			if !ui.Interactive() {
				return cmd.Help()
			}
			return runDashboard()
		},
	}

	root.PersistentFlags().BoolVar(&plain, "plain", false,
		"disable interactive screens and colour (implied when output is not a terminal)")

	root.AddCommand(
		newCreateCmd(),
		newListCmd(),
		newStatusCmd(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newConsoleCmd(),
		newCmdCmd(),
		newLogsCmd(),
		newVersionsCmd(),
		newRemoveCmd(),
		newLegacyServerCmd(),
		newHiddenRunCmd(),
	)
	return root
}

// newLegacyServerCmd re-exposes every command under the old `svrctl server
// ...` spelling. It is hidden from help so new users only ever meet the flat
// form, but nothing that already works stops working.
func newLegacyServerCmd() *cobra.Command {
	server := &cobra.Command{
		Use:    "server",
		Short:  "Deprecated: use the top-level commands instead",
		Hidden: true,
	}
	server.AddCommand(
		newCreateCmd(),
		newListCmd(),
		newStatusCmd(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newConsoleCmd(),
		newCmdCmd(),
		newLogsCmd(),
		newRemoveCmd(),
	)
	return server
}

// runDashboard opens the interactive dashboard, then carries out whatever the
// user asked for on the way out. Attaching a console or running the create
// wizard means handing the terminal to a second Bubble Tea program, which can
// only happen once the first has released it — hence the loop back into the
// dashboard when that program finishes.
func runDashboard() error {
	for {
		next, err := tui.RunDashboard(dashboardDeps())
		if err != nil {
			return err
		}
		switch next.Action {
		case tui.ActionQuit:
			return nil
		case tui.ActionConsole:
			if err := attachConsole(next.Server); err != nil {
				reportError(err)
			}
		case tui.ActionCreate:
			if err := runCreateWizard(os.Stdout, ProvisionSpec{}); err != nil {
				reportError(err)
			}
		}
	}
}

// reportError prints a failure from a dashboard-launched action without
// tearing down the dashboard loop.
func reportError(err error) {
	fmt.Fprintln(os.Stderr, ui.Failure.Render(ui.GlyphFail)+" "+err.Error())
}
