// edit implements `svrctl edit`: changing a registered server's memory or
// port without hand-editing servers.yaml.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/paths"
	"github.com/adammcgrogan/svrctl/internal/registry"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

func newEditCmd() *cobra.Command {
	var memory string
	var port int
	var group string

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Change a server's memory, port, or group",
		Long: "Updates the memory allocation, port, and/or group svrctl runs a server with.\n" +
			"A running server needs a restart before memory/port changes take effect.",
		Example:           "  svrctl edit survival --memory 8G\n  svrctl edit survival --port 25570\n  svrctl edit survival --group network",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			out := cmd.OutOrStdout()

			var memoryChange, groupChange *string
			var portChange *int
			if cmd.Flags().Changed("memory") {
				memoryChange = &memory
			}
			if cmd.Flags().Changed("port") {
				portChange = &port
			}
			if cmd.Flags().Changed("group") {
				groupChange = &group
			}
			if memoryChange == nil && portChange == nil && groupChange == nil {
				return fmt.Errorf("nothing to change — pass --memory, --port, and/or --group")
			}

			s, err := editServer(name, memoryChange, groupChange, portChange)
			if err != nil {
				return err
			}

			ui.Okf(out, "Updated %s", ui.Strong.Render(name))
			if buildView(name, s).running() {
				ui.Hintf(out, "svrctl restart %s", name)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&memory, "memory", "", "JVM heap size, e.g. 4G")
	cmd.Flags().IntVar(&port, "port", 0, "port to listen on")
	cmd.Flags().StringVar(&group, "group", "", "tag this server so start/stop/restart --group can act on it with others (empty string clears it)")
	return cmd
}

// editServer applies whichever of memory/group/port is non-nil to name's
// registry entry, shared by the `edit` command and the dashboard's edit
// sub-mode so the two don't drift apart.
func editServer(name string, memory, group *string, port *int) (registry.Server, error) {
	regPath, err := paths.RegistryFile()
	if err != nil {
		return registry.Server{}, err
	}

	var s registry.Server
	err = registry.WithLock(regPath, func(reg *registry.Registry) error {
		var ok bool
		s, ok = reg.Get(name)
		if !ok {
			return unknownServerError(reg, name)
		}
		if memory != nil {
			s.Memory = *memory
		}
		if port != nil {
			s.Port = *port
		}
		if group != nil {
			s.Group = *group
		}
		reg.Put(name, s)
		return nil
	})
	if err != nil {
		return registry.Server{}, err
	}

	if port != nil {
		if err := writeServerPort(s.Path, *port); err != nil {
			return registry.Server{}, fmt.Errorf("updating server.properties: %w", err)
		}
	}
	return s, nil
}
