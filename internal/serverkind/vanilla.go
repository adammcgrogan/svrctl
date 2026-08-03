// Vanilla resolves official Mojang server jars via the launcher version manifest.
package serverkind

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/adammcgrogan/svrctl/internal/fetch"
)

// mojangManifestURL is a var (not const) so tests can point it at a fake server.
var mojangManifestURL = "https://launchermeta.mojang.com/mc/game/version_manifest_v2.json"

type Vanilla struct{}

type manifest struct {
	Latest struct {
		Release string `json:"release"`
	} `json:"latest"`
	Versions []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"versions"`
}

type versionMeta struct {
	Downloads struct {
		Server struct {
			URL  string `json:"url"`
			Sha1 string `json:"sha1"`
		} `json:"server"`
	} `json:"downloads"`
	JavaVersion struct {
		MajorVersion int `json:"majorVersion"`
	} `json:"javaVersion"`
}

// getJSON fetches and decodes a JSON document into a fresh T.
func getJSON[T any](url string) (*T, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func fetchManifest() (*manifest, error) {
	m, err := fetchOnce(mojangManifestURL, func() (*manifest, error) {
		return getJSON[manifest](mojangManifestURL)
	})
	if err != nil {
		return nil, fmt.Errorf("fetching version manifest: %w", err)
	}
	return m, nil
}

func fetchVersionMeta(version string) (*versionMeta, error) {
	m, err := fetchManifest()
	if err != nil {
		return nil, err
	}

	var versionURL string
	for _, v := range m.Versions {
		if v.ID == version {
			versionURL = v.URL
			break
		}
	}
	if versionURL == "" {
		return nil, fmt.Errorf("minecraft version %q not found in manifest", version)
	}

	vm, err := fetchOnce(versionURL, func() (*versionMeta, error) {
		return getJSON[versionMeta](versionURL)
	})
	if err != nil {
		return nil, fmt.Errorf("fetching version metadata: %w", err)
	}
	return vm, nil
}

func (Vanilla) ResolveDownload(version string) (Download, error) {
	vm, err := fetchVersionMeta(version)
	if err != nil {
		return Download{}, err
	}
	if vm.Downloads.Server.URL == "" {
		return Download{}, fmt.Errorf("no server jar published for minecraft version %q", version)
	}
	return Download{
		URL:      vm.Downloads.Server.URL,
		Checksum: fetch.Checksum{Algo: "sha1", Hex: vm.Downloads.Server.Sha1},
	}, nil
}

func (Vanilla) RequiredJavaMajor(version string) (int, error) {
	vm, err := fetchVersionMeta(version)
	if err != nil {
		return 0, err
	}
	if vm.JavaVersion.MajorVersion == 0 {
		return 0, fmt.Errorf("no java version metadata for minecraft version %q", version)
	}
	return vm.JavaVersion.MajorVersion, nil
}

// ListVersions returns Mojang's released versions, newest first. Snapshots and
// pre-releases are filtered out: they are rarely what someone spinning up a
// server wants, and including them buries the versions that are.
func (Vanilla) ListVersions() ([]string, error) {
	m, err := fetchManifest()
	if err != nil {
		return nil, err
	}
	versions := make([]string, 0, len(m.Versions))
	for _, v := range m.Versions {
		if v.Type == "release" {
			versions = append(versions, v.ID)
		}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no released versions found in manifest")
	}
	return versions, nil
}
