// attach runs a server's java process in the foreground with its stdio
// connected directly to the current terminal (used by `server start --attach`).
package cli

import (
	"os"
	"os/exec"
)

func runAttached(serverDir, javaPath, memory string) error {
	args := []string{}
	if memory != "" {
		args = append(args, "-Xms"+memory, "-Xmx"+memory)
	}
	args = append(args, "-jar", "server.jar", "nogui")

	c := exec.Command(javaPath, args...)
	c.Dir = serverDir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
