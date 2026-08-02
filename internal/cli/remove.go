// remove implements `svrctl remove`: unregistering a server and, on request,
// deleting its files.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

func newRemoveCmd() *cobra.Command {
	var purge, yes bool

	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Unregister a server",
		Long: "Removes a server from svrctl. By default the files are left alone and\n" +
			"only the registry entry goes away, so the world can be re-added later.\n" +
			"Pass --purge to delete the server directory as well.",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			out := cmd.OutOrStdout()

			reg, _, err := loadRegistry()
			if err != nil {
				return err
			}
			s, ok := reg.Get(name)
			if !ok {
				return unknownServerError(reg, name)
			}
			v := buildView(name, s)
			if v.running() {
				return fmt.Errorf("%q is running — stop it first with `svrctl stop %s`", name, name)
			}

			// --purge destroys worlds. Confirm it unless the caller has said
			// in advance that they mean it.
			if purge && !yes {
				if !ui.Interactive() {
					return fmt.Errorf("refusing to delete %s without confirmation; pass --yes to proceed", s.Path)
				}
				if err := confirmPurge(cmd.InOrStdin(), out, name, s.Path); err != nil {
					return err
				}
			}

			if err := removeServer(name, purge); err != nil {
				return err
			}
			if purge {
				ui.Okf(out, "Removed %s and deleted its files", ui.Strong.Render(name))
				return nil
			}

			ui.Okf(out, "Removed %s", ui.Strong.Render(name))
			fmt.Fprintln(out, ui.Subtle.Render("  its files are still at "+s.Path))
			return nil
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false, "also delete the server's files, including its worlds")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt for --purge")
	return cmd
}

// removeServer unregisters a server and, if purge is set, deletes its files.
// It re-checks existence and running state itself rather than trusting a
// caller's earlier look, since the dashboard's confirmation step leaves a
// window for the server to have started or vanished in the meantime.
func removeServer(name string, purge bool) error {
	reg, regPath, err := loadRegistry()
	if err != nil {
		return err
	}
	s, ok := reg.Get(name)
	if !ok {
		return unknownServerError(reg, name)
	}
	v := buildView(name, s)
	if v.running() {
		return fmt.Errorf("%q is running — stop it first with `svrctl stop %s`", name, name)
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
	return nil
}

// confirmPurge requires the user to type the server's name, not just "y".
// Deleting a world is unrecoverable and a reflexive yes is too easy.
func confirmPurge(in io.Reader, out io.Writer, name, path string) error {
	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.Warning.Render("This permanently deletes "+path))
	fmt.Fprintln(out, ui.Subtle.Render("Worlds, configs, and plugins in that directory all go with it."))
	fmt.Fprint(out, ui.Body.Render("Type the server's name to confirm: "))

	line, _ := bufio.NewReader(in).ReadString('\n')
	if strings.TrimSpace(line) != name {
		return fmt.Errorf("names did not match, so nothing was deleted")
	}
	return nil
}
