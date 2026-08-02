// vanilla_test verifies Vanilla's resolution logic against a fake Mojang
// version-manifest server.
package serverkind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newFakeMojangServer serves a version manifest at /manifest.json whose
// single entry ("1.21.1") points back at /version/1.21.1.json on the same
// server, mimicking Mojang's two-step manifest -> per-version metadata flow.
func newFakeMojangServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"versions": []map[string]string{
				{"id": "1.21.1", "url": srv.URL + "/version/1.21.1.json"},
			},
		})
	})
	mux.HandleFunc("/version/1.21.1.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"downloads": map[string]any{
				"server": map[string]string{"url": "https://example.com/server-1.21.1.jar"},
			},
			"javaVersion": map[string]int{"majorVersion": 21},
		})
	})

	return srv
}

func withFakeMojangServer(t *testing.T) {
	t.Helper()
	srv := newFakeMojangServer(t)
	orig := mojangManifestURL
	mojangManifestURL = srv.URL + "/manifest.json"
	t.Cleanup(func() { mojangManifestURL = orig })
}

func TestVanillaResolveDownloadURL(t *testing.T) {
	withFakeMojangServer(t)

	got, err := Vanilla{}.ResolveDownloadURL("1.21.1")
	if err != nil {
		t.Fatalf("ResolveDownloadURL: %v", err)
	}
	if want := "https://example.com/server-1.21.1.jar"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestVanillaRequiredJavaMajor(t *testing.T) {
	withFakeMojangServer(t)

	got, err := Vanilla{}.RequiredJavaMajor("1.21.1")
	if err != nil {
		t.Fatalf("RequiredJavaMajor: %v", err)
	}
	if got != 21 {
		t.Errorf("got %d, want 21", got)
	}
}

func TestVanillaUnknownVersion(t *testing.T) {
	withFakeMojangServer(t)

	if _, err := (Vanilla{}).ResolveDownloadURL("99.99"); err == nil {
		t.Errorf("expected error for unknown version")
	}
}
