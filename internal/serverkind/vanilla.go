package serverkind

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// mojangManifestURL is a var (not const) so tests can point it at a fake server.
var mojangManifestURL = "https://launchermeta.mojang.com/mc/game/version_manifest_v2.json"

type Vanilla struct{}

type manifest struct {
	Versions []struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"versions"`
}

type versionMeta struct {
	Downloads struct {
		Server struct {
			URL string `json:"url"`
		} `json:"server"`
	} `json:"downloads"`
	JavaVersion struct {
		MajorVersion int `json:"majorVersion"`
	} `json:"javaVersion"`
}

func fetchVersionMeta(version string) (*versionMeta, error) {
	resp, err := http.Get(mojangManifestURL)
	if err != nil {
		return nil, fmt.Errorf("fetching version manifest: %w", err)
	}
	defer resp.Body.Close()

	var m manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("parsing version manifest: %w", err)
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

	vResp, err := http.Get(versionURL)
	if err != nil {
		return nil, fmt.Errorf("fetching version metadata: %w", err)
	}
	defer vResp.Body.Close()

	var vm versionMeta
	if err := json.NewDecoder(vResp.Body).Decode(&vm); err != nil {
		return nil, fmt.Errorf("parsing version metadata: %w", err)
	}
	return &vm, nil
}

func (Vanilla) ResolveDownloadURL(version string) (string, error) {
	vm, err := fetchVersionMeta(version)
	if err != nil {
		return "", err
	}
	if vm.Downloads.Server.URL == "" {
		return "", fmt.Errorf("no server jar published for minecraft version %q", version)
	}
	return vm.Downloads.Server.URL, nil
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
