// java resolves and lazily installs the JDK a given registered server needs.
package cli

import (
	"fmt"

	"github.com/adammcgrogan/svrctl/internal/javamgr"
	"github.com/adammcgrogan/svrctl/internal/registry"
	"github.com/adammcgrogan/svrctl/internal/serverkind"
)

// resolveJavaPath ensures the correct cached JDK is present for s and returns its java binary path.
func resolveJavaPath(s registry.Server) (string, error) {
	resolver, err := serverkind.For(s.Type)
	if err != nil {
		return "", err
	}
	major, err := resolver.RequiredJavaMajor(s.Version)
	if err != nil {
		return "", fmt.Errorf("determining required Java version: %w", err)
	}
	javaPath, err := javamgr.EnsureJDK(major)
	if err != nil {
		return "", fmt.Errorf("ensuring Java %d is installed: %w", major, err)
	}
	return javaPath, nil
}
