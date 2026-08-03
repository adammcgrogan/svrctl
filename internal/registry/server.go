// Server describes a single registered Minecraft server entry.
package registry

// Server is one entry in the servers.yaml registry.
type Server struct {
	Type    string `yaml:"type"` // "vanilla" or "paper"
	Version string `yaml:"version"`
	Path    string `yaml:"path"`
	Port    int    `yaml:"port,omitempty"`
	Memory  string `yaml:"memory,omitempty"`
	Group   string `yaml:"group,omitempty"` // optional tag for operating on several servers at once
}
