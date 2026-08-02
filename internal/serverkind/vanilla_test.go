// vanilla_test verifies Vanilla's resolution logic against a fake Mojang
// version-manifest server.
package serverkind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

// newFakeMojangManifest serves a manifest containing a mix of releases and
// snapshots, for exercising ListVersions.
func withMixedMojangManifest(t *testing.T) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"latest": map[string]string{"release": "1.21.4"},
			"versions": []map[string]string{
				{"id": "1.21.5-pre1", "type": "snapshot", "url": "http://x/1"},
				{"id": "1.21.4", "type": "release", "url": "http://x/2"},
				{"id": "24w01a", "type": "snapshot", "url": "http://x/3"},
				{"id": "1.21.1", "type": "release", "url": "http://x/4"},
				{"id": "b1.7.3", "type": "old_beta", "url": "http://x/5"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	orig := mojangManifestURL
	mojangManifestURL = srv.URL + "/manifest.json"
	t.Cleanup(func() { mojangManifestURL = orig })
}

func TestVanillaListVersionsReturnsReleasesNewestFirst(t *testing.T) {
	withMixedMojangManifest(t)

	got, err := Vanilla{}.ListVersions()
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	// Snapshots and betas are noise for someone standing up a server; the
	// manifest's own order is newest-first and is preserved.
	want := []string{"1.21.4", "1.21.1"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManifestIsFetchedOncePerURL(t *testing.T) {
	var hits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		json.NewEncoder(w).Encode(map[string]any{
			"versions": []map[string]string{
				{"id": "1.21.1", "type": "release", "url": "http://example.invalid/v"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	orig := mojangManifestURL
	mojangManifestURL = srv.URL + "/manifest.json"
	t.Cleanup(func() { mojangManifestURL = orig })

	// Creating a server asks for the version list, the jar URL, and the Java
	// requirement. That used to mean re-downloading the manifest each time.
	for i := 0; i < 3; i++ {
		if _, err := (Vanilla{}).ListVersions(); err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("manifest fetched %d times, want 1", got)
	}
}
