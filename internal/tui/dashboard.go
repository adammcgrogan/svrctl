// dashboard is what `svrctl` shows when run with no arguments.
//
// Previously that printed a wall of Cobra help, leaving a new user to work out
// which of ten subcommands to try first. The dashboard instead shows the thing
// they came to see — their servers and whether each is up — and puts the
// common actions on single keys.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

// refreshInterval is how often the dashboard re-checks which servers are up.
// Fast enough that a server started elsewhere shows up promptly, slow enough
// that it is not stat-ing the filesystem in a tight loop.
const refreshInterval = 2 * time.Second

// ServerRow is one server as the dashboard needs to display it.
type ServerRow struct {
	Name    string
	Type    string
	Version string
	Path    string
	Port    int
	Running bool
	PID     int
	Uptime  time.Duration
}

// Action is what the user asked for as the dashboard closed. Actions that need
// the whole terminal cannot run inside the dashboard's own Bubble Tea program,
// so they are handed back to the caller to perform.
type Action int

const (
	ActionQuit Action = iota
	ActionConsole
	ActionCreate
)

// Outcome is the dashboard's exit result.
type Outcome struct {
	Action Action
	Server string
}

// DashboardDeps are the operations the dashboard performs on the user's
// behalf, injected so this package does not depend on the command layer.
type DashboardDeps struct {
	List    func() ([]ServerRow, error)
	Start   func(name string) error
	Stop    func(name string) error
	Restart func(name string) error
	// Remove unregisters a server, deleting its files too when purge is set.
	Remove func(name string, purge bool) error
}

// RunDashboard shows the dashboard and reports what to do next.
func RunDashboard(deps DashboardDeps) (Outcome, error) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.Running

	final, err := tea.NewProgram(dashboardModel{
		deps:    deps,
		spinner: sp,
		width:   100,
		height:  24,
	}, tea.WithAltScreen()).Run()
	if err != nil {
		return Outcome{}, err
	}
	m := final.(dashboardModel)
	return m.outcome, m.err
}

type dashboardModel struct {
	deps    DashboardDeps
	rows    []ServerRow
	cursor  int
	spinner spinner.Model

	// busy names the action in flight, e.g. "starting survival". Actions are
	// slow (a start waits for the control socket), so the dashboard says what
	// it is doing rather than freezing silently.
	busy    string
	status  string
	failure string

	// confirming asks which of the two remove modes the user means, so a
	// single keystroke can never destroy a world by accident — it stays
	// inside this program rather than handing back an Action, since it does
	// not need the whole terminal.
	confirming bool

	width, height int
	outcome       Outcome
	err           error
}

type rowsMsg struct {
	rows []ServerRow
	err  error
}
type actionDoneMsg struct {
	verb string
	name string
	err  error
}
type tickMsg time.Time

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(m.refresh(), m.spinner.Tick, tick())
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m dashboardModel) refresh() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.deps.List()
		return rowsMsg{rows: rows, err: err}
	}
}

// runAction performs a lifecycle operation off the UI goroutine.
func (m dashboardModel) runAction(verb, name string, fn func(string) error) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{verb: verb, name: name, err: fn(name)}
	}
}

// runRemove performs the remove off the UI goroutine, same as runAction but
// for the one lifecycle operation that takes a second argument.
func (m dashboardModel) runRemove(name string, purge bool) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{verb: "removed", name: name, err: m.deps.Remove(name, purge)}
	}
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		return m, tea.Batch(m.refresh(), tick())

	case rowsMsg:
		if msg.err != nil {
			m.failure = msg.err.Error()
			return m, nil
		}
		m.rows = msg.rows
		if m.cursor >= len(m.rows) {
			m.cursor = max(len(m.rows)-1, 0)
		}
		return m, nil

	case actionDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = fmt.Sprintf("could not %s %s: %v", msg.verb, msg.name, msg.err)
		} else {
			m.status = fmt.Sprintf("%s %s", msg.verb, msg.name)
		}
		return m, m.refresh()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m dashboardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// One action at a time: a second start while the first is still waiting on
	// the control socket would race for the same directory.
	if m.busy != "" {
		return m, nil
	}
	if m.confirming {
		return m.handleConfirmKey(msg)
	}

	m.status, m.failure = "", ""
	current, hasCurrent := m.current()

	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.outcome = Outcome{Action: ActionQuit}
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}

	case "n":
		m.outcome = Outcome{Action: ActionCreate}
		return m, tea.Quit

	case "s":
		if !hasCurrent {
			break
		}
		if current.Running {
			m.failure = current.Name + " is already running"
			break
		}
		m.busy = "starting " + current.Name
		return m, m.runAction("started", current.Name, m.deps.Start)

	case "x":
		if !hasCurrent {
			break
		}
		if !current.Running {
			m.failure = current.Name + " is not running"
			break
		}
		m.busy = "stopping " + current.Name
		return m, m.runAction("stopped", current.Name, m.deps.Stop)

	case "r":
		if !hasCurrent {
			break
		}
		m.busy = "restarting " + current.Name
		return m, m.runAction("restarted", current.Name, m.deps.Restart)

	case "c", "enter":
		if !hasCurrent {
			break
		}
		if !current.Running {
			m.failure = current.Name + " is not running — press s to start it"
			break
		}
		m.outcome = Outcome{Action: ActionConsole, Server: current.Name}
		return m, tea.Quit

	case "d":
		if !hasCurrent {
			break
		}
		m.confirming = true
	}
	return m, nil
}

