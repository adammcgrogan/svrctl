// console_test drives the console model without a terminal or a live server.
package tui

import (
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

// fakeConn stands in for the control-socket connection: reads never yield
// (the test injects lines as messages instead) and writes are recorded.
type fakeConn struct {
	mu      sync.Mutex
	written strings.Builder
	block   chan struct{}
}

func newFakeConn() *fakeConn { return &fakeConn{block: make(chan struct{})} }

func (c *fakeConn) Read(p []byte) (int, error) {
	<-c.block
	return 0, nil
}

func (c *fakeConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.written.Write(p)
}

func (c *fakeConn) sent() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.written.String()
}

func newTestConsole(conn *fakeConn, seed ...string) consoleModel {
	m := newConsoleModel(ConsoleOptions{
		Name:   "survival",
		Detail: "paper 1.21.1 · port 25565",
		Conn:   conn,
		Seed:   seed,
	})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	return next.(consoleModel)
}

func sendConsole(m consoleModel, msgs ...tea.Msg) consoleModel {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(consoleModel)
	}
	return m
}

func typeConsole(m consoleModel, s string) consoleModel {
	for _, r := range s {
		m = sendConsole(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestConsoleSeedsScrollbackFromTheLog(t *testing.T) {
	m := newTestConsole(newFakeConn(), "[12:00:00] [Server thread/INFO]: Done (5.2s)!")

	// Attaching to a quiet server used to show a blank screen until it next
	// spoke, which read as a broken connection.
	if view := ui.StripANSI(m.View()); !strings.Contains(view, "Done (5.2s)!") {
		t.Errorf("seeded log line not shown:\n%s", view)
	}
}

func TestConsoleShowsServerOutput(t *testing.T) {
	m := sendConsole(newTestConsole(newFakeConn()), serverLineMsg("[12:00:01] [Server thread/INFO]: Ada joined the game"))

	if view := ui.StripANSI(m.View()); !strings.Contains(view, "Ada joined the game") {
		t.Errorf("server output not shown:\n%s", view)
	}
}

func TestConsoleSendsAndEchoesTypedCommands(t *testing.T) {
	conn := newFakeConn()
	m := typeConsole(newTestConsole(conn), "say hello")
	m = sendConsole(m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := conn.sent(); got != "say hello\n" {
		t.Errorf("sent %q, want %q", got, "say hello\n")
	}
	// The server echoes results but never the command, so the console does it.
	if view := ui.StripANSI(m.View()); !strings.Contains(view, "say hello") {
		t.Errorf("typed command not echoed into the transcript:\n%s", view)
	}
	if m.input.Value() != "" {
		t.Errorf("input not cleared after sending, still %q", m.input.Value())
	}
}

func TestConsoleIgnoresEmptySubmissions(t *testing.T) {
	conn := newFakeConn()
	m := sendConsole(newTestConsole(conn), tea.KeyMsg{Type: tea.KeyEnter})
	m = typeConsole(m, "   ")
	m = sendConsole(m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := conn.sent(); got != "" {
		t.Errorf("blank input was sent to the server as %q", got)
	}
}

func TestConsoleRecallsHistory(t *testing.T) {
	conn := newFakeConn()
	m := typeConsole(newTestConsole(conn), "list")
	m = sendConsole(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = typeConsole(m, "time set day")
	m = sendConsole(m, tea.KeyMsg{Type: tea.KeyEnter})

	m = sendConsole(m, tea.KeyMsg{Type: tea.KeyUp})
	if got := m.input.Value(); got != "time set day" {
		t.Errorf("first recall gave %q, want the most recent command", got)
	}
	m = sendConsole(m, tea.KeyMsg{Type: tea.KeyUp})
	if got := m.input.Value(); got != "list" {
		t.Errorf("second recall gave %q, want the earlier command", got)
	}
	m = sendConsole(m, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyDown})
	if got := m.input.Value(); got != "" {
		t.Errorf("walking past the newest entry left %q, want an empty input", got)
	}
}

func TestConsoleSaysDetachingLeavesTheServerUp(t *testing.T) {
	view := ui.StripANSI(newTestConsole(newFakeConn()).View())

	// The old console's only exit was a Ctrl+C that looked like a kill.
	if !strings.Contains(view, "detach") || !strings.Contains(view, "keeps running") {
		t.Errorf("footer does not reassure that detaching is safe:\n%s", view)
	}
}

func TestConsoleReportsDisconnection(t *testing.T) {
	m := sendConsole(newTestConsole(newFakeConn()), serverClosedMsg{})

	if view := ui.StripANSI(m.View()); !strings.Contains(view, "disconnected") {
		t.Errorf("a dropped connection is not reported:\n%s", view)
	}
}

func TestConsoleCapsScrollback(t *testing.T) {
	m := newTestConsole(newFakeConn())
	for i := 0; i < maxConsoleLines+50; i++ {
		m.append("line")
	}
	if len(m.lines) > maxConsoleLines {
		t.Errorf("scrollback grew to %d lines, cap is %d", len(m.lines), maxConsoleLines)
	}
}

func TestColorizeLogLineMarksSeverity(t *testing.T) {
	warn := colorizeLogLine("[12:00:00] [Server thread/WARN]: Can't keep up!")
	errLine := colorizeLogLine("[12:00:00] [Server thread/ERROR]: Exception")
	info := colorizeLogLine("[12:00:00] [Server thread/INFO]: Done")

	if warn == ui.StripANSI(warn) {
		t.Error("warnings are not highlighted")
	}
	if errLine == ui.StripANSI(errLine) {
		t.Error("errors are not highlighted")
	}
	if info != ui.StripANSI(info) {
		t.Error("ordinary INFO lines should be left unstyled")
	}
}
