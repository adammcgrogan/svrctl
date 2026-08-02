// registry_helpers loads the servers.yaml registry and looks up individual
// server entries by name, shared by every server subcommand.
package cli

import (
	"fmt"

	"github.com/adammcgrogan/svrctl/internal/paths"
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
		return registry.Server{}, fmt.Errorf("no server named %q (see `svrctl server list`)", name)
	}
	return s, nil
}
