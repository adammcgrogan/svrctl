// spawn launches the hidden `svrctl __run` subcommand as a detached
// background process that outlives the invoking `svrctl server start`.
package process

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Spawn launches `<self> __run <name>` as a detached background process and
// waits briefly for it to report a listening control socket via run.json.
func Spawn(serverDir, name string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}

	// Clear out any error left by a previous failed attempt so it isn't
	// mistaken for this one.
	clearRunError(serverDir)

	cmd := exec.Command(self, "__run", name)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	setDetached(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawning server process: %w", err)
	}
	// The runner is now independent; don't wait on it.
	go cmd.Process.Release()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := IsRunning(serverDir); ok {
			return nil
		}
		if msg, ok := readRunError(serverDir); ok {
			return fmt.Errorf("server failed to start: %s", msg)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if msg, ok := readRunError(serverDir); ok {
		return fmt.Errorf("server failed to start: %s", msg)
	}
	return fmt.Errorf("timed out waiting for server to start")
}
