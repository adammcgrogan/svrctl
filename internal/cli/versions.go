// versions implements `svrctl versions`: what you can actually pass to
// `create --version`.
//
// Without this, the only way to discover a valid version string was to guess
// one and see whether the download failed.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/serverkind"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

func newVersionsCmd() *cobra.Command {
	var kind string
	var limit int

	cmd := &cobra.Command{
		Use:     "versions",
		Short:   "List the Minecraft versions you can create",
		Example: "  svrctl versions\n  svrctl versions --type paper\n  svrctl versions --type paper --limit 0",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolver, err := serverkind.For(kind)
			if err != nil {
				return err
			}
			versions, err := resolver.ListVersions()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			shown := versions
			if limit > 0 && len(shown) > limit {
				shown = shown[:limit]
			}
			for i, v := range shown {
				if i == 0 {
					fmt.Fprintln(out, ui.Body.Render(v)+"  "+ui.Success.Render("latest"))
					continue
				}
				fmt.Fprintln(out, ui.Body.Render(v))
			}
			if len(shown) < len(versions) {
				fmt.Fprintln(out, ui.Subtle.Render(fmt.Sprintf(
					"… and %d older (--limit 0 for all)", len(versions)-len(shown))))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&kind, "type", "vanilla", "server type: "+strings.Join(serverkind.KindNames(), ", "))
	cmd.Flags().IntVar(&limit, "limit", 20, "how many to show, newest first (0 for all)")
	cmd.RegisterFlagCompletionFunc("type", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return serverkind.KindNames(), cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
