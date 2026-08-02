// server_list implements `svrctl server list`: a table of every registered
// server and whether it's currently running.
package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newServerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List registered servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, err := loadRegistry()
			if err != nil {
				return err
			}

			names := make([]string, 0, len(reg.Servers))
			for name := range reg.Servers {
				names = append(names, name)
			}
			sort.Strings(names)

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tVERSION\tSTATUS\tPORT\tPATH")
			for _, name := range names {
				s := reg.Servers[name]
				status := "stopped"
				if _, ok := process.IsRunning(s.Path); ok {
					status = "running"
				}
				port := "-"
				if s.Port != 0 {
					port = fmt.Sprintf("%d", s.Port)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, s.Type, s.Version, status, port, s.Path)
			}
			return w.Flush()
		},
	}
}
