// paper_test verifies Paper's resolution logic against a fake PaperMC Fill v3 API server.
package serverkind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
					"server:default": map[string]string{
						"name": "paper-1.21.1-10.jar",
						"url":  "https://example.com/paper-1.21.1-10.jar",
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

func TestPaperResolveDownloadURLPrefersStableChannel(t *testing.T) {
	withFakePaperServer(t)

	got, err := Paper{}.ResolveDownloadURL("1.21.1")
	if err != nil {
		t.Fatalf("ResolveDownloadURL: %v", err)
	}
	if want := "https://example.com/paper-1.21.1-10.jar"; got != want {
		t.Errorf("got %q, want %q", got, want)
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
