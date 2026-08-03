// registry_helpers loads the servers.yaml registry and looks up individual
// server entries by name, shared by every command.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adammcgrogan/svrctl/internal/paths"
	"github.com/adammcgrogan/svrctl/internal/process"
	"github.com/adammcgrogan/svrctl/internal/registry"
)

func loadRegistry() (*registry.Registry, string, error) {
	path, err := paths.RegistryFile()
	if err != nil {
		return nil, "", err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return nil, "", err
	}
	return reg, path, nil
}

func resolveServer(name string) (registry.Server, error) {
	reg, _, err := loadRegistry()
	if err != nil {
		return registry.Server{}, err
	}
	s, ok := reg.Get(name)
	if !ok {
		return registry.Server{}, unknownServerError(reg, name)
	}
	return s, nil
}

// unknownServerError explains what to do about a name that is not registered.
// If one registered name is a near miss it is suggested by name, since the
// overwhelmingly common cause is a typo rather than genuine confusion.
func unknownServerError(reg *registry.Registry, name string) error {
	if suggestion, ok := closestName(reg, name); ok {
		return fmt.Errorf("no server named %q — did you mean %q?", name, suggestion)
	}
	if len(reg.Servers) == 0 {
		return fmt.Errorf("no server named %q, and none are registered yet (run `svrctl create`)", name)
	}
	return fmt.Errorf("no server named %q (run `svrctl list` to see the %d you have)", name, len(reg.Servers))
}

// closestName finds a registered name within a small edit distance of name.
func closestName(reg *registry.Registry, name string) (string, bool) {
	best, bestDist := "", 3 // only suggest genuinely close matches
	for candidate := range reg.Servers {
		if d := editDistance(name, candidate); d < bestDist {
			best, bestDist = candidate, d
		}
	}
	return best, best != ""
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// targetNames resolves which servers a lifecycle/status command should act
// on: either the single name argument, or every server tagged with the given
// group. Exactly one of the two must be supplied.
func targetNames(args []string, group string) ([]string, error) {
	if group != "" {
		if len(args) > 0 {
			return nil, fmt.Errorf("pass a server name or --group, not both")
		}
		views, err := viewAll()
		if err != nil {
			return nil, err
		}
		var names []string
		for _, v := range views {
			if v.Server.Group == group {
				names = append(names, v.Name)
			}
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("no servers in group %q", group)
		}
		return names, nil
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("specify a server name, or --group <name> to act on a whole group")
	}
	return args, nil
}

// forEachTarget runs action for every name and collects the failures rather
// than stopping at the first one, so one bad server in a --group doesn't
// block the rest. It still returns a non-nil error if anything failed, so
// scripts see a non-zero exit code.
func forEachTarget(names []string, action func(name string) error) error {
	var failed []string
	for _, name := range names {
		if err := action(name); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", name, err))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d server(s) failed:\n  %s", len(failed), len(names), strings.Join(failed, "\n  "))
	}
	return nil
}

// serverNames returns every registered name, sorted. Used by shell completion.
func serverNames() []string {
	reg, _, err := loadRegistry()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(reg.Servers))
	for name := range reg.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// serverView is a registry entry plus its live runtime state, which is what
// almost every read-only command actually wants to show.
type serverView struct {
	Name   string
	Server registry.Server
	State  *process.RunState
	Uptime time.Duration
}

func (v serverView) running() bool { return v.State != nil }

// port reports the configured port, falling back to Minecraft's default.
func (v serverView) port() int {
	if v.Server.Port != 0 {
		return v.Server.Port
	}
	return 25565
}

// viewServer gathers the registry entry and runtime state for one server.
func viewServer(name string) (serverView, error) {
	s, err := resolveServer(name)
	if err != nil {
		return serverView{}, err
	}
	return buildView(name, s), nil
}

// viewAll gathers every server's entry and runtime state, sorted by name.
func viewAll() ([]serverView, error) {
	reg, _, err := loadRegistry()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(reg.Servers))
	for name := range reg.Servers {
		names = append(names, name)
	}
	sort.Strings(names)

	views := make([]serverView, 0, len(names))
	for _, name := range names {
		views = append(views, buildView(name, reg.Servers[name]))
	}
	return views, nil
}

func buildView(name string, s registry.Server) serverView {
	v := serverView{Name: name, Server: s}
	if st, ok := process.IsRunning(s.Path); ok {
		v.State = st
		v.Uptime = runUptime(s.Path)
	}
	return v
}

// runUptime approximates how long a server has been up from the mtime of the
// run-state file its runner writes once the control socket is listening.
func runUptime(serverDir string) time.Duration {
	info, err := os.Stat(filepath.Join(serverDir, ".svrctl", "run.json"))
	if err != nil {
		return 0
	}
	return time.Since(info.ModTime())
}
