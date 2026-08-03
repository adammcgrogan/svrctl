// paper_test verifies Paper's resolution logic against a fake PaperMC Fill v3 API server.
package serverkind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withFakePaperServer(t *testing.T) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/versions/1.21.1/builds", func(w http.ResponseWriter, r *http.Request) {
		// Newest first, matching the real API's ordering; includes a
		// newer non-stable build that should be skipped in favor of stable.
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":      11,
				"channel": "EXPERIMENTAL",
				"downloads": map[string]any{
					"server:default": map[string]string{
						"name": "paper-1.21.1-11.jar",
						"url":  "https://example.com/paper-1.21.1-11.jar",
					},
				},
			},
			{
				"id":      10,
				"channel": "STABLE",
				"downloads": map[string]any{
					"server:default": map[string]any{
						"name":      "paper-1.21.1-10.jar",
						"url":       "https://example.com/paper-1.21.1-10.jar",
						"checksums": map[string]string{"sha256": "cafef00d"},
					},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	orig := paperAPIBase
	paperAPIBase = srv.URL
	t.Cleanup(func() { paperAPIBase = orig })
}

func TestPaperResolveDownloadPrefersStableChannel(t *testing.T) {
	withFakePaperServer(t)

	got, err := Paper{}.ResolveDownload("1.21.1")
	if err != nil {
		t.Fatalf("ResolveDownload: %v", err)
	}
	if want := "https://example.com/paper-1.21.1-10.jar"; got.URL != want {
		t.Errorf("got %q, want %q", got.URL, want)
	}
	if got.Checksum.Algo != "sha256" || got.Checksum.Hex != "cafef00d" {
		t.Errorf("got checksum %+v, want sha256:cafef00d", got.Checksum)
	}
}

func TestPaperResolveDownloadReportsUnexpectedStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/versions/0.0.0-does-not-exist/builds", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	orig := paperAPIBase
	paperAPIBase = srv.URL
	t.Cleanup(func() { paperAPIBase = orig })

	_, err := Paper{}.ResolveDownload("0.0.0-does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
	if want := "unexpected status 404"; !strings.Contains(err.Error(), want) {
		t.Errorf("got error %q, want it to contain %q", err.Error(), want)
	}
}

func TestPaperRequiredJavaMajorDelegatesToVanilla(t *testing.T) {
	withFakeMojangServer(t)

	got, err := Paper{}.RequiredJavaMajor("1.21.1")
	if err != nil {
		t.Fatalf("RequiredJavaMajor: %v", err)
	}
	if got != 21 {
		t.Errorf("got %d, want 21", got)
	}
}

func TestPaperListVersionsFlattensGroupsNewestFirst(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// The API groups versions under minor keys, and JSON objects have no
		// order, so the ordering has to be reconstructed.
		json.NewEncoder(w).Encode(map[string]any{
			"versions": map[string][]string{
				"1.20": {"1.20.6", "1.20.1"},
				"1.21": {"1.21.4", "1.21.4-rc1", "1.21.1"},
				"1.8":  {"1.8.8"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	orig := paperAPIBase
	paperAPIBase = srv.URL
	t.Cleanup(func() { paperAPIBase = orig })

	got, err := Paper{}.ListVersions()
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}

	want := []string{"1.21.4", "1.21.1", "1.20.6", "1.20.1", "1.8.8"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCompareVersionsOrdersNumerically(t *testing.T) {
	cases := []struct {
		a, b string
		want string // "newer", "older", or "same"
	}{
		{"1.21", "1.20", "newer"},
		{"1.9", "1.10", "older"},  // lexical sorting gets this one wrong
		{"26.2", "1.21", "newer"}, // the version numbering scheme changed
		{"1.21", "1.21", "same"},
		{"1.21.1", "1.21", "newer"},
	}
	for _, c := range cases {
		got := compareVersions(c.a, c.b)
		var label string
		switch {
		case got > 0:
			label = "newer"
		case got < 0:
			label = "older"
		default:
			label = "same"
		}
		if label != c.want {
			t.Errorf("compareVersions(%q, %q) says %s, want %s", c.a, c.b, label, c.want)
		}
	}
}
