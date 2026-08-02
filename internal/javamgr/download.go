// download resolves the Adoptium (Eclipse Temurin) binary URL for the current
// OS/arch and fetches it into a cache directory.
package javamgr

import (
	"fmt"
	"runtime"
	"strconv"

	"github.com/adammcgrogan/svrctl/internal/fetch"
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

	url := fmt.Sprintf("%s/%s/ga/%s/%s/jdk/hotspot/normal/eclipse",
		adoptiumBinaryURL, strconv.Itoa(major), osName, archName)

	body, total, err := fetch.Open(url)
	if err != nil {
		return fmt.Errorf("downloading JDK %d: %w", major, err)
	}
	defer body.Close()

	// The archive is extracted as it streams, so progress tracks bytes pulled
	// off the wire rather than files written — which is what the wait is.
	src := fetch.Reader(body, total, report)

	if osName == "windows" {
		return extractZip(src, destDir)
	}
	return extractTarGz(src, destDir)
}
