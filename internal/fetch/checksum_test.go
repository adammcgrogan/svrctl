package fetch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyFileAcceptsMatchingChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// sha256("hello")
	c := Checksum{Algo: "sha256", Hex: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"}
	if err := VerifyFile(path, c); err != nil {
		t.Errorf("expected matching checksum to pass, got %v", err)
	}
}

func TestVerifyFileRejectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Checksum{Algo: "sha256", Hex: "0000000000000000000000000000000000000000000000000000000000000"}
	if err := VerifyFile(path, c); err == nil {
		t.Error("expected a mismatched checksum to fail")
	}
}

func TestVerifyFileSkipsWhenNoChecksumPublished(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(path, Checksum{}); err != nil {
		t.Errorf("expected empty checksum to be a no-op, got %v", err)
	}
}

func TestVerifyFileRejectsUnsupportedAlgorithm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(path, Checksum{Algo: "md5", Hex: "x"}); err == nil {
		t.Error("expected an unsupported algorithm to fail")
	}
}
