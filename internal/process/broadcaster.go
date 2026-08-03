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

// subscriberQueueSize bounds how many unwritten lines a subscriber can
// accumulate before it's treated as a laggard and disconnected, so a stuck
// socket can't stall broadcast delivery to everyone else.
const subscriberQueueSize = 256

type subscriber struct {
	conn net.Conn
	ch   chan string
}

type broadcaster struct {
	mu          sync.Mutex
	logFile     *os.File
	stdin       io.WriteCloser
	subscribers map[net.Conn]*subscriber
}

func newBroadcaster(logFile *os.File, stdin io.WriteCloser) *broadcaster {
	return &broadcaster{logFile: logFile, stdin: stdin, subscribers: map[net.Conn]*subscriber{}}
}

func (b *broadcaster) broadcast(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logFile.WriteString(line)
	for c, sub := range b.subscribers {
		select {
		case sub.ch <- line:
		default:
			// Subscriber isn't draining fast enough; drop it rather than
			// block delivery to the log file and every other subscriber.
			close(sub.ch)
			delete(b.subscribers, c)
			c.Close()
		}
	}
}

func (b *broadcaster) addSubscriber(c net.Conn) {
	sub := &subscriber{conn: c, ch: make(chan string, subscriberQueueSize)}
	b.mu.Lock()
	b.subscribers[c] = sub
	b.mu.Unlock()

	go func() {
		for line := range sub.ch {
			if _, err := c.Write([]byte(line)); err != nil {
				b.removeSubscriber(c)
				return
			}
		}
	}()
}

func (b *broadcaster) removeSubscriber(c net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sub, ok := b.subscribers[c]; ok {
		close(sub.ch)
		delete(b.subscribers, c)
	}
}
