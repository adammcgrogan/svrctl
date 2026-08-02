// download resolves the Adoptium (Eclipse Temurin) binary URL for the current
// OS/arch and fetches it into a cache directory.
package javamgr

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
)

// adoptiumBinaryURL is a var (not const) so tests can point it at a fake server.
var adoptiumBinaryURL = "https://api.adoptium.net/v3/binary/latest"

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

func downloadAndExtractJDK(major int, destDir string) error {
	osName, err := adoptiumOS()
	if err != nil {
		return err
	}
	archName, err := adoptiumArch()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/%s/ga/%s/%s/jdk/hotspot/normal/eclipse",
		adoptiumBinaryURL, strconv.Itoa(major), osName, archName)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading JDK %d: %w", major, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading JDK %d: unexpected status %s", major, resp.Status)
	}

	if osName == "windows" {
		return extractZip(resp.Body, destDir)
	}
	return extractTarGz(resp.Body, destDir)
}
