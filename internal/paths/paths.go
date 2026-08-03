// Package paths resolves the OS-appropriate directories svrctl uses for
// config, cached downloads (JDKs, server jars), and other persistent state.
package paths

import (
	"os"
	"path/filepath"
	"strconv"
)

const appName = "svrctl"

// ConfigDir returns the directory holding servers.yaml, creating it if needed.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// RegistryFile returns the full path to servers.yaml.
func RegistryFile() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "servers.yaml"), nil
}

// CacheDir returns the base directory for cached downloads (JDKs, jars), creating it if needed.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// BackupsDir returns the directory holding a server's world backups,
// creating it if needed. Backups live alongside the registry rather than in
// the server's own directory so `svrctl remove --purge` deleting that
// directory doesn't take every backup down with it.
func BackupsDir(serverName string) (string, error) {
	base, err := ConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "backups", serverName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// JDKDir returns the cache directory for a specific JDK major version, creating it if needed.
func JDKDir(major int) (string, error) {
	base, err := CacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "jdks", strconv.Itoa(major))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
