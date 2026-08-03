// download resolves the Adoptium (Eclipse Temurin) binary URL and published
// checksum for the current OS/arch, and fetches it into a cache directory.
package javamgr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/adammcgrogan/svrctl/internal/fetch"
)

// adoptiumAssetsURL is a var (not const) so tests can point it at a fake server.
var adoptiumAssetsURL = "https://api.adoptium.net/v3/assets/latest"

// adoptiumAsset is the shape of one entry in the assets/latest response,
// trimmed to the fields svrctl needs: where to download the archive and the
// sha256 to verify it against before it's extracted and run.
type adoptiumAsset struct {
	Binary struct {
		Package struct {
			Link     string `json:"link"`
			Checksum string `json:"checksum"`
		} `json:"package"`
	} `json:"binary"`
}

func adoptiumOS() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "mac", nil
	case "windows":
		return "windows", nil
	case "linux":
		return "linux", nil
	default:
		return "", fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
}

func adoptiumArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x64", nil
	case "arm64":
		return "aarch64", nil
	default:
		return "", fmt.Errorf("unsupported architecture %q", runtime.GOARCH)
	}
}

// fetchAdoptiumAsset looks up the latest GA Temurin JDK build for major/os/arch.
func fetchAdoptiumAsset(major int, osName, archName string) (adoptiumAsset, error) {
	url := fmt.Sprintf("%s/%d/hotspot?os=%s&architecture=%s&image_type=jdk&jvm_impl=hotspot&vendor=eclipse",
		adoptiumAssetsURL, major, osName, archName)

	resp, err := fetch.Client.Get(url)
	if err != nil {
		return adoptiumAsset{}, fmt.Errorf("looking up JDK %d: %w", major, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return adoptiumAsset{}, fmt.Errorf("looking up JDK %d: unexpected status %s", major, resp.Status)
	}

	var assets []adoptiumAsset
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return adoptiumAsset{}, fmt.Errorf("parsing JDK %d metadata: %w", major, err)
	}
	if len(assets) == 0 {
		return adoptiumAsset{}, fmt.Errorf("no Temurin JDK %d build found for %s/%s", major, osName, archName)
	}
	return assets[0], nil
}

func downloadAndExtractJDK(major int, destDir string, report fetch.Reporter) error {
	osName, err := adoptiumOS()
	if err != nil {
		return err
	}
	archName, err := adoptiumArch()
	if err != nil {
		return err
	}
	// Temurin never shipped a native macOS/arm64 build for JDK 8; fall back
	// to the x64 build, which runs fine on Apple Silicon under Rosetta.
	if major == 8 && osName == "mac" && archName == "aarch64" {
		archName = "x64"
	}

	asset, err := fetchAdoptiumAsset(major, osName, archName)
	if err != nil {
		return err
	}
	pkg := asset.Binary.Package
	if pkg.Link == "" {
		return fmt.Errorf("JDK %d metadata for %s/%s has no download link", major, osName, archName)
	}

	// Downloaded to a temp file rather than streamed straight into the
	// extractor so the checksum can be verified before anything from the
	// archive is trusted or written to disk.
	tmp, err := os.CreateTemp("", "svrctl-jdk-*.archive")
	if err != nil {
		return fmt.Errorf("downloading JDK %d: %w", major, err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := fetch.ToFile(pkg.Link, tmpPath, report); err != nil {
		return fmt.Errorf("downloading JDK %d: %w", major, err)
	}
	if err := fetch.VerifyFile(tmpPath, fetch.Checksum{Algo: "sha256", Hex: pkg.Checksum}); err != nil {
		return fmt.Errorf("verifying JDK %d download: %w", major, err)
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("reading JDK %d archive: %w", major, err)
	}
	defer f.Close()

	if osName == "windows" {
		return extractZip(f, destDir)
	}
	return extractTarGz(f, destDir)
}
