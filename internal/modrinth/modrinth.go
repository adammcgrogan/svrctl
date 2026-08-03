// Package modrinth is a thin client for the parts of the Modrinth API
// svrctl's plugin management needs: searching plugins and finding a version
// compatible with a specific Minecraft version and server loader.
package modrinth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/adammcgrogan/svrctl/internal/fetch"
)

// baseURL is a var (not const) so tests can point it at a fake server.
var baseURL = "https://api.modrinth.com/v2"

// SearchHit is one entry in a search result.
type SearchHit struct {
	ProjectID   string `json:"project_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Downloads   int    `json:"downloads"`
}

// Search looks up plugins matching query, most relevant first.
func Search(query string, limit int) ([]SearchHit, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("facets", `[["project_type:plugin"]]`)
	q.Set("limit", fmt.Sprintf("%d", limit))

	var resp struct {
		Hits []SearchHit `json:"hits"`
	}
	if err := getJSON(baseURL+"/search?"+q.Encode(), &resp); err != nil {
		return nil, fmt.Errorf("searching modrinth: %w", err)
	}
	return resp.Hits, nil
}

// VersionFile is one downloadable artifact of a Version.
type VersionFile struct {
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Primary  bool              `json:"primary"`
	Hashes   map[string]string `json:"hashes"`
}

// Version is one published release of a plugin.
type Version struct {
	ID            string        `json:"id"`
	ProjectID     string        `json:"project_id"`
	Name          string        `json:"name"`
	VersionNumber string        `json:"version_number"`
	Files         []VersionFile `json:"files"`
}

// PrimaryFile returns the file to download for this version: the one marked
// primary, or the first one if none is (Modrinth guarantees at least one).
func (v Version) PrimaryFile() (VersionFile, bool) {
	for _, f := range v.Files {
		if f.Primary {
			return f, true
		}
	}
	if len(v.Files) > 0 {
		return v.Files[0], true
	}
	return VersionFile{}, false
}

// Checksum returns the strongest checksum Modrinth published for the file,
// preferring sha512 over sha1.
func (f VersionFile) Checksum() fetch.Checksum {
	if hex := f.Hashes["sha512"]; hex != "" {
		return fetch.Checksum{Algo: "sha512", Hex: hex}
	}
	if hex := f.Hashes["sha1"]; hex != "" {
		return fetch.Checksum{Algo: "sha1", Hex: hex}
	}
	return fetch.Checksum{}
}

// LatestVersion returns the newest version of project (an ID or slug) built
// for loader (e.g. "paper") and compatible with gameVersion, or an error if
// none is published.
func LatestVersion(project, gameVersion, loader string) (Version, error) {
	q := url.Values{}
	q.Set("loaders", fmt.Sprintf(`["%s"]`, loader))
	q.Set("game_versions", fmt.Sprintf(`["%s"]`, gameVersion))

	var versions []Version
	if err := getJSON(fmt.Sprintf("%s/project/%s/version?%s", baseURL, url.PathEscape(project), q.Encode()), &versions); err != nil {
		return Version{}, fmt.Errorf("fetching versions for %q: %w", project, err)
	}
	if len(versions) == 0 {
		return Version{}, fmt.Errorf("no %s build of %q found for Minecraft %s", loader, project, gameVersion)
	}
	// Modrinth returns versions newest-first.
	return versions[0], nil
}

func getJSON(url string, out any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
