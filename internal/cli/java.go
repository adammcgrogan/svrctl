// java resolves and lazily installs the JDK a given registered server needs.
package cli

import (
	"fmt"

	"github.com/adammcgrogan/svrctl/internal/fetch"
	"github.com/adammcgrogan/svrctl/internal/javamgr"
	"github.com/adammcgrogan/svrctl/internal/registry"
	"github.com/adammcgrogan/svrctl/internal/serverkind"
)

// resolveJavaPath ensures the correct cached JDK is present for s and returns
// its java binary path. report, which may be nil, receives download progress
// the first time a given Java major version is needed.
func resolveJavaPath(s registry.Server, report fetch.Reporter) (string, error) {
	resolver, err := serverkind.For(s.Type)
	if err != nil {
		return "", err
	}
	major, err := resolver.RequiredJavaMajor(s.Version)
	if err != nil {
		return "", fmt.Errorf("determining required Java version: %w", err)
	}
	javaPath, err := javamgr.EnsureJDK(major, report)
	if err != nil {
		return "", fmt.Errorf("ensuring Java %d is installed: %w", major, err)
	}
	return javaPath, nil
}
