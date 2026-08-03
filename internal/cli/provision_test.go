// provision_test covers the pure parts of the creation pipeline: the checks
// that run before any network work, and the config file it writes.
package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adammcgrogan/svrctl/internal/paths"
	"github.com/adammcgrogan/svrctl/internal/registry"
)

func TestValidateNameRejectsUnusableNames(t *testing.T) {
	// Each of these would either break the directory layout or produce a name
	// no one could type back into another command.
	bad := map[string]string{
		"empty":          "",
		"blank":          "   ",
		"path separator": "worlds/survival",
		"parent":         "..",
		"current":        ".",
		"space":          "my server",
		"tab":            "my\tserver",
	}
	for label, name := range bad {
		if err := ValidateName(name); err == nil {
			t.Errorf("%s: %q was accepted", label, name)
		}
	}
}

func TestValidateNameAcceptsOrdinaryNames(t *testing.T) {
	for _, name := range []string{"survival", "creative-2", "smp_1.21", "Test"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("%q rejected: %v", name, err)
		}
	}
}

func TestWriteServerPortCreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeServerPort(dir, 25570); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, filepath.Join(dir, "server.properties"))
	if got != "server-port=25570\n" {
		t.Errorf("got %q", got)
	}
}

func TestWriteServerPortPreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	existing := "motd=A Minecraft Server\nserver-port=25565\nmax-players=20\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeServerPort(dir, 25570); err != nil {
		t.Fatal(err)
	}

	// Truncating the file, as this used to, silently discarded every setting
	// the user had configured.
	got := readFile(t, path)
	for _, want := range []string{"motd=A Minecraft Server", "max-players=20", "server-port=25570"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "server-port=25565") {
		t.Errorf("old port left behind:\n%s", got)
	}
}

func TestWriteServerPortAppendsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	if err := os.WriteFile(path, []byte("motd=Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeServerPort(dir, 25580); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, "motd=Hello") || !strings.Contains(got, "server-port=25580") {
		t.Errorf("got:\n%s", got)
	}
}

func TestDefaultServerPathUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := DefaultServerPath("survival"), filepath.Join(home, "mcservers", "survival"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEULARoundTrip(t *testing.T) {
	dir := t.TempDir()
	if eulaAccepted(dir) {
		t.Fatal("a fresh directory should not count as accepted")
	}
	if err := writeEULA(dir); err != nil {
		t.Fatal(err)
	}
	if !eulaAccepted(dir) {
		t.Error("acceptance was not recorded")
	}
}

func TestPromptEULARequiresAffirmative(t *testing.T) {
	for _, answer := range []string{"y\n", "yes\n", "Y\n"} {
		if err := promptEULA(strings.NewReader(answer), io.Discard); err != nil {
			t.Errorf("%q was not accepted: %v", answer, err)
		}
	}
	for _, answer := range []string{"n\n", "\n", "maybe\n", ""} {
		if err := promptEULA(strings.NewReader(answer), io.Discard); err == nil {
			t.Errorf("%q was treated as acceptance", answer)
		}
	}
}

func TestCheckPathAvailableRejectsAPathAnotherServerOwns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	regPath, err := paths.RegistryFile()
	if err != nil {
		t.Fatal(err)
	}
	reg := &registry.Registry{Servers: map[string]registry.Server{}}
	reg.Put("foo", registry.Server{Type: "vanilla", Version: "1.21.1", Path: "/tmp/shared"})
	if err := reg.Save(regPath); err != nil {
		t.Fatal(err)
	}

	if err := CheckPathAvailable("/tmp/shared"); err == nil {
		t.Fatal("expected a path collision with server \"foo\" to be rejected")
	}
	if err := CheckPathAvailable("/tmp/not-shared"); err != nil {
		t.Errorf("expected an unused path to be accepted, got %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
