// create implements `svrctl create`: the wizard on a terminal, the flag-driven
// form everywhere else. Both run the same provisioning pipeline.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/fetch"
	"github.com/adammcgrogan/svrctl/internal/serverkind"
	"github.com/adammcgrogan/svrctl/internal/tui"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

func newCreateCmd() *cobra.Command {
	var spec ProvisionSpec
	var acceptEULA bool

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Set up a new server",
		Long: "Creates a server: downloads the right jar, installs the Java version it\n" +
			"needs, and registers it under a name you can use with the other commands.\n\n" +
			"Run without arguments on a terminal to be walked through it. Supply the\n" +
			"flags instead to create one non-interactively, which is what scripts want.",
		Example: "  svrctl create\n" +
			"  svrctl create survival --type paper --version 1.21.1 --accept-eula\n" +
			"  svrctl create creative --type vanilla --version 1.21.1 --memory 4G --accept-eula",
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				spec.Name = args[0]
			}

			// The wizard is for filling in what is missing. If the caller
			// already supplied everything, honour it and skip the questions
			// even on a terminal — otherwise a fully-specified command would
			// still stop and ask.
			complete := spec.Name != "" && spec.Kind != "" && spec.Version != ""
			if !complete && ui.Interactive() {
				// Carry everything already supplied into the wizard so it only
				// asks for what is genuinely still missing.
				return runCreateWizard(cmd.OutOrStdout(), spec)
			}

			if err := requireCreateFlags(spec); err != nil {
				return err
			}
			if !acceptEULA {
				if !ui.Interactive() {
					return fmt.Errorf(
						"Mojang's EULA must be accepted before a server can run.\n"+
							"See %s, then pass --accept-eula", EULAURL)
				}
				if err := promptEULA(cmd.InOrStdin(), cmd.OutOrStdout()); err != nil {
					return err
				}
			}
			return createPlain(cmd.OutOrStdout(), spec)
		},
	}

	cmd.Flags().StringVar(&spec.Kind, "type", "", "server type: "+joinOr(serverkind.KindNames()))
	cmd.Flags().StringVar(&spec.Version, "version", "", "Minecraft version, e.g. 1.21.1 (see `svrctl versions`)")
	cmd.Flags().StringVar(&spec.Path, "path", "", "server directory (default ~/mcservers/<name>)")
	cmd.Flags().IntVar(&spec.Port, "port", 0, "port to listen on (default 25565)")
	cmd.Flags().StringVar(&spec.Memory, "memory", "", "JVM heap size, e.g. 4G")
	cmd.Flags().StringVar(&spec.Group, "group", "", "tag this server so `svrctl start/stop/restart --group` can act on it with others")
	cmd.Flags().BoolVar(&acceptEULA, "accept-eula", false, "accept Mojang's EULA ("+EULAURL+")")

	cmd.RegisterFlagCompletionFunc("type", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return serverkind.KindNames(), cobra.ShellCompDirectiveNoFileComp
	})
	cmd.RegisterFlagCompletionFunc("version", completeVersions)
	return cmd
}

// requireCreateFlags reports what is missing, all of it at once, rather than
// one flag per failed attempt.
func requireCreateFlags(spec ProvisionSpec) error {
	var missing []string
	if spec.Name == "" {
		missing = append(missing, "a name")
	}
	if spec.Kind == "" {
		missing = append(missing, "--type")
	}
	if spec.Version == "" {
		missing = append(missing, "--version")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing %s\n\n  svrctl create <name> --type <%s> --version <mc-version> --accept-eula\n\n"+
		"Run `svrctl versions` to see which versions exist, or just `svrctl create`\n"+
		"on a terminal to be walked through it.",
		joinAnd(missing), strings.Join(serverkind.KindNames(), "|"))
}

// createPlain provisions with line-by-line output, for pipes and scripts.
func createPlain(out io.Writer, spec ProvisionSpec) error {
	hooks := ProvisionHooks{
		Phase: func(label string) { ui.Stepf(out, "%s", label) },
	}
	// Byte-level progress is only rendered on a terminal; in a log it would be
	// thousands of lines of noise.
	if ui.IsTerminal(os.Stdout) {
		hooks.Progress = plainProgress(out)
	}

	s, err := Provision(spec, hooks)
	if err != nil {
		return err
	}
	printCreated(out, spec.Name, s.Path)
	return nil
}

// plainProgress renders an in-place percentage on a terminal that is not
// running the full TUI (say, `svrctl create ... --plain`).
func plainProgress(out io.Writer) func(fetch.Progress) {
	return func(p fetch.Progress) {
		if p.Total <= 0 {
			fmt.Fprintf(out, "\r  %s downloaded", ui.Bytes(p.Done))
			return
		}
		fmt.Fprintf(out, "\r  %3.0f%%  %s / %s   ", p.Ratio()*100, ui.Bytes(p.Done), ui.Bytes(p.Total))
		if p.Done >= p.Total {
			fmt.Fprintln(out)
		}
	}
}

// runCreateWizard drives the interactive wizard, starting from whatever the
// caller already supplied, and reports the result.
func runCreateWizard(out io.Writer, prefill ProvisionSpec) error {
	deps := tui.CreateDeps{
		Kinds:        kindItems(),
		ListVersions: listVersions,
		ValidateName: CheckNameAvailable,
		DefaultPath:  DefaultServerPath,
		EULAURL:      EULAURL,
		Provision: func(v tui.CreateValues, phase func(string), progress func(done, total int64)) error {
			_, err := Provision(ProvisionSpec{
				Name:    v.Name,
				Kind:    v.Kind,
				Version: v.Version,
				Path:    v.Path,
				Memory:  v.Memory,
				Port:    v.Port,
			}, ProvisionHooks{
				Phase:    phase,
				Progress: func(p fetch.Progress) { progress(p.Done, p.Total) },
			})
			return err
		},
	}

	values, created, err := tui.RunCreate(deps, tui.CreateValues{
		Name:    prefill.Name,
		Kind:    prefill.Kind,
		Version: prefill.Version,
		Memory:  prefill.Memory,
		Port:    prefill.Port,
		Path:    prefill.Path,
	})
	if err != nil {
		return err
	}
	if !created {
		fmt.Fprintln(out, ui.Subtle.Render("Cancelled — nothing was created."))
		return nil
	}

	path := values.Path
	if path == "" {
		path = DefaultServerPath(values.Name)
	}
	printCreated(out, values.Name, path)
	return nil
}

// printCreated closes the loop: it says what exists now and what to type next,
// because "Created server" on its own leaves the user to go hunting.
func printCreated(out io.Writer, name, path string) {
	ui.Okf(out, "Created %s", ui.Strong.Render(name))
	fmt.Fprintln(out, ui.Subtle.Render("  "+path))
	ui.Hintf(out, "svrctl start %s", name)
}

func kindItems() []tui.Item {
	kinds := serverkind.Kinds()
	items := make([]tui.Item, 0, len(kinds))
	for _, k := range kinds {
		items = append(items, tui.Item{Title: k.Name, Desc: k.Summary})
	}
	return items
}

func listVersions(kind string) ([]string, error) {
	resolver, err := serverkind.For(kind)
	if err != nil {
		return nil, err
	}
	return resolver.ListVersions()
}
