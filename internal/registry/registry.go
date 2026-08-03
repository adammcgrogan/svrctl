// Package registry loads, saves, and queries the servers.yaml file that
// tracks every server svrctl knows about.
package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Registry is the top-level servers.yaml document.
type Registry struct {
	Servers map[string]Server `yaml:"servers"`
}

// Load reads the registry from path. A missing file yields an empty Registry.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Registry{Servers: map[string]Server{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}
	if reg.Servers == nil {
		reg.Servers = map[string]Server{}
	}
	return &reg, nil
}

// Save writes the registry to path. The write is atomic — data is written to
// a temp file in the same directory and renamed into place — so a crash or
// power loss mid-write can never leave a truncated or invalid registry.
func (r *Registry) Save(path string) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("encoding registry: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".servers-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}
	return nil
}

// Get returns the named server and whether it exists.
func (r *Registry) Get(name string) (Server, bool) {
	s, ok := r.Servers[name]
	return s, ok
}

// Put adds or replaces a server entry.
func (r *Registry) Put(name string, s Server) {
	if r.Servers == nil {
		r.Servers = map[string]Server{}
	}
	r.Servers[name] = s
}

// Remove deletes a server entry.
func (r *Registry) Remove(name string) {
	delete(r.Servers, name)
}
