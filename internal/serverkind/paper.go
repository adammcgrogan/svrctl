// Paper resolves PaperMC server jar builds via the PaperMC "Fill" v3 API
// (api.papermc.io's older v2 API was sunset; fill.papermc.io is its replacement).
package serverkind

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// paperAPIBase is a var (not const) so tests can point it at a fake server.
var paperAPIBase = "https://fill.papermc.io/v3/projects/paper"

type Paper struct{}

// paperBuild is one entry in the array returned by
// GET {paperAPIBase}/versions/{version}/builds, which is sorted newest-first.
type paperBuild struct {
	ID        int    `json:"id"`
	Channel   string `json:"channel"`
	Downloads map[string]struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"downloads"`
}

func (Paper) ResolveDownloadURL(version string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/versions/%s/builds", paperAPIBase, version))
	if err != nil {
		return "", fmt.Errorf("fetching paper builds: %w", err)
	}
	defer resp.Body.Close()

	var builds []paperBuild
	if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil {
		return "", fmt.Errorf("parsing paper builds: %w", err)
	}
	if len(builds) == 0 {
		return "", fmt.Errorf("no paper builds found for minecraft version %q", version)
	}

	// Builds are sorted newest-first; prefer the newest STABLE build, falling
	// back to the newest build of any channel if none is marked stable.
	best := builds[0]
	for _, b := range builds {
		if b.Channel == "STABLE" {
			best = b
			break
		}
	}

	dl, ok := best.Downloads["server:default"]
	if !ok {
		return "", fmt.Errorf("paper build %d for %q has no server download", best.ID, version)
	}
	return dl.URL, nil
}

func (Paper) RequiredJavaMajor(version string) (int, error) {
	// Paper doesn't publish its own Java requirement; it tracks vanilla's for the same MC version.
	return Vanilla{}.RequiredJavaMajor(version)
}
