// provision holds the server-creation pipeline shared by the interactive
// wizard and the flag-driven `svrctl create`. Both paths run exactly the same
// steps; they differ only in how the spec is collected and how progress is
// drawn, which keeps the two from drifting apart.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adammcgrogan/svrctl/internal/fetch"
	"github.com/adammcgrogan/svrctl/internal/javamgr"
	"github.com/adammcgrogan/svrctl/internal/paths"
	"github.com/adammcgrogan/svrctl/internal/registry"
	"github.com/adammcgrogan/svrctl/internal/serverkind"
)

// ProvisionSpec is everything needed to create a server, once the user has
// supplied it by flag or by wizard.
type ProvisionSpec struct {
	Name    string
	Kind    string
	Version string
	Path    string // empty means ~/mcservers/<name>
	Memory  string // empty means JVM defaults
	Port    int    // 0 means leave server.properties alone (Minecraft's 25565)
}

// ProvisionHooks lets the caller render progress however it likes: plain lines
// for a pipe, animated bars inside the wizard.
type ProvisionHooks struct {
	// Phase announces a new step by name, e.g. "Downloading server.jar".
	Phase func(label string)
	// Progress reports byte progress within the current phase.
	Progress func(fetch.Progress)
}

func (h ProvisionHooks) phase(label string) {
	if h.Phase != nil {
		h.Phase(label)
	}
}

func (h ProvisionHooks) reporter() fetch.Reporter {
	if h.Progress == nil {
		return nil
	}
	return fetch.Reporter(h.Progress)
}

// DefaultServerPath returns where a server called name lives unless the user
// picks somewhere else.
func DefaultServerPath(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("mcservers", name)
	}
	return filepath.Join(home, "mcservers", name)
}

// ValidateName rejects names that would be confusing or unsafe as a directory
// component. Catching this before any network work means the user finds out
// immediately rather than after a 200 MB download.
func ValidateName(name string) error {
	switch {
	case strings.TrimSpace(name) == "":
		return fmt.Errorf("name cannot be empty")
	case name != filepath.Base(name) || name == "." || name == "..":
		return fmt.Errorf("name cannot contain path separators")
	case strings.ContainsAny(name, " \t"):
		return fmt.Errorf("name cannot contain spaces")
	}
	return nil
}

// CheckNameAvailable reports whether the registry already has this name.
func CheckNameAvailable(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	reg, _, err := loadRegistry()
	if err != nil {
		return err
	}
	if _, exists := reg.Get(name); exists {
		return fmt.Errorf("a server named %q already exists", name)
	}
	return nil
}

// CheckPathAvailable reports whether absPath already belongs to a registered
// server. Two servers sharing a directory would also share server.jar and
// the runner's .svrctl state, so starting, stopping, or purging either one
// would corrupt the other.
func CheckPathAvailable(absPath string) error {
	reg, _, err := loadRegistry()
	if err != nil {
		return err
	}
	for name, s := range reg.Servers {
		if filepath.Clean(s.Path) == filepath.Clean(absPath) {
			return fmt.Errorf("%q is already used by server %q", absPath, name)
		}
	}
	return nil
}

// Provision creates the server described by spec: it makes the directory,
// downloads the jar, caches the JDK, writes the EULA and port, and registers
// the result. If anything fails partway, a directory Provision created is
// removed again, so a failed create never leaves half a server behind for the
// user to clean up by hand.
func Provision(spec ProvisionSpec, hooks ProvisionHooks) (registry.Server, error) {
	var zero registry.Server

	resolver, err := serverkind.For(spec.Kind)
	if err != nil {
		return zero, err
	}
	if err := CheckNameAvailable(spec.Name); err != nil {
		return zero, err
	}

	path := spec.Path
	if path == "" {
		path = DefaultServerPath(spec.Name)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return zero, err
	}
	if err := CheckPathAvailable(absPath); err != nil {
		return zero, err
	}

	// Remember whether the directory was already there: cleaning up on failure
	// must never delete a directory the user pointed us at.
	_, statErr := os.Stat(absPath)
	createdDir := os.IsNotExist(statErr)
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return zero, fmt.Errorf("creating server directory: %w", err)
	}

	rollback := func() {
		if createdDir {
			os.RemoveAll(absPath)
		}
	}

	hooks.phase(fmt.Sprintf("Finding %s %s", spec.Kind, spec.Version))
	download, err := resolver.ResolveDownload(spec.Version)
	if err != nil {
		rollback()
		return zero, err
	}

	hooks.phase("Downloading server.jar")
	jarPath := filepath.Join(absPath, "server.jar")
	if err := fetch.ToFile(download.URL, jarPath, hooks.reporter()); err != nil {
		rollback()
		return zero, err
	}
	if err := fetch.VerifyFile(jarPath, download.Checksum); err != nil {
		rollback()
		return zero, fmt.Errorf("verifying server.jar: %w", err)
	}

	major, err := resolver.RequiredJavaMajor(spec.Version)
	if err != nil {
		rollback()
		return zero, fmt.Errorf("determining required Java version: %w", err)
	}
	hooks.phase(fmt.Sprintf("Installing Java %d", major))
	if _, err := javamgr.EnsureJDK(major, hooks.reporter()); err != nil {
		rollback()
		return zero, fmt.Errorf("ensuring Java %d is installed: %w", major, err)
	}

	hooks.phase("Writing configuration")
	if err := writeEULA(absPath); err != nil {
		rollback()
		return zero, err
	}
	if spec.Port != 0 {
		if err := writeServerPort(absPath, spec.Port); err != nil {
			rollback()
			return zero, err
		}
	}

	s := registry.Server{
		Type:    spec.Kind,
		Version: spec.Version,
		Path:    absPath,
		Port:    spec.Port,
		Memory:  spec.Memory,
	}

	regPath, err := paths.RegistryFile()
	if err != nil {
		rollback()
		return zero, err
	}
	err = registry.WithLock(regPath, func(reg *registry.Registry) error {
		// Re-check under the lock: another svrctl process could have
		// registered this name or path while this one was downloading.
		if _, exists := reg.Get(spec.Name); exists {
			return fmt.Errorf("a server named %q already exists", spec.Name)
		}
		for other, existing := range reg.Servers {
			if filepath.Clean(existing.Path) == filepath.Clean(absPath) {
				return fmt.Errorf("%q is already used by server %q", absPath, other)
			}
		}
		reg.Put(spec.Name, s)
		return nil
	})
	if err != nil {
		rollback()
		return zero, err
	}
	return s, nil
}

// writeServerPort sets server-port in server.properties, preserving any other
// settings already there rather than truncating the file.
func writeServerPort(serverDir string, port int) error {
	propsPath := filepath.Join(serverDir, "server.properties")
	line := fmt.Sprintf("server-port=%d", port)

	existing, err := os.ReadFile(propsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading server.properties: %w", err)
		}
		if err := os.WriteFile(propsPath, []byte(line+"\n"), 0o644); err != nil {
			return fmt.Errorf("writing server.properties: %w", err)
		}
		return nil
	}

	lines := strings.Split(string(existing), "\n")
	replaced := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "server-port=") {
			lines[i] = line
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, line)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.WriteFile(propsPath, []byte(out), 0o644); err != nil {
		return fmt.Errorf("writing server.properties: %w", err)
	}
	return nil
}
