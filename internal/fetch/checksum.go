package fetch

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// Checksum names a hash algorithm and the expected hex digest for a
// download. The zero value means no checksum is available to check against,
// which VerifyFile treats as a no-op rather than a failure — not every
// source publishes one.
type Checksum struct {
	Algo string // "sha1" or "sha256"
	Hex  string
}

// VerifyFile hashes the file at path and compares it against c, failing
// closed on any mismatch or unsupported algorithm. Callers that receive a
// mismatch should remove the downloaded file rather than use it.
func VerifyFile(path string, c Checksum) error {
	if c.Algo == "" {
		return nil
	}

	var h hash.Hash
	switch strings.ToLower(c.Algo) {
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	default:
		return fmt.Errorf("unsupported checksum algorithm %q", c.Algo)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("verifying checksum: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("verifying checksum: %w", err)
	}

	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, c.Hex) {
		return fmt.Errorf("checksum mismatch: got %s %s, want %s", c.Algo, got, c.Hex)
	}
	return nil
}
