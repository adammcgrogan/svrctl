// console is the interactive server console.
//
// The old console was an io.Copy in each direction against the raw terminal:
// whatever you were halfway through typing got shredded by the next line of
// server output, there was no scrollback, and the only way out was a Ctrl+C
// that looked indistinguishable from killing the server. This replaces it with
// a proper split view — scrollable output above, a stable input line below.
package tui

import (
	"bufio"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

// maxConsoleLines caps retained scrollback. A busy server emits enough output
// to grow without bound otherwise, and the view is rebuilt from this slice.
const maxConsoleLines = 2000

// ConsoleOptions configures a console session.
type ConsoleOptions struct {
	// Name of the server, for the header.
	Name string
	// Detail is the subtitle shown top-right, e.g. "paper 1.21.1".
	Detail string
	// Conn is an authenticated console connection: reads yield server output,
	// writes are forwarded to the server's stdin.
	Conn io.ReadWriter
	// Seed pre-fills the scrollback, normally the tail of latest.log, so
	// attaching shows context instead of an empty screen.
	Seed []string
}

// RunConsole attaches an interactive console and blocks until the user
// detaches or the connection closes.
func RunConsole(opts ConsoleOptions) error {
	m := newConsoleModel(opts)
	go m.readLoop()

	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// newConsoleModel builds the console's initial state. Kept separate from
// RunConsole so tests can drive the model without a terminal or a live server.
func newConsoleModel(opts ConsoleOptions) consoleModel {
	input := textinput.New()
	input.Placeholder = "type a server command, e.g. list"
	input.Prompt = ui.Running.Render(ui.GlyphPrompt + " ")
	input.CharLimit = 512
	input.Focus()

	return consoleModel{
		opts:     opts,
		input:    input,
		vp:       viewport.New(80, 20),
		lines:    append([]string{}, opts.Seed...),
		incoming: make(chan string, 256),
		closed:   make(chan struct{}),
	}
}

type consoleModel struct {
	opts  ConsoleOptions
	input textinput.Model
	vp    viewport.Model

	lines   []string
	history []string
	histIdx int // index into history while browsing; len(history) means "not browsing"

	incoming chan string
	closed   chan struct{}

	width  int
	height int
	ready  bool
	status string
}

// readLoop pumps the connection into a channel the Bubble Tea loop drains.
func (m consoleModel) readLoop() {
	scanner := bufio.NewScanner(m.opts.Conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		m.incoming <- scanner.Text()
	}
	close(m.closed)
}

type serverLineMsg string
type serverClosedMsg struct{}

// waitForLine blocks until the next line arrives or the server disconnects.
func (m consoleModel) waitForLine() tea.Cmd {
	return func() tea.Msg {
		select {
		case line := <-m.incoming:
			return serverLineMsg(line)
		case <-m.closed:
			return serverClosedMsg{}
		}
	}
}

func (m consoleModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.waitForLine())
}

func (m consoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.ready = true
		m.refresh()

	case serverLineMsg:
		m.append(string(msg))
		m.refresh()
		cmds = append(cmds, m.waitForLine())

	case serverClosedMsg:
		m.status = ui.Warning.Render("server disconnected — press esc to close")

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			// Detaching is not stopping. Say so on the way out, because the
			// old console left people unsure whether they had killed anything.
			return m, tea.Quit

		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				break
			}
			m.input.Reset()
			m.history = append(m.history, text)
			m.histIdx = len(m.history)
			// Echo locally: the server writes command *results* to stdout but
			// never the command itself, so without this the transcript reads
			// as answers to unasked questions.
			m.append(ui.Running.Render(ui.GlyphPrompt+" ") + ui.Strong.Render(text))
			m.refresh()
			if _, err := io.WriteString(m.opts.Conn, text+"\n"); err != nil {
				m.status = ui.Failure.Render("could not send: " + err.Error())
			}

		case tea.KeyUp:
			if len(m.history) > 0 && m.histIdx > 0 {
				m.histIdx--
				m.input.SetValue(m.history[m.histIdx])
				m.input.CursorEnd()
			}

		case tea.KeyDown:
			if m.histIdx < len(m.history)-1 {
				m.histIdx++
				m.input.SetValue(m.history[m.histIdx])
				m.input.CursorEnd()
			} else {
				m.histIdx = len(m.history)
				m.input.Reset()
			}

		case tea.KeyPgUp, tea.KeyPgDown:
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// layout sizes the viewport around the fixed header, rule, and input rows.
func (m *consoleModel) layout() {
	const chrome = 5 // header, rule, rule, input, help
	h := m.height - chrome
	if h < 3 {
		h = 3
	}
	m.vp.Width = m.width
	m.vp.Height = h
	m.input.Width = m.width - 4
}

func (m *consoleModel) append(line string) {
	m.lines = append(m.lines, line)
	if len(m.lines) > maxConsoleLines {
		m.lines = m.lines[len(m.lines)-maxConsoleLines:]
	}
}

// refresh re-renders the scrollback and sticks to the bottom unless the user
// has scrolled up to read something, in which case their position is kept.
func (m *consoleModel) refresh() {
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

// colorizeLogLine tints a Minecraft log line by severity so warnings and
// stack traces stand out of the wall of INFO.
func colorizeLogLine(line string) string {
	switch {
	case strings.Contains(line, "/ERROR]"), strings.Contains(line, "/FATAL]"):
		return ui.Failure.Render(line)
	case strings.Contains(line, "/WARN]"):
		return ui.Warning.Render(line)
	default:
		return line
	}
}

func (m consoleModel) View() string {
	if !m.ready {
		return "\n  attaching…"
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		ui.Title.Render(" "+m.opts.Name),
		"  ",
		ui.Running.Render(ui.GlyphRunning+" attached"),
	)
	if m.opts.Detail != "" {
		gap := m.width - lipgloss.Width(header) - lipgloss.Width(m.opts.Detail) - 1
		if gap > 0 {
			header += strings.Repeat(" ", gap) + ui.Subtle.Render(m.opts.Detail)
		}
	}

	footer := m.status
	if footer == "" {
		footer = ui.HelpBar(
			"enter", "send",
			"↑↓", "history",
			"pgup/pgdn", "scroll",
			"esc", "detach (server keeps running)",
		)
	}

	return strings.Join([]string{
		header,
		ui.Rule(m.width),
		m.vp.View(),
		ui.Rule(m.width),
		" " + m.input.View(),
		" " + footer,
	}, "\n")
}
