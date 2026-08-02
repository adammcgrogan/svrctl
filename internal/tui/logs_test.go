// logs_test drives the log viewer model through Update/View directly.
package tui

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

// blockingReader never yields data and never returns until closed, standing
// in for a Follow reader with nothing new to say.
type blockingReader struct {
	closed chan struct{}
}

func newBlockingReader() *blockingReader {
	return &blockingReader{closed: make(chan struct{})}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	<-r.closed
	return 0, io.EOF
}

func (r *blockingReader) Close() error {
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
	return nil
}

func newTestLogsModel(opts LogsOptions) logsModel {
	m := newLogsModel(opts)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return next.(logsModel)
}

func TestLogsShowsSeededLines(t *testing.T) {
	m := newTestLogsModel(LogsOptions{
		Name:   "survival",
		Seed:   []string{"line one", "line two"},
		Follow: newBlockingReader(),
	})

	view := ui.StripANSI(m.View())
	for _, want := range []string{"survival", "line one", "line two"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestLogsAppendsFollowedLines(t *testing.T) {
	m := newTestLogsModel(LogsOptions{Follow: newBlockingReader()})

	next, _ := m.Update(logLineMsg("fresh line"))
	m = next.(logsModel)

	view := ui.StripANSI(m.View())
	if !strings.Contains(view, "fresh line") {
		t.Errorf("view missing followed line:\n%s", view)
	}
}

func TestLogsEscQuits(t *testing.T) {
	m := newTestLogsModel(LogsOptions{Follow: newBlockingReader()})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to quit")
	}
}

func TestLogsShowsEndedStatus(t *testing.T) {
	m := newTestLogsModel(LogsOptions{Follow: newBlockingReader()})

	next, _ := m.Update(logEndedMsg{})
	m = next.(logsModel)

	view := ui.StripANSI(m.View())
	if !strings.Contains(view, "log ended") {
		t.Errorf("view missing end-of-log status:\n%s", view)
	}
}
