package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adammcgrogan/svrctl/internal/paths"
)

func TestWorldDirectoriesDefaultsToWorld(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "world"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "world_nether"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := worldDirectories(dir)
	if err != nil {
		t.Fatalf("worldDirectories: %v", err)
	}
	want := []string{"world", "world_nether"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWorldDirectoriesHonorsLevelName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "server.properties"), []byte("level-name=myworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "myworld"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A stray "world" dir with the default name should be ignored once
	// level-name points elsewhere.
	if err := os.MkdirAll(filepath.Join(dir, "world"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := worldDirectories(dir)
	if err != nil {
		t.Fatalf("worldDirectories: %v", err)
	}
	if len(got) != 1 || got[0] != "myworld" {
		t.Errorf("got %v, want [myworld]", got)
	}
}

func TestCreateAndExtractWorldArchiveRoundTrips(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "world", "region"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "world", "region", "r.0.0.mca"), []byte("chunk data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "world", "level.dat"), []byte("level data"), 0o644); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(t.TempDir(), "backup.tar.gz")
	if err := createWorldArchive(archivePath, srcDir, []string{"world"}); err != nil {
		t.Fatalf("createWorldArchive: %v", err)
	}

	destDir := t.TempDir()
	if err := extractWorldArchive(archivePath, destDir); err != nil {
		t.Fatalf("extractWorldArchive: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "world", "region", "r.0.0.mca"))
	if err != nil {
		t.Fatalf("reading extracted region file: %v", err)
	}
	if string(got) != "chunk data" {
		t.Errorf("region file contents = %q", got)
	}
	got, err = os.ReadFile(filepath.Join(destDir, "world", "level.dat"))
	if err != nil {
		t.Fatalf("reading extracted level.dat: %v", err)
	}
	if string(got) != "level data" {
		t.Errorf("level.dat contents = %q", got)
	}
}

func TestResolveBackupSupportsLatestAndByID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir, err := paths.BackupsDir("survival")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"2026-01-01T00-00-00", "2026-01-02T00-00-00"} {
		if err := os.WriteFile(filepath.Join(dir, id+".tar.gz"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	path, id, err := resolveBackup("survival", "latest")
	if err != nil {
		t.Fatalf("resolveBackup(latest): %v", err)
	}
	if id != "2026-01-02T00-00-00" {
		t.Errorf("latest resolved to %q, want the newer backup", id)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("resolved path does not exist: %v", err)
	}

	if _, _, err := resolveBackup("survival", "not-a-real-backup"); err == nil {
		t.Error("expected an error for an unknown backup ID")
	}
}

func TestResolveBackupErrorsWhenNoneExist(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := resolveBackup("empty-server", "latest"); err == nil {
		t.Error("expected an error when no backups exist")
	}
}

