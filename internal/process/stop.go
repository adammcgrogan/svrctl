// stop implements graceful (in-game "stop" command, with wait) and forced
// (process kill) shutdown of a running server.
package process

import (
	"os"
	"time"
)

// Stop asks the server to shut down gracefully, waiting up to timeout before
// force-killing the process. Returns nil if the server was already stopped.
func Stop(serverDir string, timeout time.Duration, force bool) error {
	st, ok := IsRunning(serverDir)
	if !ok {
		return nil
	}

	if !force {
		if err := SendCommand(serverDir, "stop"); err == nil {
			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				if _, running := IsRunning(serverDir); !running {
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	proc, err := os.FindProcess(st.PID)
	if err != nil {
		clearRunState(serverDir)
		return nil
	}
	_ = proc.Kill()
	clearRunState(serverDir)
	return nil
}
