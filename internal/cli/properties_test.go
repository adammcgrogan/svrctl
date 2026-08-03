package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPropertiesSkipsCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	content := "#Minecraft server properties\nmotd=A Minecraft Server\n\nmax-players=20\nserver-port=25565\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	props, order, err := readProperties(path)
	if err != nil {
		t.Fatalf("readProperties: %v", err)
	}
	want := map[string]string{"motd": "A Minecraft Server", "max-players": "20", "server-port": "25565"}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("props[%q] = %q, want %q", k, props[k], v)
		}
	}
	wantOrder := []string{"motd", "max-players", "server-port"}
	if len(order) != len(wantOrder) {
		t.Fatalf("got order %v, want %v", order, wantOrder)
	}
	for i := range wantOrder {
		if order[i] != wantOrder[i] {
			t.Errorf("position %d: got %q, want %q", i, order[i], wantOrder[i])
		}
	}
}

func TestReadPropertiesMissingFileYieldsEmpty(t *testing.T) {
	props, order, err := readProperties(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("readProperties: %v", err)
	}
	if len(props) != 0 || len(order) != 0 {
		t.Errorf("expected empty result, got %v %v", props, order)
	}
}

func TestSetPropertyReplacesExistingKeyInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	if err := os.WriteFile(path, []byte("motd=Hello\nmax-players=20\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := setProperty(path, "max-players", "10"); err != nil {
		t.Fatalf("setProperty: %v", err)
	}

	props, order, err := readProperties(path)
	if err != nil {
		t.Fatal(err)
	}
	if props["max-players"] != "10" {
		t.Errorf("max-players = %q, want 10", props["max-players"])
	}
	if props["motd"] != "Hello" {
		t.Errorf("motd was disturbed: %q", props["motd"])
	}
	// In place, not appended: the order shouldn't change.
	if order[0] != "motd" || order[1] != "max-players" {
		t.Errorf("got order %v, want [motd max-players]", order)
	}
}

func TestSetPropertyAppendsNewKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	if err := os.WriteFile(path, []byte("motd=Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := setProperty(path, "difficulty", "hard"); err != nil {
		t.Fatalf("setProperty: %v", err)
	}

	props, _, err := readProperties(path)
	if err != nil {
		t.Fatal(err)
	}
	if props["difficulty"] != "hard" || props["motd"] != "Hello" {
		t.Errorf("got %+v", props)
	}
}

func TestSetPropertyCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")

	if err := setProperty(path, "server-port", "25570"); err != nil {
		t.Fatalf("setProperty: %v", err)
	}

	props, _, err := readProperties(path)
	if err != nil {
		t.Fatal(err)
	}
	if props["server-port"] != "25570" {
		t.Errorf("got %+v", props)
	}
}
