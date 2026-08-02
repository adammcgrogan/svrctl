// Package serverkind resolves, per Minecraft server flavor (vanilla, paper, ...),
// where to download the server jar from and which Java major version it needs.
package serverkind

import "fmt"

// Resolver is implemented by each supported server type.
type Resolver interface {
	// ResolveDownloadURL returns the direct download URL for the server jar
	// matching the given Minecraft version.
	ResolveDownloadURL(version string) (string, error)
	// RequiredJavaMajor returns the Java major version (e.g. 21) needed to run
	// the given Minecraft version.
	RequiredJavaMajor(version string) (int, error)
}

// For returns the Resolver for a server type name ("vanilla" or "paper").
func For(kind string) (Resolver, error) {
	switch kind {
	case "vanilla":
		return Vanilla{}, nil
	case "paper":
		return Paper{}, nil
	default:
		return nil, fmt.Errorf("unknown server type %q (want vanilla or paper)", kind)
	}
}
