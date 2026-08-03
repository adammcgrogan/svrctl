// client implements the control-socket calls used by `svrctl server cmd` and
// `svrctl server console` to talk to an already-running server's runner.
package process

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

func dial(serverDir string) (net.Conn, *RunState, error) {
	st, ok := IsRunning(serverDir)
	if !ok {
		return nil, nil, fmt.Errorf("server is not running")
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", st.Port), 3*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to server control socket: %w", err)
	}
	return conn, st, nil
}

// verifyControlSocket confirms a PID that's merely alive is actually our
// runner, by checking that its recorded control socket answers to the token
// we handed it when it started. An unrelated process the OS reassigned that
// PID to won't have anything listening there, or won't know the token.
func verifyControlSocket(st *RunState) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", st.Port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if _, err := fmt.Fprintf(conn, "AUTH %s CMD\n", st.Token); err != nil {
		return false
	}
	resp, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && resp == "OK\n"
}

// SendCommand sends a single console command to the running server and waits for acknowledgement.
func SendCommand(serverDir, text string) error {
	conn, st, err := dial(serverDir)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "AUTH %s CMD %s\n", st.Token, text); err != nil {
		return err
	}
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("no response from server: %w", err)
	}
	if resp != "OK\n" {
		return fmt.Errorf("server rejected command: %s", resp)
	}
	return nil
}

// OpenConsole authenticates a CONSOLE-mode connection and returns it for the
// caller to pump bidirectionally (read = live log lines, write = commands).
func OpenConsole(serverDir string) (net.Conn, error) {
	conn, st, err := dial(serverDir)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(conn, "AUTH %s CONSOLE\n", st.Token); err != nil {
		conn.Close()
		return nil, err
	}
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || resp != "OK\n" {
		conn.Close()
		return nil, fmt.Errorf("server rejected console attach")
	}
	return conn, nil
}
