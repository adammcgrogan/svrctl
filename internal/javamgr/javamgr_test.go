// javamgr_test verifies the pure logic in this package: locating a java
// binary within an extracted JDK tree.
package javamgr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindJavaBinary(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "jdk-21.0.4+7", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	javaPath := filepath.Join(binDir, javaBinName())
	if err := os.WriteFile(javaPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := findJavaBinary(dir)
	if !ok {
		t.Fatalf("expected to find java binary under %s", dir)
	}
	if got != javaPath {
		t.Errorf("got %q, want %q", got, javaPath)
	}
}

func TestFindJavaBinaryMissing(t *testing.T) {
	dir := t.TempDir()
	if _, ok := findJavaBinary(dir); ok {
		t.Errorf("expected no java binary to be found in empty dir")
	}
}