// handleConfirmKey asks which of the two remove modes the user means, then
// runs it directly — choosing a mode is itself the confirmation.
func (m dashboardModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	current, hasCurrent := m.current()
	switch msg.String() {
	case "u":
		if !hasCurrent {
			break
		}
		m.confirming = false
		m.busy = "removing " + current.Name
		return m, m.runRemove(current.Name, false)

	case "p":
		if !hasCurrent {
			break
		}
		m.confirming = false
		m.busy = "removing " + current.Name
		return m, m.runRemove(current.Name, true)

	case "esc", "ctrl+c":
		m.confirming = false
	}
	return m, nil
}

func (m dashboardModel) current() (ServerRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return ServerRow{}, false
	}
	return m.rows[m.cursor], true
}

func (m dashboardModel) View() string {
	var b strings.Builder
	b.WriteString("\n " + ui.Title.Render("svrctl") + "  " +
		ui.Subtle.Render(fmt.Sprintf("%d server%s", len(m.rows), plural(len(m.rows)))) + "\n\n")

	if len(m.rows) == 0 {
		b.WriteString(" " + ui.Body.Render("No servers yet.") + "\n")
		b.WriteString(" " + ui.Subtle.Render("Press ") + ui.Key.Render("n") +
			ui.Subtle.Render(" to set one up.") + "\n")
		b.WriteString("\n " + ui.HelpBar("n", "new server", "q", "quit") + "\n")
		return b.String()
	}

	b.WriteString(m.table())
	b.WriteString("\n")

	if m.confirming {
		b.WriteString(m.confirmView())
		return b.String()
	}

	switch {
	case m.busy != "":
		b.WriteString(" " + m.spinner.View() + ui.Subtle.Render(" "+m.busy) + "\n")
	case m.failure != "":
		b.WriteString(" " + ui.Failure.Render(ui.GlyphFail+" "+m.failure) + "\n")
	case m.status != "":
		b.WriteString(" " + ui.Success.Render(ui.GlyphOK+" "+m.status) + "\n")
	default:
		b.WriteString("\n")
	}

	b.WriteString("\n " + ui.HelpBar(
		"↑↓", "select",
		"s", "start",
		"x", "stop",
		"r", "restart",
		"c", "console",
		"d", "remove",
		"n", "new",
		"q", "quit",
	) + "\n")
	return b.String()
}

// confirmView asks whether to unregister or purge, so the dashboard never
// silently picks one on the caller's behalf.
func (m dashboardModel) confirmView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Warning.Render("Remove "+current.Name+"?") + "\n")
	b.WriteString(" " + ui.Subtle.Render("Files stay at "+current.Path+" unless you purge them.") + "\n")
	b.WriteString("\n " + ui.HelpBar(
		"u", "unregister, keep files",
		"p", "purge, delete files",
		"esc", "cancel",
	) + "\n")
	return b.String()
}

// table lays the servers out in fixed columns sized to the content, so the
// status column stays put as servers come up and down.
func (m dashboardModel) table() string {
	nameW, typeW, verW := 4, 4, 7
	for _, r := range m.rows {
		nameW = max(nameW, ui.TextWidth(r.Name))
		typeW = max(typeW, ui.TextWidth(r.Type))
		verW = max(verW, ui.TextWidth(r.Version))
	}

	var b strings.Builder
	b.WriteString("  " + ui.Header.Render(
		ui.Pad("NAME", nameW+2)+ui.Pad("TYPE", typeW+2)+
			ui.Pad("VERSION", verW+2)+ui.Pad("STATUS", 12)+"UPTIME") + "\n")

	for i, r := range m.rows {
		marker := "  "
		name := ui.Body.Render(ui.Pad(r.Name, nameW+2))
		if i == m.cursor {
			marker = ui.Selected.Render(ui.GlyphPrompt + " ")
			name = ui.Selected.Render(ui.Pad(r.Name, nameW+2))
		}

		uptime := ui.Subtle.Render("—")
		if r.Running {
			uptime = ui.Body.Render(ui.Duration(r.Uptime))
		}

		b.WriteString(marker + name +
			ui.Subtle.Render(ui.Pad(r.Type, typeW+2)) +
			ui.Subtle.Render(ui.Pad(r.Version, verW+2)) +
			ui.PadStyled(ui.StatusGlyph(r.Running), 12) +
			uptime + "\n")
	}
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
