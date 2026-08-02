// registry_test verifies servers.yaml round-trips through Save/Load.
package registry

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.yaml")

	reg := &Registry{Servers: map[string]Server{}}
	reg.Put("survival", Server{Type: "paper", Version: "1.21.1", Path: "/srv/survival", Port: 25565, Memory: "4G"})

	if err := reg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, ok := loaded.Get("survival")
	if !ok {
		t.Fatalf("expected server %q to be present after reload", "survival")
	}
	want := Server{Type: "paper", Version: "1.21.1", Path: "/srv/survival", Port: 25565, Memory: "4G"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadMissingFileYieldsEmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	reg, err := Load(filepath.Join(dir, "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reg.Servers) != 0 {
		t.Errorf("expected empty registry, got %+v", reg.Servers)
	}
}

func TestRemove(t *testing.T) {
	reg := &Registry{Servers: map[string]Server{}}
	reg.Put("survival", Server{Type: "vanilla", Version: "1.21.1"})
	reg.Remove("survival")
	if _, ok := reg.Get("survival"); ok {
		t.Errorf("expected server to be removed")
	}
}
