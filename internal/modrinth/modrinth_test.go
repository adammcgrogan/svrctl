package modrinth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withFakeModrinthServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"hits": []map[string]any{
				{"project_id": "abc123", "slug": "luckperms", "title": "LuckPerms", "description": "Permissions", "downloads": 1000},
			},
		})
	})
	mux.HandleFunc("/project/luckperms/version", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":             "v2",
				"project_id":     "abc123",
				"version_number": "5.5.0",
				"files": []map[string]any{
					{"url": "https://example.com/lp.jar", "filename": "lp.jar", "primary": true,
						"hashes": map[string]string{"sha1": "aaa", "sha512": "bbb"}},
				},
			},
			{
				"id":             "v1",
				"project_id":     "abc123",
				"version_number": "5.4.0",
				"files":          []map[string]any{},
			},
		})
	})
	mux.HandleFunc("/project/empty/version", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{})
	})

	orig := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = orig })
	return srv
}

func TestSearchReturnsHits(t *testing.T) {
	withFakeModrinthServer(t)

	hits, err := Search("luckperms", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].Slug != "luckperms" {
		t.Errorf("got %+v", hits)
	}
}

func TestLatestVersionReturnsNewestFirst(t *testing.T) {
	withFakeModrinthServer(t)

	v, err := LatestVersion("luckperms", "1.21.1", "paper")
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if v.VersionNumber != "5.5.0" {
		t.Errorf("got version %q, want 5.5.0 (the newest)", v.VersionNumber)
	}
}

func TestLatestVersionErrorsWhenNoneExist(t *testing.T) {
	withFakeModrinthServer(t)

	if _, err := LatestVersion("empty", "1.21.1", "paper"); err == nil {
		t.Error("expected an error when no compatible version exists")
	}
}

func TestPrimaryFilePrefersMarkedPrimary(t *testing.T) {
	v := Version{Files: []VersionFile{
		{Filename: "a.jar", Primary: false},
		{Filename: "b.jar", Primary: true},
	}}
	f, ok := v.PrimaryFile()
	if !ok || f.Filename != "b.jar" {
		t.Errorf("got %+v, want b.jar", f)
	}
}

func TestPrimaryFileFallsBackToFirstFile(t *testing.T) {
	v := Version{Files: []VersionFile{{Filename: "only.jar"}}}
	f, ok := v.PrimaryFile()
	if !ok || f.Filename != "only.jar" {
		t.Errorf("got %+v, want only.jar", f)
	}
}

func TestPrimaryFileFalseWhenNoFiles(t *testing.T) {
	if _, ok := (Version{}).PrimaryFile(); ok {
		t.Error("expected no primary file when Files is empty")
	}
}

func TestChecksumPrefersSha512(t *testing.T) {
	f := VersionFile{Hashes: map[string]string{"sha1": "aaa", "sha512": "bbb"}}
	c := f.Checksum()
	if c.Algo != "sha512" || c.Hex != "bbb" {
		t.Errorf("got %+v, want sha512:bbb", c)
	}
}

func TestChecksumFallsBackToSha1(t *testing.T) {
	f := VersionFile{Hashes: map[string]string{"sha1": "aaa"}}
	c := f.Checksum()
	if c.Algo != "sha1" || c.Hex != "aaa" {
		t.Errorf("got %+v, want sha1:aaa", c)
	}
}
