// Paper resolves PaperMC server jar builds via the PaperMC "Fill" v3 API
// (api.papermc.io's older v2 API was sunset; fill.papermc.io is its replacement).
package serverkind

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/adammcgrogan/svrctl/internal/fetch"
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
		Name      string            `json:"name"`
		URL       string            `json:"url"`
		Checksums map[string]string `json:"checksums"`
	} `json:"downloads"`
}

func (Paper) ResolveDownload(version string) (Download, error) {
	resp, err := fetch.Client.Get(fmt.Sprintf("%s/versions/%s/builds", paperAPIBase, version))
	if err != nil {
		return Download{}, fmt.Errorf("fetching paper builds: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Download{}, fmt.Errorf("fetching paper builds: unexpected status %s", resp.Status)
	}

	var builds []paperBuild
	if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil {
		return Download{}, fmt.Errorf("parsing paper builds: %w", err)
	}
	if len(builds) == 0 {
		return Download{}, fmt.Errorf("no paper builds found for minecraft version %q", version)
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
		return Download{}, fmt.Errorf("paper build %d for %q has no server download", best.ID, version)
	}
	return Download{
		URL:      dl.URL,
		Checksum: fetch.Checksum{Algo: "sha256", Hex: dl.Checksums["sha256"]},
	}, nil
}

func (Paper) RequiredJavaMajor(version string) (int, error) {
	// Paper doesn't publish its own Java requirement; it tracks vanilla's for the same MC version.
	return Vanilla{}.RequiredJavaMajor(version)
}

// paperProject is the shape of GET {paperAPIBase}: a map of minor version
// ("1.21") to the full versions under it ("1.21.11", "1.21.10", ...), already
// sorted newest-first within each group.
type paperProject struct {
	Versions map[string][]string `json:"versions"`
}

// ListVersions returns every Minecraft version Paper publishes a build for,
// newest first. Release candidates and pre-releases are dropped for the same
// reason vanilla's snapshots are.
func (Paper) ListVersions() ([]string, error) {
	proj, err := fetchOnce(paperAPIBase, func() (*paperProject, error) {
		return getJSON[paperProject](paperAPIBase)
	})
	if err != nil {
		return nil, fmt.Errorf("fetching paper versions: %w", err)
	}

	// The API groups versions under minor-version keys, and JSON objects have
	// no order, so sort the groups newest-first ourselves before flattening.
	groups := make([]string, 0, len(proj.Versions))
	for k := range proj.Versions {
		groups = append(groups, k)
	}
	sort.Slice(groups, func(i, j int) bool { return compareVersions(groups[i], groups[j]) > 0 })

	var versions []string
	for _, g := range groups {
		for _, v := range proj.Versions[g] {
			if !strings.Contains(v, "-") {
				versions = append(versions, v)
			}
		}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no paper versions found")
	}
	return versions, nil
}

// compareVersions orders dotted numeric versions, returning >0 when a is newer
// than b. Non-numeric components sort before numeric ones, which is enough to
// keep the groups in a sensible order.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var an, bn int
		if i < len(as) {
			an, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bn, _ = strconv.Atoi(bs[i])
		}
		if an != bn {
			return an - bn
		}
	}
	return 0
}
