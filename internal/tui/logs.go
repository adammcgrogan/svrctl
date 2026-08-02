// logs is a read-only, scrollable view of a server's log output.
//
// Console already shows a running server's log as it seeds its scrollback
// from latest.log, but it only ever attaches to a live control connection —
// a stopped server has no way to see what happened from the dashboard at
// all. This reuses console's viewport for that gap, without the input line
// or the connection console needs.
package tui

import (
	"bufio"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

// maxLogViewLines caps retained scrollback, matching the console's limit.
const maxLogViewLines = 2000

// LogsOptions configures a log viewer session.
type LogsOptions struct {
	// Name of the server, for the header.
	Name string
	// Detail is the subtitle shown top-right, e.g. "paper 1.21.1".
	Detail string
	// Seed pre-fills the scrollback, normally the tail of the log file, so
	// opening the viewer shows context instead of an empty screen.
	Seed []string
	// Follow streams newly appended log lines. Its Read blocks rather than
	// returning EOF while waiting for more — the same contract as `tail -f`
	// — and is closed automatically when the viewer exits.
	Follow io.ReadCloser
}

// RunLogs shows the log viewer and blocks until the user backs out.
func RunLogs(opts LogsOptions) error {
	m := newLogsModel(opts)
	defer opts.Follow.Close()
	go m.readLoop()

	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newLogsModel(opts LogsOptions) logsModel {
	return logsModel{
		opts:     opts,
		vp:       viewport.New(80, 20),
		lines:    append([]string{}, opts.Seed...),
		incoming: make(chan string, 256),
		closed:   make(chan struct{}),
	}
}

type logsModel struct {
	opts LogsOptions
	vp   viewport.Model

	lines []string

	incoming chan string
	closed   chan struct{}

	width, height int
	ready         bool
	status        string
}

// readLoop pumps the follower into a channel the Bubble Tea loop drains.
func (m logsModel) readLoop() {
	scanner := bufio.NewScanner(m.opts.Follow)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		m.incoming <- scanner.Text()
	}
	close(m.closed)
}

type logLineMsg string
type logEndedMsg struct{}

// waitForLine blocks until the next line arrives or the log ends.
func (m logsModel) waitForLine() tea.Cmd {
	return func() tea.Msg {
		select {
		case line := <-m.incoming:
			return logLineMsg(line)
		case <-m.closed:
			return logEndedMsg{}
		}
	}
}

func (m logsModel) Init() tea.Cmd {
	return m.waitForLine()
}

func (m logsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.ready = true
		m.refresh()
		return m, nil

	case logLineMsg:
		m.append(string(msg))
		m.refresh()
		return m, m.waitForLine()

	case logEndedMsg:
		m.status = ui.Subtle.Render("log ended — press esc to close")
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc || msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// layout sizes the viewport around the fixed header, rule, and footer rows.
func (m *logsModel) layout() {
	const chrome = 3 // header, rule, help
	h := m.height - chrome
	if h < 3 {
		h = 3
	}
	m.vp.Width = m.width
	m.vp.Height = h
}

func (m *logsModel) append(line string) {
	m.lines = append(m.lines, line)
	if len(m.lines) > maxLogViewLines {
		m.lines = m.lines[len(m.lines)-maxLogViewLines:]
	}
}

// refresh re-renders the scrollback and sticks to the bottom unless the user
// has scrolled up to read something, in which case their position is kept.
func (m *logsModel) refresh() {
	wrap := lipgloss.NewStyle().Width(max(m.vp.Width, 20))
	rendered := make([]string, 0, len(m.lines))
	for _, l := range m.lines {
		rendered = append(rendered, wrap.Render(colorizeLogLine(l)))
	}
	atBottom := m.vp.AtBottom()
	m.vp.SetContent(strings.Join(rendered, "\n"))
	if atBottom {
		m.vp.GotoBottom()
	}
}

func (m logsModel) View() string {
	if !m.ready {
		return "\n  loading…"
	}

	header := ui.Title.Render(" " + m.opts.Name + " logs")
	if m.opts.Detail != "" {
		gap := m.width - lipgloss.Width(header) - lipgloss.Width(m.opts.Detail) - 1
		if gap > 0 {
			header += strings.Repeat(" ", gap) + ui.Subtle.Render(m.opts.Detail)
		}
	}

	footer := m.status
	if footer == "" {
		footer = ui.HelpBar("↑↓", "scroll", "pgup/pgdn", "page", "esc", "back")
	}

	return strings.Join([]string{
		header,
		ui.Rule(m.width),
		m.vp.View(),
		" " + footer,
	}, "\n")
}
