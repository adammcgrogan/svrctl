// Package process manages the detached "runner" that owns a Minecraft
// server's child process, plus the loopback control socket used by
// `svrctl server console/cmd/stop` to talk to it.
package process

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
)

// RunOptions configures a server launch.
type RunOptions struct {
	ServerDir string
	JavaPath  string
	Memory    string // e.g. "4G"; empty means use JVM defaults
}

// Run starts the Minecraft server child process in the foreground of the
// calling process (intended to be called from the hidden `svrctl __run`
// subcommand, itself spawned detached by Spawn) and blocks until it exits.
func Run(opts RunOptions) error {
	jarPath := filepath.Join(opts.ServerDir, "server.jar")
	if _, err := os.Stat(jarPath); err != nil {
		return fmt.Errorf("server.jar not found in %s: %w", opts.ServerDir, err)
	}

	// Capture the child's stdio to our own file, not logs/latest.log: the
	// server writes that one itself, and two writers truncating and appending
	// to the same path produced a log where every line appeared twice, in two
	// different formats, interleaved unpredictably.
	if err := os.MkdirAll(runStateDir(opts.ServerDir), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(ConsoleLogPath(opts.ServerDir), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	args := []string{}
	if opts.Memory != "" {
		args = append(args, "-Xms"+opts.Memory, "-Xmx"+opts.Memory)
	}
	args = append(args, "-jar", "server.jar", "nogui")

	cmd := exec.Command(opts.JavaPath, args...)
	cmd.Dir = opts.ServerDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting java process: %w", err)
	}

	token, err := randomToken()
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("opening control socket: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	if err := writeRunState(opts.ServerDir, RunState{PID: os.Getpid(), Port: port, Token: token}); err != nil {
		return err
	}
	defer clearRunState(opts.ServerDir)

	b := newBroadcaster(logFile, stdin)
	go pump(stdout, b)
	go pump(stderr, b)
	go acceptLoop(listener, b, token)

	return cmd.Wait()
}

func acceptLoop(listener net.Listener, b *broadcaster, token string) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go handleConn(conn, b, token)
	}
}

// pump copies each line from r into the broadcaster (used for the child
// process's stdout and stderr).
func pump(r io.Reader, b *broadcaster) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		b.broadcast(scanner.Text() + "\n")
	}
}
