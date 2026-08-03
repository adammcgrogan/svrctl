// list implements `svrctl list` and `svrctl status`: the read-only views of
// what exists and what is up.
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

func newListCmd() *cobra.Command {
	var asJSON bool
	var group string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your servers",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			views, err := viewAll()
			if err != nil {
				return err
			}
			if group != "" {
				views = filterByGroup(views, group)
			}
			out := cmd.OutOrStdout()

			if asJSON {
				return writeJSON(out, serversToJSON(views))
			}
			if len(views) == 0 {
				fmt.Fprintln(out, ui.Subtle.Render("No servers yet."))
				ui.Hintf(out, "svrctl create")
				return nil
			}
			printServerTable(out, views)
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON instead of a table")
	cmd.Flags().StringVar(&group, "group", "", "only list servers in this group")
	return cmd
}

// filterByGroup keeps only the views tagged with group.
func filterByGroup(views []serverView, group string) []serverView {
	out := views[:0]
	for _, v := range views {
		if v.Server.Group == group {
			out = append(out, v)
		}
	}
	return out
}

func newStatusCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:               "status <name>",
		Short:             "Show everything about one server",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := viewServer(args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			if asJSON {
				return writeJSON(out, serverToJSON(v))
			}

			fmt.Fprintln(out)
			fmt.Fprintln(out, " "+ui.Title.Render(v.Name)+"  "+ui.StatusGlyph(v.running()))
			fmt.Fprintln(out)
			ui.Field(out, "Type", v.Server.Type)
			ui.Field(out, "Version", v.Server.Version)
			ui.Field(out, "Port", fmt.Sprintf("%d", v.port()))
			if v.Server.Memory != "" {
				ui.Field(out, "Memory", v.Server.Memory)
			}
			if v.Server.Group != "" {
				ui.Field(out, "Group", v.Server.Group)
			}
			ui.Field(out, "Location", v.Server.Path)
			if v.running() {
				ui.Field(out, "PID", fmt.Sprintf("%d", v.State.PID))
				ui.Field(out, "Uptime", ui.Duration(v.Uptime))
			}
			fmt.Fprintln(out)

			// End on the action the user most likely came here to take.
			if v.running() {
				ui.Hintf(out, "svrctl console %s", v.Name)
			} else {
				ui.Hintf(out, "svrctl start %s", v.Name)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}

// printServerTable renders the listing. Running servers are pulled out of the
// list visually so a glance answers "what's up right now?".
func printServerTable(out io.Writer, views []serverView) {
	nameW, typeW, verW, groupW := 4, 4, 7, 5 // the header labels
	showGroup := false
	for _, v := range views {
		nameW = max(nameW, ui.TextWidth(v.Name))
		typeW = max(typeW, ui.TextWidth(v.Server.Type))
		verW = max(verW, ui.TextWidth(v.Server.Version))
		if v.Server.Group != "" {
			showGroup = true
			groupW = max(groupW, ui.TextWidth(v.Server.Group))
		}
	}

	header := ui.Pad("NAME", nameW+2) + ui.Pad("TYPE", typeW+2) + ui.Pad("VERSION", verW+2)
	if showGroup {
		header += ui.Pad("GROUP", groupW+2)
	}
	header += ui.Pad("STATUS", 12) + ui.Pad("PORT", 8) + "UPTIME"
	fmt.Fprintln(out, ui.Header.Render(header))

	for _, v := range views {
		uptime := ui.Subtle.Render("—")
		if v.running() {
			uptime = ui.Duration(v.Uptime)
		}
		line := ui.Body.Render(ui.Pad(v.Name, nameW+2)) +
			ui.Subtle.Render(ui.Pad(v.Server.Type, typeW+2)) +
			ui.Subtle.Render(ui.Pad(v.Server.Version, verW+2))
		if showGroup {
			line += ui.Subtle.Render(ui.Pad(v.Server.Group, groupW+2))
		}
		line += ui.PadStyled(ui.StatusGlyph(v.running()), 12) +
			ui.Subtle.Render(ui.Pad(fmt.Sprintf("%d", v.port()), 8)) +
			uptime
		fmt.Fprintln(out, line)
	}
}

// serverJSON is the stable shape emitted by --json. Scripts should be able to
// depend on it without parsing a table meant for humans.
type serverJSON struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Port      int    `json:"port"`
	Memory    string `json:"memory,omitempty"`
	Group     string `json:"group,omitempty"`
	Running   bool   `json:"running"`
	PID       int    `json:"pid,omitempty"`
	UptimeSec int    `json:"uptime_seconds,omitempty"`
}

func serverToJSON(v serverView) serverJSON {
	out := serverJSON{
		Name:    v.Name,
		Type:    v.Server.Type,
		Version: v.Server.Version,
		Path:    v.Server.Path,
		Port:    v.port(),
		Memory:  v.Server.Memory,
		Group:   v.Server.Group,
		Running: v.running(),
	}
	if v.running() {
		out.PID = v.State.PID
		out.UptimeSec = int(v.Uptime.Seconds())
	}
	return out
}

func serversToJSON(views []serverView) []serverJSON {
	out := make([]serverJSON, 0, len(views))
	for _, v := range views {
		out = append(out, serverToJSON(v))
	}
	return out
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
