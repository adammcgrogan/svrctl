// socket implements the control-socket line protocol spoken over the runner's
// loopback TCP listener: an AUTH line selects CONSOLE (stream + forward) or
// CMD (send one line) mode.
package process

import (
	"bufio"
	"io"
	"net"
	"strings"
)

// handleConn implements the control-socket protocol. The first line must be:
//
//	AUTH <token> CONSOLE
//	AUTH <token> CMD <command text>
//
// CONSOLE mode streams broadcast log lines to the client and forwards any
// line the client sends back to the child process's stdin, until the client
// disconnects. CMD mode sends one command to stdin and closes.
func handleConn(conn net.Conn, b *broadcaster, token string) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	authLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	fields := strings.SplitN(strings.TrimRight(authLine, "\r\n"), " ", 4)
	if len(fields) < 3 || fields[0] != "AUTH" || fields[1] != token {
		conn.Write([]byte("ERR bad auth\n"))
		return
	}

	switch fields[2] {
	case "CMD":
		if len(fields) == 4 && fields[3] != "" {
			io.WriteString(b.stdin, fields[3]+"\n")
		}
		conn.Write([]byte("OK\n"))
	case "CONSOLE":
		b.addSubscriber(conn)
		defer b.removeSubscriber(conn)
		conn.Write([]byte("OK\n"))
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				continue
			}
			io.WriteString(b.stdin, line+"\n")
		}
	default:
		conn.Write([]byte("ERR unknown mode\n"))
	}
}
