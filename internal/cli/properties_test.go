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

func TestSetPropertyRejectsValuesThatWouldInjectSettings(t *testing.T) {
	bad := map[string]string{
		"newline":         "Welcome\nwhite-list=false",
		"carriage return": "Welcome\rwhite-list=false",
		"control char":    "Welcome\x00",
	}
	for label, value := range bad {
		dir := t.TempDir()
		path := filepath.Join(dir, "server.properties")
		if err := os.WriteFile(path, []byte("motd=Hello\nwhite-list=true\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := setProperty(path, "motd", value); err == nil {
			t.Errorf("%s: value %q was accepted", label, value)
		}

		// The file must be untouched, not partially rewritten.
		props, _, err := readProperties(path)
		if err != nil {
			t.Fatal(err)
		}
		if props["motd"] != "Hello" || props["white-list"] != "true" {
			t.Errorf("%s: file was modified: %+v", label, props)
		}
	}
}

func TestSetPropertyRejectsUnusableKeys(t *testing.T) {
	bad := map[string]string{
		"empty":          "",
		"separator":      "a=b",
		"colon":          "a:b",
		"comment marker": "#motd",
		"bang":           "!motd",
		"space":          "max players",
		"newline":        "motd\nwhite-list",
	}
	for label, key := range bad {
		path := filepath.Join(t.TempDir(), "server.properties")
		if err := setProperty(path, key, "x"); err == nil {
			t.Errorf("%s: key %q was accepted", label, key)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s: key %q created a file", label, key)
		}
	}
}

func TestSetPropertyValueReadsBackUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.properties")
	// Punctuation an MOTD might plausibly carry, including the separator,
	// which is legal in a value even though it is not in a key.
	value := "Welcome! Rules: be nice = be welcome (see #general)"

	if err := setProperty(path, "motd", value); err != nil {
		t.Fatalf("setProperty: %v", err)
	}
	props, _, err := readProperties(path)
	if err != nil {
		t.Fatal(err)
	}
	if props["motd"] != value {
		t.Errorf("motd read back as %q, want %q", props["motd"], value)
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
