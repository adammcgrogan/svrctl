// Package registry manages the servers.yaml file that tracks every server
// svrctl knows about: its type, version, filesystem path, and launch settings.
package registry

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Server describes one registered Minecraft server.
type Server struct {
	Type    string `yaml:"type"`   // "vanilla" or "paper"
	Version string `yaml:"version"`
	Path    string `yaml:"path"`
	Port    int    `yaml:"port,omitempty"`
	Memory  string `yaml:"memory,omitempty"`
}

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

// Save writes the registry to path.
func (r *Registry) Save(path string) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("encoding registry: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
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
