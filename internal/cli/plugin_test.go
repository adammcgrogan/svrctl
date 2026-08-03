package cli

import (
	"testing"

	"github.com/adammcgrogan/svrctl/internal/paths"
	"github.com/adammcgrogan/svrctl/internal/registry"
)

// setUpRegistryForPluginTest registers a single server under HOME (which the
// caller must have already redirected via t.Setenv) so requirePaperServer
// can resolve it.
func setUpRegistryForPluginTest(t *testing.T, name, kind string) string {
	t.Helper()
	regPath, err := paths.RegistryFile()
	if err != nil {
		t.Fatal(err)
	}
	reg := &registry.Registry{Servers: map[string]registry.Server{}}
	reg.Put(name, registry.Server{Type: kind, Version: "1.21.1", Path: t.TempDir()})
	if err := reg.Save(regPath); err != nil {
		t.Fatal(err)
	}
	return regPath
}

func TestLoadPluginManifestMissingFileYieldsEmpty(t *testing.T) {
	m, err := loadPluginManifest(t.TempDir())
	if err != nil {
		t.Fatalf("loadPluginManifest: %v", err)
	}
	if len(m.Plugins) != 0 {
		t.Errorf("expected empty manifest, got %+v", m.Plugins)
	}
}

func TestPluginManifestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	m, err := loadPluginManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Plugins["luckperms"] = installedPlugin{
		ProjectID:     "abc123",
		Slug:          "luckperms",
		VersionID:     "v2",
		VersionNumber: "5.5.0",
		Filename:      "lp.jar",
	}
	if err := m.save(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	reloaded, err := loadPluginManifest(dir)
	if err != nil {
		t.Fatalf("loadPluginManifest after save: %v", err)
	}
	got, ok := reloaded.Plugins["luckperms"]
	if !ok {
		t.Fatal("expected luckperms to survive the round trip")
	}
	if got.VersionNumber != "5.5.0" || got.Filename != "lp.jar" {
		t.Errorf("got %+v", got)
	}
}

func TestRequirePaperServerRejectsVanilla(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	setUpRegistryForPluginTest(t, "vanilla-box", "vanilla")

	if _, err := requirePaperServer("vanilla-box"); err == nil {
		t.Error("expected an error for a vanilla server")
	}
}

func TestRequirePaperServerAcceptsPaper(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	setUpRegistryForPluginTest(t, "paper-box", "paper")

	if _, err := requirePaperServer("paper-box"); err != nil {
		t.Errorf("expected a paper server to be accepted, got %v", err)
	}
}
