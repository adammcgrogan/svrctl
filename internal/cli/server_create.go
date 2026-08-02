// server_create implements `svrctl server create`: provisions a new server's
// directory, downloads its jar, ensures its JDK is cached, and registers it.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/registry"
	"github.com/adammcgrogan/svrctl/internal/serverkind"
)

func newServerCreateCmd() *cobra.Command {
	var kind, version, path string
	var port int
	var memory string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Register and provision a new server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if kind == "" || version == "" {
				return fmt.Errorf("--type and --version are required")
			}
			resolver, err := serverkind.For(kind)
			if err != nil {
				return err
			}

			reg, regPath, err := loadRegistry()
			if err != nil {
				return err
			}
			if _, exists := reg.Get(name); exists {
				return fmt.Errorf("a server named %q already exists", name)
			}

			if path == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				path = filepath.Join(home, "mcservers", name)
			}
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(absPath, 0o755); err != nil {
				return fmt.Errorf("creating server directory: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Resolving %s %s download...\n", kind, version)
			downloadURL, err := resolver.ResolveDownloadURL(version)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Downloading server.jar...")
			if err := downloadFile(downloadURL, filepath.Join(absPath, "server.jar")); err != nil {
				return err
			}

			s := registry.Server{Type: kind, Version: version, Path: absPath, Port: port, Memory: memory}

			fmt.Fprintln(cmd.OutOrStdout(), "Ensuring required Java version is installed (this may take a while on first use)...")
			if _, err := resolveJavaPath(s); err != nil {
				return err
			}

			if err := acceptEULA(cmd, absPath); err != nil {
				return err
			}

			if port != 0 {
				propsPath := filepath.Join(absPath, "server.properties")
				line := fmt.Sprintf("server-port=%d\n", port)
				if err := os.WriteFile(propsPath, []byte(line), 0o644); err != nil {
					return fmt.Errorf("writing server.properties: %w", err)
				}
			}

			reg.Put(name, s)
			if err := reg.Save(regPath); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created server %q (%s %s) at %s\n", name, kind, version, absPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&kind, "type", "", "server type: vanilla or paper")
	cmd.Flags().StringVar(&version, "version", "", "Minecraft version, e.g. 1.21.1")
	cmd.Flags().StringVar(&path, "path", "", "server directory (default ~/mcservers/<name>)")
	cmd.Flags().IntVar(&port, "port", 0, "server port (default 25565 if unset)")
	cmd.Flags().StringVar(&memory, "memory", "", "JVM heap size, e.g. 4G")
	return cmd
}
