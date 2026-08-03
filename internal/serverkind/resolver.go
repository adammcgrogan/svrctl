// Resolver is the contract each supported Minecraft server flavor implements.
package serverkind

import "github.com/adammcgrogan/svrctl/internal/fetch"

// Download is where to fetch a server jar and, when the source publishes
// one, the checksum to verify it against before it's trusted.
type Download struct {
	URL      string
	Checksum fetch.Checksum
}

// Resolver knows how to locate a server jar download and its Java requirement
// for a given server flavor (vanilla, paper, ...).
type Resolver interface {
	// ResolveDownload returns the direct download for the server jar matching
	// the given Minecraft version.
	ResolveDownload(version string) (Download, error)
	// RequiredJavaMajor returns the Java major version (e.g. 21) needed to run
	// the given Minecraft version.
	RequiredJavaMajor(version string) (int, error)
	// ListVersions returns the Minecraft versions this flavor publishes a
	// server for, newest first, so callers can offer a choice instead of
	// making the user guess a version string.
	ListVersions() ([]string, error)
}
