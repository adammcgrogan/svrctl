// plugin implements `svrctl plugin`: searching, installing, updating, and
// removing Paper plugins from Modrinth.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/adammcgrogan/svrctl/internal/fetch"
	"github.com/adammcgrogan/svrctl/internal/modrinth"
	"github.com/adammcgrogan/svrctl/internal/registry"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

// paperLoader is the Modrinth loader tag for Paper (and Spigot/Bukkit)
// servers, the only kind svrctl installs plugins onto.
const paperLoader = "paper"

func newPluginCmd() *cobra.Command {
	plugin := &cobra.Command{
		Use:   "plugin",
		Short: "Search, install, update, and remove Paper plugins",
		Long:  "Manages plugins on a paper server via Modrinth. Vanilla servers don't support plugins.",
	}
	plugin.AddCommand(
		newPluginSearchCmd(),
		newPluginInstallCmd(),
		newPluginListCmd(),
		newPluginUpdateCmd(),
		newPluginRemoveCmd(),
	)
	return plugin
}

func newPluginSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "search <query>",
		Short:   "Search Modrinth for a plugin",
		Example: "  svrctl plugin search luckperms",
		Args:    requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hits, err := modrinth.Search(args[0], 10)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(hits) == 0 {
				fmt.Fprintln(out, ui.Subtle.Render("No plugins found."))
				return nil
			}
			for _, h := range hits {
				fmt.Fprintln(out, ui.Strong.Render(h.Slug)+ui.Subtle.Render(" — "+h.Title))
				fmt.Fprintln(out, "  "+ui.Truncate(h.Description, 100))
			}
			ui.Hintf(out, "svrctl plugin install <server> <slug>")
			return nil
		},
	}
	return cmd
}

func newPluginInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "install <name> <plugin>",
		Short:             "Install a plugin from Modrinth",
		Example:           "  svrctl plugin install survival luckperms",
		Args:              requireArgs(2),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, project := args[0], args[1]
			out := cmd.OutOrStdout()

			s, err := requirePaperServer(name)
			if err != nil {
				return err
			}

			version, err := modrinth.LatestVersion(project, s.Version, paperLoader)
			if err != nil {
				return err
			}
			installed, err := installPluginVersion(s.Path, project, version)
			if err != nil {
				return err
			}

			ui.Okf(out, "Installed %s %s", ui.Strong.Render(installed.Slug), installed.VersionNumber)
			if buildView(name, s).running() {
				ui.Hintf(out, "svrctl restart %s", name)
			}
			return nil
		},
	}
	return cmd
}

func newPluginListCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:               "list <name>",
		Short:             "List a server's installed plugins",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := requirePaperServer(name)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			manifest, err := loadPluginManifest(s.Path)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(out, manifest.Plugins)
			}
			if len(manifest.Plugins) == 0 {
				fmt.Fprintln(out, ui.Subtle.Render("No plugins installed."))
				ui.Hintf(out, "svrctl plugin search <query>")
				return nil
			}
			for _, p := range manifest.Plugins {
				fmt.Fprintln(out, ui.Body.Render(ui.Pad(p.Slug, 24))+ui.Subtle.Render(p.VersionNumber))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON instead of a table")
	return cmd
}

func newPluginUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "update <name> [plugin]",
		Short:             "Update one or all of a server's plugins to the latest compatible version",
		Example:           "  svrctl plugin update survival\n  svrctl plugin update survival luckperms",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := requirePaperServer(name)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			manifest, err := loadPluginManifest(s.Path)
			if err != nil {
				return err
			}

			var targets []string
			if len(args) == 2 {
				if _, ok := manifest.Plugins[args[1]]; !ok {
					return fmt.Errorf("%q is not installed on %q (run `svrctl plugin list %s`)", args[1], name, name)
				}
				targets = []string{args[1]}
			} else {
				for slug := range manifest.Plugins {
					targets = append(targets, slug)
				}
			}
			if len(targets) == 0 {
				fmt.Fprintln(out, ui.Subtle.Render("No plugins installed."))
				return nil
			}

			changed := false
			for _, slug := range targets {
				current := manifest.Plugins[slug]
				version, err := modrinth.LatestVersion(current.ProjectID, s.Version, paperLoader)
				if err != nil {
					ui.Warnf(out, "%s: %v", slug, err)
					continue
				}
				if version.ID == current.VersionID {
					ui.Stepf(out, "%s is already up to date (%s)", slug, current.VersionNumber)
					continue
				}
				if err := os.Remove(filepath.Join(s.Path, "plugins", current.Filename)); err != nil && !os.IsNotExist(err) {
					ui.Warnf(out, "%s: removing old jar: %v", slug, err)
					continue
				}
				installed, err := installPluginVersion(s.Path, slug, version)
				if err != nil {
					ui.Warnf(out, "%s: %v", slug, err)
					continue
				}
				changed = true
				ui.Okf(out, "Updated %s: %s → %s", ui.Strong.Render(slug), current.VersionNumber, installed.VersionNumber)
			}
			if changed && buildView(name, s).running() {
				ui.Hintf(out, "svrctl restart %s", name)
			}
			return nil
		},
	}
	return cmd
}

func newPluginRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remove <name> <plugin>",
		Short:             "Remove an installed plugin",
		Args:              requireArgs(2),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, slug := args[0], args[1]
			s, err := requirePaperServer(name)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			manifest, err := loadPluginManifest(s.Path)
			if err != nil {
				return err
			}
			p, ok := manifest.Plugins[slug]
			if !ok {
				return fmt.Errorf("%q is not installed on %q (run `svrctl plugin list %s`)", slug, name, name)
			}

			if err := os.Remove(filepath.Join(s.Path, "plugins", p.Filename)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing plugin jar: %w", err)
			}
			delete(manifest.Plugins, slug)
			if err := manifest.save(s.Path); err != nil {
				return err
			}

			ui.Okf(out, "Removed %s", ui.Strong.Render(slug))
			if buildView(name, s).running() {
				ui.Hintf(out, "svrctl restart %s", name)
			}
			return nil
		},
	}
	return cmd
}

// requirePaperServer resolves name and rejects it unless it's a paper
// server — plugin management doesn't apply to vanilla.
func requirePaperServer(name string) (registry.Server, error) {
	s, err := resolveServer(name)
	if err != nil {
		return registry.Server{}, err
	}
	if s.Type != "paper" {
		return registry.Server{}, fmt.Errorf("%q is a %s server — plugin management only works on paper servers", name, s.Type)
	}
	return s, nil
}

// installedPlugin is one entry of a server's plugin manifest.
type installedPlugin struct {
	ProjectID     string `yaml:"project_id" json:"project_id"`
	Slug          string `yaml:"slug" json:"slug"`
	VersionID     string `yaml:"version_id" json:"version_id"`
	VersionNumber string `yaml:"version_number" json:"version_number"`
	Filename      string `yaml:"filename" json:"filename"`
}

// pluginManifest tracks what svrctl has installed into a server's plugins/
// directory, keyed by Modrinth slug. It lives in the server's own directory
// so it travels with a copied or backed-up server.
type pluginManifest struct {
	Plugins map[string]installedPlugin `yaml:"plugins"`
}

func pluginManifestPath(serverDir string) string {
	return filepath.Join(serverDir, ".svrctl", "plugins.yaml")
}

func loadPluginManifest(serverDir string) (*pluginManifest, error) {
	data, err := os.ReadFile(pluginManifestPath(serverDir))
	if os.IsNotExist(err) {
		return &pluginManifest{Plugins: map[string]installedPlugin{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading plugin manifest: %w", err)
	}
	var m pluginManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing plugin manifest: %w", err)
	}
	if m.Plugins == nil {
		m.Plugins = map[string]installedPlugin{}
	}
	return &m, nil
}

func (m *pluginManifest) save(serverDir string) error {
	path := pluginManifestPath(serverDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("writing plugin manifest: %w", err)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("encoding plugin manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing plugin manifest: %w", err)
	}
	return nil
}

// installPluginVersion downloads version's primary file into serverDir's
// plugins/ directory, verifies its checksum, and records it in the manifest
// under slug — the identifier the user asked for (typically Modrinth's slug),
// so `svrctl plugin remove/update` can look it back up by the same name.
func installPluginVersion(serverDir, slug string, version modrinth.Version) (installedPlugin, error) {
	file, ok := version.PrimaryFile()
	if !ok {
		return installedPlugin{}, fmt.Errorf("%s %s has no downloadable file", version.ProjectID, version.VersionNumber)
	}

	pluginsDir := filepath.Join(serverDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return installedPlugin{}, fmt.Errorf("creating plugins directory: %w", err)
	}
	destPath := filepath.Join(pluginsDir, file.Filename)

	if err := fetch.ToFile(file.URL, destPath, nil); err != nil {
		return installedPlugin{}, err
	}
	if err := fetch.VerifyFile(destPath, file.Checksum()); err != nil {
		os.Remove(destPath)
		return installedPlugin{}, fmt.Errorf("verifying %s: %w", file.Filename, err)
	}

	manifest, err := loadPluginManifest(serverDir)
	if err != nil {
		return installedPlugin{}, err
	}
	installed := installedPlugin{
		ProjectID:     version.ProjectID,
		Slug:          slug,
		VersionID:     version.ID,
		VersionNumber: version.VersionNumber,
		Filename:      file.Filename,
	}
	manifest.Plugins[slug] = installed
	if err := manifest.save(serverDir); err != nil {
		return installedPlugin{}, err
	}
	return installed, nil
}
