// eula records acceptance of Mojang's EULA, which a Minecraft server refuses
// to start without.
//
// Acceptance is collected before any work begins — by flag, by wizard step, or
// by prompt — so nobody downloads a few hundred megabytes only to be stopped
// at the last moment by a question they could have been asked first.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

// EULAURL is Mojang's canonical EULA link, shown wherever we ask about it.
const EULAURL = "https://aka.ms/MinecraftEULA"

// eulaAccepted reports whether a server directory already records acceptance.
func eulaAccepted(serverDir string) bool {
	data, err := os.ReadFile(filepath.Join(serverDir, "eula.txt"))
	return err == nil && strings.Contains(string(data), "eula=true")
}

// writeEULA records acceptance in the server directory.
func writeEULA(serverDir string) error {
	if eulaAccepted(serverDir) {
		return nil
	}
	path := filepath.Join(serverDir, "eula.txt")
	if err := os.WriteFile(path, []byte("eula=true\n"), 0o644); err != nil {
		return fmt.Errorf("recording EULA acceptance: %w", err)
	}
	return nil
}

// promptEULA asks the user to accept the EULA on a plain (non-TUI) terminal.
// Non-interactive callers must pass --accept-eula instead; erroring out beats
// hanging forever on a read from a pipe that will never contain an answer.
func promptEULA(in io.Reader, out io.Writer) error {
	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.Body.Render("Minecraft servers require accepting Mojang's EULA."))
	fmt.Fprintln(out, ui.Subtle.Render(EULAURL))
	fmt.Fprint(out, ui.Body.Render("Accept? ")+ui.Subtle.Render("[y/N] "))

	reader := bufio.NewReader(in)
	line, _ := reader.ReadString('\n')
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("EULA not accepted, so nothing was created")
	}
}
