// dashboard_test covers the adapter between the dashboard TUI and the command
// layer, where the two sides' conventions for "leave this alone" have to line
// up: the TUI reports a blank field as a zero value, and only this layer knows
// whether that means "clear it" or "don't touch it".
package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/adammcgrogan/svrctl/internal/paths"
	"github.com/adammcgrogan/svrctl/internal/registry"
)

// setUpServerForDashboardTest registers one server under HOME (which the
// caller must have already redirected via t.Setenv) with a port set in both
// the registry and server.properties, and returns its directory.
func setUpServerForDashboardTest(t *testing.T, name string, port int) string {
	t.Helper()
	dir := t.TempDir()

	regPath, err := paths.RegistryFile()
	if err != nil {
		t.Fatal(err)
	}
	reg := &registry.Registry{Servers: map[string]registry.Server{}}
	reg.Put(name, registry.Server{
		Type: "paper", Version: "1.21.1", Path: dir, Port: port, Memory: "2G",
	})
	if err := reg.Save(regPath); err != nil {
		t.Fatal(err)
	}

	props := "motd=Hello\nserver-port=" + strconv.Itoa(port) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "server.properties"), []byte(props), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func registeredServer(t *testing.T, name string) registry.Server {
	t.Helper()
	regPath, err := paths.RegistryFile()
	if err != nil {
		t.Fatal(err)
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := reg.Get(name)
	if !ok {
		t.Fatalf("%q is no longer registered", name)
	}
	return s
}

func TestDashboardEditWithBlankPortLeavesThePortAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := setUpServerForDashboardTest(t, "survival", 25570)

	// A blank port field reaches the adapter as zero.
	if err := dashboardDeps().Edit("survival", "4G", 0); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	if got := registeredServer(t, "survival").Port; got != 25570 {
		t.Errorf("registry port = %d, want it left at 25570", got)
	}
	props, _, err := readProperties(filepath.Join(dir, "server.properties"))
	if err != nil {
		t.Fatal(err)
	}
	if props["server-port"] != "25570" {
		t.Errorf("server-port = %q, want it left at 25570", props["server-port"])
	}
	// The edit still has to have done something.
	if got := registeredServer(t, "survival").Memory; got != "4G" {
		t.Errorf("memory = %q, want 4G", got)
	}
}

func TestDashboardEditWithAPortUpdatesRegistryAndProperties(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := setUpServerForDashboardTest(t, "survival", 25570)

	if err := dashboardDeps().Edit("survival", "4G", 25580); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	if got := registeredServer(t, "survival").Port; got != 25580 {
		t.Errorf("registry port = %d, want 25580", got)
	}
	props, _, err := readProperties(filepath.Join(dir, "server.properties"))
	if err != nil {
		t.Fatal(err)
	}
	if props["server-port"] != "25580" {
		t.Errorf("server-port = %q, want 25580", props["server-port"])
	}
}

func TestDashboardEditWithBlankMemoryStillClearsIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	setUpServerForDashboardTest(t, "survival", 25570)

	if err := dashboardDeps().Edit("survival", "", 0); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	if got := registeredServer(t, "survival").Memory; got != "" {
		t.Errorf("memory = %q, want it cleared", got)
	}
}
