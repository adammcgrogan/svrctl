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

func TestVerifyFileAcceptsMatchingSha512(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Checksum{Algo: "sha512", Hex: "9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043"}
	if err := VerifyFile(path, c); err != nil {
		t.Errorf("expected matching sha512 checksum to pass, got %v", err)
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
