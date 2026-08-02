// Package javamgr ensures the right JDK major version is available locally,
// downloading and caching Eclipse Temurin builds from Adoptium on demand.
package javamgr

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adammcgrogan/svrctl/internal/fetch"
	"github.com/adammcgrogan/svrctl/internal/paths"
)

// EnsureJDK returns the path to a "java" binary for the given major version,
// downloading and caching the JDK from Adoptium if it isn't already cached.
//
// report, which may be nil, receives download progress. A first-run JDK fetch
// is a few hundred megabytes, so callers should always show it to the user.
func EnsureJDK(major int, report fetch.Reporter) (string, error) {
	dir, err := paths.JDKDir(major)
	if err != nil {
		return "", err
	}

	if bin, ok := findJavaBinary(dir); ok {
		return bin, nil
	}

	if err := downloadAndExtractJDK(major, dir, report); err != nil {
		// A partial extraction would look like a usable cached JDK on the next
		// run, so clear it out rather than leave a broken install behind.
		os.RemoveAll(dir)
		return "", err
	}

	bin, ok := findJavaBinary(dir)
	if !ok {
		return "", fmt.Errorf("java binary not found after extracting JDK %d", major)
	}
	return bin, nil
}

// javaBinName is the name of the java executable for the current OS.
func javaBinName() string {
	if runtime.GOOS == "windows" {
		return "java.exe"
	}
	return "java"
}

// findJavaBinary walks a cached JDK's install tree looking for its bin/java(.exe).
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
