// fetch_test verifies that downloads report progress and land intact.
package fetch

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToFileWritesTheBodyAndReportsCompletion(t *testing.T) {
	body := strings.Repeat("x", 4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "server.jar")
	var last Progress
	if err := ToFile(srv.URL, dest, func(p Progress) { last = p }); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("downloaded %d bytes, want %d", len(got), len(body))
	}
	// The bar must reach the end, or a finished download looks stuck at 97%.
	if last.Done != int64(len(body)) || last.Ratio() != 1 {
		t.Errorf("final progress was %+v (ratio %v), want a completed download", last, last.Ratio())
	}
}

func TestToFileReportsHTTPFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "server.jar")
	err := ToFile(srv.URL, dest, nil)
	if err == nil {
		t.Fatal("expected an error for a 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("got %q, want it to name the status", err)
	}
	// A failed download must not leave a truncated file that later looks like
	// a valid cached jar.
	if _, err := os.Stat(dest); err == nil {
		t.Error("a file was created for a failed download")
	}
}

func TestToFileWorksWithoutAReporter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello")
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "f")
	if err := ToFile(srv.URL, dest, nil); err != nil {
		t.Fatal(err)
	}
}

func TestReaderCountsBytesAndFinishesAtEOF(t *testing.T) {
	src := bytes.NewReader(make([]byte, 1000))
	var last Progress
	r := Reader(src, 1000, func(p Progress) { last = p })

	if _, err := io.Copy(io.Discard, r); err != nil {
		t.Fatal(err)
	}
	if last.Done != 1000 || last.Total != 1000 {
		t.Errorf("got %+v, want 1000/1000", last)
	}
}

func TestRatioHandlesUnknownTotal(t *testing.T) {
	// Servers that send no Content-Length leave the total at zero; the caller
	// falls back to an indeterminate indicator rather than dividing by it.
	if got := (Progress{Done: 500, Total: 0}).Ratio(); got != 0 {
		t.Errorf("got %v, want 0 for an unknown total", got)
	}
	if got := (Progress{Done: 600, Total: 500}).Ratio(); got != 1 {
		t.Errorf("got %v, want the ratio clamped to 1", got)
	}
}
