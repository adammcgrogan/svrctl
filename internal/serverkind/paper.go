package serverkind

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// paperAPIBase is a var (not const) so tests can point it at a fake server.
var paperAPIBase = "https://api.papermc.io/v2/projects/paper"

type Paper struct{}

type paperBuildsResponse struct {
	Builds []struct {
		Build     int    `json:"build"`
		Channel   string `json:"channel"`
		Downloads struct {
			Application struct {
				Name string `json:"name"`
			} `json:"application"`
		} `json:"downloads"`
	} `json:"builds"`
}

func (Paper) ResolveDownloadURL(version string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/versions/%s/builds", paperAPIBase, version))
	if err != nil {
		return "", fmt.Errorf("fetching paper builds: %w", err)
	}
	defer resp.Body.Close()

	var br paperBuildsResponse
	if err := json.NewDecoder(resp.Body).Decode(&br); err != nil {
		return "", fmt.Errorf("parsing paper builds: %w", err)
	}
	if len(br.Builds) == 0 {
		return "", fmt.Errorf("no paper builds found for minecraft version %q", version)
	}

	// Prefer the latest "default" channel build; fall back to the latest build overall.
	best := br.Builds[len(br.Builds)-1]
	for i := len(br.Builds) - 1; i >= 0; i-- {
		if br.Builds[i].Channel == "default" {
			best = br.Builds[i]
			break
		}
	}

	return fmt.Sprintf("%s/versions/%s/builds/%d/downloads/%s",
		paperAPIBase, version, best.Build, best.Downloads.Application.Name), nil
}

func (Paper) RequiredJavaMajor(version string) (int, error) {
	// Paper doesn't publish its own Java requirement; it tracks vanilla's for the same MC version.
	return Vanilla{}.RequiredJavaMajor(version)
}
