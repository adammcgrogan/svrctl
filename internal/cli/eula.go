// eula prompts for and records acceptance of Mojang's EULA, required before
// a Minecraft server will run.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func acceptEULA(cmd *cobra.Command, serverDir string) error {
	eulaPath := filepath.Join(serverDir, "eula.txt")
	if data, err := os.ReadFile(eulaPath); err == nil && strings.Contains(string(data), "eula=true") {
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nMinecraft requires accepting Mojang's EULA: https://aka.ms/MinecraftEULA")
	fmt.Fprint(cmd.OutOrStdout(), "Do you accept the EULA? [y/N] ")

	reader := bufio.NewReader(cmd.InOrStdin())
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return fmt.Errorf("EULA not accepted; aborting")
	}

	return os.WriteFile(eulaPath, []byte("eula=true\n"), 0o644)
}
