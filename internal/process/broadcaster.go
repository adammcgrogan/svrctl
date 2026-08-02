// broadcaster fans out the running server's console output to the log file
// and to any attached control-socket subscribers, and relays their input
// back to the server's stdin.
package process

import (
	"io"
	"net"
	"os"
	"sync"
)

type broadcaster struct {
	mu          sync.Mutex
	logFile     *os.File
	stdin       io.WriteCloser
	subscribers map[net.Conn]struct{}
}

func newBroadcaster(logFile *os.File, stdin io.WriteCloser) *broadcaster {
	return &broadcaster{logFile: logFile, stdin: stdin, subscribers: map[net.Conn]struct{}{}}
}

func (b *broadcaster) broadcast(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logFile.WriteString(line)
	for c := range b.subscribers {
		_, _ = c.Write([]byte(line))
	}
}

func (b *broadcaster) addSubscriber(c net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[c] = struct{}{}
}

func (b *broadcaster) removeSubscriber(c net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, c)
}
