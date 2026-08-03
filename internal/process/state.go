// state persists and reads the RunState (pid, control-socket port, auth
// token) written by a running server's runner to <serverDir>/.svrctl/run.json.
package process

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func runErrorPath(serverDir string) string {
	return filepath.Join(runStateDir(serverDir), "run-error")
}

// ConsoleLogPath is where the runner records everything the server process
// wrote to stdout and stderr. It deliberately sits outside logs/, which the
// server owns, and it captures the JVM-level output — bad jars, out-of-memory
// kills — that dies before the server's own logging ever starts.
func ConsoleLogPath(serverDir string) string {
	return filepath.Join(runStateDir(serverDir), "console.log")
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

// WriteRunError records why the detached runner failed to start the server,
// so Spawn can surface the real reason instead of just timing out. It's
// tagged on with the tail of the console log, since the Go error alone (e.g.
// "exit status 1" from a crashed JVM) rarely says what actually went wrong —
// that ends up in the child's stderr, which the console log captures.
func WriteRunError(serverDir string, cause error) error {
	if err := os.MkdirAll(runStateDir(serverDir), 0o755); err != nil {
		return err
	}
	msg := cause.Error()
	if tail, err := tailFile(ConsoleLogPath(serverDir), 5); err == nil && len(tail) > 0 {
		msg = msg + "\n" + strings.Join(tail, "\n")
	}
	return os.WriteFile(runErrorPath(serverDir), []byte(msg), 0o644)
}

// tailFile returns up to the last n lines of the file at path.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ring []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		ring = append(ring, scanner.Text())
		if len(ring) > n {
			ring = ring[1:]
		}
	}
	return ring, scanner.Err()
}

func clearRunError(serverDir string) {
	_ = os.Remove(runErrorPath(serverDir))
}

// readRunError returns and clears a startup failure the runner left behind,
// if any.
func readRunError(serverDir string) (string, bool) {
	data, err := os.ReadFile(runErrorPath(serverDir))
	if err != nil {
		return "", false
	}
	clearRunError(serverDir)
	return string(data), true
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
