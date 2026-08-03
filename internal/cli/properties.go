// properties implements `svrctl properties`: reading and writing
// server.properties without disturbing settings the command doesn't touch.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/registry"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

func newPropertiesCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "properties <name> [key] [value]",
		Short: "Get, set, or list a server's server.properties",
		Long: "With just a name, lists every setting. With a key, prints its value.\n" +
			"With a key and a value, sets it — every other line in the file is left alone.",
		Example: "  svrctl properties survival\n" +
			"  svrctl properties survival motd\n" +
			"  svrctl properties survival motd \"Welcome!\"",
		Args:              cobra.RangeArgs(1, 3),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			out := cmd.OutOrStdout()

			switch len(args) {
			case 1:
				props, order, err := serverProperties(name)
				if err != nil {
					return err
				}
				if asJSON {
					return writeJSON(out, props)
				}
				for _, k := range order {
					ui.Field(out, k, props[k])
				}
				return nil

			case 2:
				props, _, err := serverProperties(name)
				if err != nil {
					return err
				}
				v, ok := props[args[1]]
				if !ok {
					return fmt.Errorf("%q is not set in %s's server.properties", args[1], name)
				}
				fmt.Fprintln(out, v)
				return nil

			default:
				key, value := args[1], args[2]
				s, err := setServerProperty(name, key, value)
				if err != nil {
					return err
				}
				ui.Okf(out, "Set %s = %s", ui.Strong.Render(key), value)
				if buildView(name, s).running() {
					ui.Hintf(out, "svrctl restart %s", name)
				}
				return nil
			}
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print every setting as JSON (only when no key is given)")
	return cmd
}

// serverProperties reads name's server.properties, shared by the `properties`
// command and the dashboard's properties sub-mode.
func serverProperties(name string) (map[string]string, []string, error) {
	s, err := resolveServer(name)
	if err != nil {
		return nil, nil, err
	}
	return readProperties(filepath.Join(s.Path, "server.properties"))
}

// setServerProperty writes key=value into name's server.properties, shared
// by the `properties` command and the dashboard's properties sub-mode.
func setServerProperty(name, key, value string) (registry.Server, error) {
	s, err := resolveServer(name)
	if err != nil {
		return registry.Server{}, err
	}
	if err := setProperty(filepath.Join(s.Path, "server.properties"), key, value); err != nil {
		return registry.Server{}, err
	}
	return s, nil
}

// readProperties parses server.properties into a key->value map plus the
// keys in the order they appear in the file, skipping comments and blank
// lines. A missing file yields an empty result rather than an error, since a
// server that has never been started may not have written one yet.
func readProperties(path string) (map[string]string, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil, nil
		}
		return nil, nil, fmt.Errorf("reading server.properties: %w", err)
	}

	props := map[string]string{}
	var order []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		if _, seen := props[key]; !seen {
			order = append(order, key)
		}
		props[key] = value
	}
	return props, order, nil
}

// setProperty writes key=value into server.properties, replacing an existing
// line for that key in place or appending a new one, and leaving every other
// line — including comments — untouched.
func setProperty(propsPath, key, value string) error {
	line := key + "=" + value

	existing, err := os.ReadFile(propsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading server.properties: %w", err)
		}
		if err := os.WriteFile(propsPath, []byte(line+"\n"), 0o644); err != nil {
			return fmt.Errorf("writing server.properties: %w", err)
		}
		return nil
	}

	lines := strings.Split(string(existing), "\n")
	replaced := false
	for i, l := range lines {
		k, _, ok := strings.Cut(strings.TrimSpace(l), "=")
		if ok && k == key {
			lines[i] = line
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, line)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.WriteFile(propsPath, []byte(out), 0o644); err != nil {
		return fmt.Errorf("writing server.properties: %w", err)
	}
	return nil
}
