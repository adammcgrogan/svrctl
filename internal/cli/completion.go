// completion supplies shell completions and the small bits of phrasing shared
// across command help.
//
// Completing server names matters more here than in most tools: every command
// takes a name the user chose weeks ago, and the alternative to completion is
// running `svrctl list` first every single time.
package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/serverkind"
)

// completeServerNames completes the first positional argument with registered
// server names.
func completeServerNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, name := range serverNames() {
		if strings.HasPrefix(name, toComplete) {
			out = append(out, name)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeVersions completes --version against the versions actually published
// for the chosen --type.
func completeVersions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kind, _ := cmd.Flags().GetString("type")
	if kind == "" {
		kind = "vanilla"
	}
	resolver, err := serverkind.For(kind)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	versions, err := resolver.ListVersions()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, v := range versions {
		if strings.HasPrefix(v, toComplete) {
			out = append(out, v)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// joinOr renders a choice list as "vanilla or paper".
func joinOr(items []string) string { return joinWith(items, "or") }

// joinAnd renders a requirement list as "a name, --type and --version".
func joinAnd(items []string) string { return joinWith(items, "and") }

func joinWith(items []string, conjunction string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " " + conjunction + " " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " " + conjunction + " " + items[len(items)-1]
	}
}
