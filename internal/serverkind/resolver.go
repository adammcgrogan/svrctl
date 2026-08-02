// Resolver is the contract each supported Minecraft server flavor implements.
package serverkind

// Resolver knows how to locate a server jar download and its Java requirement
// for a given server flavor (vanilla, paper, ...).
type Resolver interface {
	// ResolveDownloadURL returns the direct download URL for the server jar
	// matching the given Minecraft version.
	ResolveDownloadURL(version string) (string, error)
	// RequiredJavaMajor returns the Java major version (e.g. 21) needed to run
	// the given Minecraft version.
	RequiredJavaMajor(version string) (int, error)
	// ListVersions returns the Minecraft versions this flavor publishes a
	// server for, newest first, so callers can offer a choice instead of
	// making the user guess a version string.
	ListVersions() ([]string, error)
}
