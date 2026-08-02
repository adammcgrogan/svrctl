// Package javamgr ensures the right JDK major version is available locally,
// downloading and caching Eclipse Temurin builds from Adoptium on demand.
package javamgr

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/adammcgrogan/svrctl/internal/paths"
)

// adoptiumBinaryURL is a var (not const) so tests can point it at a fake server.
var adoptiumBinaryURL = "https://api.adoptium.net/v3/binary/latest"

// javaBinName is the name of the java executable for the current OS.
func javaBinName() string {
	if runtime.GOOS == "windows" {
		return "java.exe"
	}
	return "java"
}

// EnsureJDK returns the path to a "java" binary for the given major version,
// downloading and caching the JDK from Adoptium if it isn't already cached.
func EnsureJDK(major int) (string, error) {
	dir, err := paths.JDKDir(major)
	if err != nil {
		return "", err
	}

	if bin, ok := findJavaBinary(dir); ok {
		return bin, nil
	}

	if err := downloadAndExtractJDK(major, dir); err != nil {
		return "", err
	}

	bin, ok := findJavaBinary(dir)
	if !ok {
		return "", fmt.Errorf("java binary not found after extracting JDK %d", major)
	}
	return bin, nil
}

func findJavaBinary(root string) (string, bool) {
	var found string
	name := javaBinName()
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if !d.IsDir() && d.Name() == name && strings.Contains(filepath.ToSlash(path), "/bin/") {
			found = path
		}
		return nil
	})
	return found, found != ""
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

func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("opening gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		target, err := safeJoin(destDir, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			// Skip symlinks; JDK archives use them for convenience paths we don't rely on.
		}
	}
}

func extractZip(r io.Reader, destDir string) error {
	tmp, err := os.CreateTemp("", "svrctl-jdk-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("buffering zip download: %w", err)
	}

	zr, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return fmt.Errorf("opening zip archive: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		target, err := safeJoin(destDir, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

// safeJoin joins base and name, rejecting archive entries that would escape base ("zip slip").
func safeJoin(base, name string) (string, error) {
	target := filepath.Join(base, name)
	if !strings.HasPrefix(target, filepath.Clean(base)+string(os.PathSeparator)) && target != filepath.Clean(base) {
		return "", fmt.Errorf("archive entry %q escapes destination directory", name)
	}
	return target, nil
}
