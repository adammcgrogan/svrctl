// state persists and reads the RunState (pid, control-socket port, auth
// token) written by a running server's runner to <serverDir>/.svrctl/run.json.
package process

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// RunState is persisted to <serverDir>/.svrctl/run.json while a server is running.
type RunState struct {
	PID   int    `json:"pid"`
	Port  int    `json:"port"`
	Token string `json:"token"`
}

func runStateDir(serverDir string) string {
	return filepath.Join(serverDir, ".svrctl")
}

func runStatePath(serverDir string) string {
	return filepath.Join(runStateDir(serverDir), "run.json")
}

func writeRunState(serverDir string, st RunState) error {
	if err := os.MkdirAll(runStateDir(serverDir), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(runStatePath(serverDir), data, 0o600)
}

// ReadRunState loads the persisted run state, if any.
func ReadRunState(serverDir string) (*RunState, bool) {
	data, err := os.ReadFile(runStatePath(serverDir))
	if err != nil {
		return nil, false
	}
	var st RunState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, false
	}
	return &st, true
}

func clearRunState(serverDir string) {
	_ = os.Remove(runStatePath(serverDir))
}

// IsRunning reports whether the process recorded in the run state is alive.
func IsRunning(serverDir string) (*RunState, bool) {
	st, ok := ReadRunState(serverDir)
	if !ok {
		return nil, false
	}
	if !processAlive(st.PID) {
		clearRunState(serverDir)
		return nil, false
	}
	return st, true
}
