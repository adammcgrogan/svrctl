package javamgr

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// buildFakeTarGz produces a minimal tar.gz containing a single java binary
// under bin/, mimicking a Temurin archive layout.
func buildFakeTarGz(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	content := []byte("#!/bin/sh\necho fake java\n")
	hdr := &tar.Header{
		Name: "jdk-21.0.4+7/bin/java",
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withFakeAdoptiumServer serves both the assets/latest metadata endpoint and
// the archive it points to, so downloadAndExtractJDK can be exercised without
// touching the real Adoptium API.
func withFakeAdoptiumServer(t *testing.T, archive []byte, checksum string) {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/21/hotspot", func(w http.ResponseWriter, r *http.Request) {
		asset := adoptiumAsset{}
		asset.Binary.Package.Link = srv.URL + "/archive.tar.gz"
		asset.Binary.Package.Checksum = checksum
		json.NewEncoder(w).Encode([]adoptiumAsset{asset})
	})
	mux.HandleFunc("/archive.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})

	orig := adoptiumAssetsURL
	adoptiumAssetsURL = srv.URL
	t.Cleanup(func() { adoptiumAssetsURL = orig })
}

func TestDownloadAndExtractJDKVerifiesChecksum(t *testing.T) {
	archive := buildFakeTarGz(t)
	sum := sha256.Sum256(archive)
	withFakeAdoptiumServer(t, archive, hex.EncodeToString(sum[:]))

	destDir := t.TempDir()
	if err := downloadAndExtractJDK(21, destDir, nil); err != nil {
		t.Fatalf("downloadAndExtractJDK: %v", err)
	}

	bin, ok := findJavaBinary(destDir)
	if !ok {
		t.Fatalf("expected a java binary to be extracted under %s", destDir)
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fake java") {
		t.Errorf("unexpected extracted binary contents: %q", data)
	}
}

func TestDownloadAndExtractJDKRejectsChecksumMismatch(t *testing.T) {
	archive := buildFakeTarGz(t)
	withFakeAdoptiumServer(t, archive, strings.Repeat("0", 64))

	destDir := t.TempDir()
	err := downloadAndExtractJDK(21, destDir, nil)
	if err == nil {
		t.Fatal("expected a checksum mismatch to fail the download")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("got %q, want it to mention the checksum failure", err)
	}
	// Nothing should have been extracted from an unverified archive.
	if _, ok := findJavaBinary(destDir); ok {
		t.Error("archive was extracted despite failing checksum verification")
	}
	if entries, _ := os.ReadDir(destDir); len(entries) != 0 {
		t.Errorf("destDir is not empty after a failed download: %v", entries)
	}
}
