// dashboard is what `svrctl` shows when run with no arguments.
//
// Previously that printed a wall of Cobra help, leaving a new user to work out
// which of ten subcommands to try first. The dashboard instead shows the thing
// they came to see — their servers and whether each is up — and puts the
// common actions on single keys.
package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	Memory  string
	Group   string
	Running bool
	PID     int
	Uptime  time.Duration
}

// BackupRow is one backup as the dashboard needs to display it.
type BackupRow struct {
	ID        string
	SizeBytes int64
	CreatedAt time.Time
}

// PluginRow is one installed plugin as the dashboard needs to display it.
type PluginRow struct {
	Slug          string
	VersionNumber string
}

// PluginHit is one Modrinth search result as the dashboard needs to display it.
type PluginHit struct {
	Slug        string
	Title       string
	Description string
}

// Action is what the user asked for as the dashboard closed. Actions that need
// the whole terminal cannot run inside the dashboard's own Bubble Tea program,
// so they are handed back to the caller to perform.
type Action int

const (
	ActionQuit Action = iota
	ActionConsole
	ActionCreate
	ActionLogs
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

	// Edit changes a server's memory and/or port. An empty memory clears it;
	// a zero port leaves the default in place.
	Edit func(name, memory string, port int) error

	// Properties returns a server's server.properties as a key->value map plus
	// the keys in file order.
	Properties func(name string) (map[string]string, []string, error)
	// SetProperty writes one server.properties key.
	SetProperty func(name, key, value string) error

	// Backups lists a server's backups, newest first.
	Backups func(name string) ([]BackupRow, error)
	// CreateBackup snapshots a server's world data.
	CreateBackup func(name string) error
	// RestoreBackup replaces a server's world data with the named backup
	// ("latest" or a specific ID).
	RestoreBackup func(name, id string) error

	// Plugins lists a server's installed plugins.
	Plugins func(name string) ([]PluginRow, error)
	// SearchPlugins searches Modrinth for plugins matching query.
	SearchPlugins func(query string) ([]PluginHit, error)
	// InstallPlugin installs a plugin (a Modrinth slug or project ID) onto a server.
	InstallPlugin func(name, project string) (PluginRow, error)
	// UpdatePlugin re-resolves slug's latest compatible build and installs it
	// if newer. changed is false when it was already up to date.
	UpdatePlugin func(name, slug string) (row PluginRow, changed bool, err error)
	// RemovePlugin removes an installed plugin.
	RemovePlugin func(name, slug string) error
}

// RunDashboard shows the dashboard and reports what to do next. focus, if
// non-empty, opens straight into that server's own dashboard instead of the
// main list — used to return to where the user was after an action (console,
// logs) that had to hand back the whole terminal.
func RunDashboard(deps DashboardDeps, focus string) (Outcome, error) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.Running

	final, err := tea.NewProgram(dashboardModel{
		deps:    deps,
		spinner: sp,
		width:   100,
		height:  24,
		focus:   focus,
	}, tea.WithAltScreen()).Run()
	if err != nil {
		return Outcome{}, err
	}
	m := final.(dashboardModel)
	return m.outcome, m.err
}

// mode selects which screen the dashboard is currently showing. modeList is
// the default; the rest are entered from it and always return to it or to
// the screen that opened them.
type mode int

const (
	modeList mode = iota
	// modeServer is a single server's own dashboard — everything but the
	// coarse lifecycle actions lives here instead of on the main list, so the
	// list stays scannable no matter how many per-server features exist.
	modeServer
	modeEdit
	modeProperties
	modePropertyEdit
	modeBackups
	modeConfirmRestore
	modePlugins
	modePluginSearch
	modePluginResults
)

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

	mode mode

	// edit sub-mode: memory/port text inputs, built fresh each time "e" is
	// pressed so they always start from the row currently selected.
	editMemory textinput.Model
	editPort   textinput.Model
	editOnPort bool // which field has focus
	editErr    string

	// properties sub-mode: a picker over the current keys, plus a text input
	// for editing whichever one is selected.
	propsPicker    picker
	propKey        string
	propValueInput textinput.Model

	// backups sub-mode: a picker over the server's snapshots.
	backupsPicker picker
	backupRows    []BackupRow
	restoreID     string

	// plugins sub-mode: a picker over the server's installed plugins, plus a
	// search step (text input, then a picker over the results) for installing
	// a new one.
	pluginsPicker picker
	pluginQuery   textinput.Model
	pluginResults []PluginHit
	resultsPicker picker

	width, height int
	outcome       Outcome
	err           error

	// focus is a server name to jump straight into modeServer for once the
	// first row list loads, then cleared — set when the dashboard is reopened
	// after an action (console, logs) that needed the whole terminal, so the
	// user lands back where they were instead of the main list.
	focus string
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
type propsLoadedMsg struct {
	name  string
	props map[string]string
	order []string
	err   error
}
type propSetMsg struct {
	name, key string
	err       error
}
type backupsLoadedMsg struct {
	name string
	rows []BackupRow
	err  error
}
type backupCreatedMsg struct {
	name string
	err  error
}
type pluginsLoadedMsg struct {
	name string
	rows []PluginRow
	err  error
}
type pluginSearchMsg struct {
	query string
	hits  []PluginHit
	err   error
}
type pluginInstalledMsg struct {
	name string
	err  error
}
type pluginUpdatedMsg struct {
	name, slug string
	changed    bool
	err        error
}
type pluginRemovedMsg struct {
	name, slug string
	err        error
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

func (m dashboardModel) loadProperties(name string) tea.Cmd {
	return func() tea.Msg {
		props, order, err := m.deps.Properties(name)
		return propsLoadedMsg{name: name, props: props, order: order, err: err}
	}
}

func (m dashboardModel) loadBackups(name string) tea.Cmd {
	return func() tea.Msg {
		rows, err := m.deps.Backups(name)
		return backupsLoadedMsg{name: name, rows: rows, err: err}
	}
}

func (m dashboardModel) loadPlugins(name string) tea.Cmd {
	return func() tea.Msg {
		rows, err := m.deps.Plugins(name)
		return pluginsLoadedMsg{name: name, rows: rows, err: err}
	}
}

func (m dashboardModel) searchPlugins(query string) tea.Cmd {
	return func() tea.Msg {
		hits, err := m.deps.SearchPlugins(query)
		return pluginSearchMsg{query: query, hits: hits, err: err}
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
		if m.focus != "" {
			for i, r := range m.rows {
				if r.Name == m.focus {
					m.cursor = i
					m.mode = modeServer
					break
				}
			}
			m.focus = ""
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

	case propsLoadedMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			m.mode = modeServer
			return m, nil
		}
		items := make([]Item, len(msg.order))
		for i, k := range msg.order {
			items[i] = Item{Title: k, Desc: msg.props[k]}
		}
		m.propsPicker = newPicker(items, 12, true)
		return m, nil

	case propSetMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			m.mode = modeServer
			return m, nil
		}
		m.status = "set " + msg.key
		return m, m.loadProperties(msg.name)

	case backupsLoadedMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			m.mode = modeServer
			return m, nil
		}
		m.backupRows = msg.rows
		items := make([]Item, len(msg.rows))
		for i, r := range msg.rows {
			items[i] = Item{Title: r.ID, Desc: ui.Bytes(r.SizeBytes) + "  " + r.CreatedAt.Format(time.RFC822)}
		}
		m.backupsPicker = newPicker(items, 10, false)
		return m, nil

	case backupCreatedMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			return m, nil
		}
		m.status = "backed up " + msg.name
		return m, m.loadBackups(msg.name)

	case pluginsLoadedMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			m.mode = modeServer
			return m, nil
		}
		items := make([]Item, len(msg.rows))
		for i, r := range msg.rows {
			items[i] = Item{Title: r.Slug, Desc: r.VersionNumber}
		}
		m.pluginsPicker = newPicker(items, 10, false)
		return m, nil

	case pluginSearchMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			m.mode = modePluginSearch
			return m, nil
		}
		m.pluginResults = msg.hits
		items := make([]Item, len(msg.hits))
		for i, h := range msg.hits {
			items[i] = Item{Title: h.Slug, Desc: ui.Truncate(h.Title+" — "+h.Description, 80)}
		}
		m.resultsPicker = newPicker(items, 10, false)
		m.mode = modePluginResults
		return m, nil

	case pluginInstalledMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			m.mode = modePlugins
			return m, m.loadPlugins(msg.name)
		}
		m.status = "installed plugin"
		m.mode = modePlugins
		return m, m.loadPlugins(msg.name)

	case pluginUpdatedMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			return m, m.loadPlugins(msg.name)
		}
		if msg.changed {
			m.status = msg.slug + " updated"
		} else {
			m.status = msg.slug + " is already up to date"
		}
		return m, m.loadPlugins(msg.name)

	case pluginRemovedMsg:
		m.busy = ""
		if msg.err != nil {
			m.failure = msg.err.Error()
			return m, m.loadPlugins(msg.name)
		}
		m.status = "removed " + msg.slug
		return m, m.loadPlugins(msg.name)

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
	m.status, m.failure = "", ""

	if m.confirming {
		return m.handleConfirmKey(msg)
	}

	switch m.mode {
	case modeServer:
		return m.handleServerKey(msg)
	case modeEdit:
		return m.handleEditKey(msg)
	case modeProperties:
		return m.handlePropertiesKey(msg)
	case modePropertyEdit:
		return m.handlePropertyEditKey(msg)
	case modeBackups:
		return m.handleBackupsKey(msg)
	case modeConfirmRestore:
		return m.handleConfirmRestoreKey(msg)
	case modePlugins:
		return m.handlePluginsKey(msg)
	case modePluginSearch:
		return m.handlePluginSearchKey(msg)
	case modePluginResults:
		return m.handlePluginResultsKey(msg)
	}

	current, hasCurrent := m.current()

	// modeList only carries the coarse lifecycle actions; everything specific
	// to one server — console, logs, edit, properties, backups, plugins —
	// lives behind "enter" on modeServer instead, so this list stays
	// scannable no matter how many per-server features exist.
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

	case "enter":
		if !hasCurrent {
			break
		}
		m.mode = modeServer

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

	case "d":
		if !hasCurrent {
			break
		}
		m.confirming = true
	}
	return m, nil
}

// handleServerKey drives a single server's own dashboard, opened by "enter"
// from the main list. Every per-server feature lives here rather than
// cluttering the list's help bar.
func (m dashboardModel) handleServerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	current, hasCurrent := m.current()
	if !hasCurrent {
		m.mode = modeList
		return m, nil
	}

	switch msg.String() {
	case "esc", "q":
		m.mode = modeList
		return m, nil

	case "s":
		if current.Running {
			m.failure = current.Name + " is already running"
			return m, nil
		}
		m.busy = "starting " + current.Name
		return m, m.runAction("started", current.Name, m.deps.Start)

	case "x":
		if !current.Running {
			m.failure = current.Name + " is not running"
			return m, nil
		}
		m.busy = "stopping " + current.Name
		return m, m.runAction("stopped", current.Name, m.deps.Stop)

	case "r":
		m.busy = "restarting " + current.Name
		return m, m.runAction("restarted", current.Name, m.deps.Restart)

	case "c":
		if !current.Running {
			m.failure = current.Name + " is not running — press s to start it"
			return m, nil
		}
		m.outcome = Outcome{Action: ActionConsole, Server: current.Name}
		return m, tea.Quit

	case "l":
		m.outcome = Outcome{Action: ActionLogs, Server: current.Name}
		return m, tea.Quit

	case "e":
		m.mode = modeEdit
		m.editErr = ""
		m.editOnPort = false
		m.editMemory = newDashboardInput("e.g. 4G (blank for default)")
		m.editMemory.SetValue(current.Memory)
		m.editMemory.CursorEnd()
		m.editMemory.Focus()
		m.editPort = newDashboardInput("e.g. 25565")
		if current.Port != 0 {
			m.editPort.SetValue(strconv.Itoa(current.Port))
		}
		m.editPort.CursorEnd()
		return m, textinput.Blink

	case "p":
		m.mode = modeProperties
		m.busy = "loading " + current.Name + "'s properties"
		return m, m.loadProperties(current.Name)

	case "b":
		m.mode = modeBackups
		m.busy = "loading " + current.Name + "'s backups"
		return m, m.loadBackups(current.Name)

	case "P":
		if current.Type != "paper" {
			m.failure = current.Name + " is a " + current.Type + " server — plugin management only works on paper servers"
			return m, nil
		}
		m.mode = modePlugins
		m.busy = "loading " + current.Name + "'s plugins"
		return m, m.loadPlugins(current.Name)
	}
	return m, nil
}

// newDashboardInput builds a textinput.Model styled like the create wizard's,
// so the dashboard's own text entry points look consistent with it.
func newDashboardInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ui.Running.Render(ui.GlyphPrompt + " ")
	ti.CharLimit = 64
	return ti
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

// handleEditKey drives the memory/port form opened by "e".
func (m dashboardModel) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeServer
		return m, nil

	case tea.KeyTab, tea.KeyShiftTab, tea.KeyDown, tea.KeyUp:
		m.editOnPort = !m.editOnPort
		if m.editOnPort {
			m.editMemory.Blur()
			m.editPort.Focus()
		} else {
			m.editPort.Blur()
			m.editMemory.Focus()
		}
		return m, textinput.Blink

	case tea.KeyEnter:
		if !m.editOnPort {
			m.editOnPort = true
			m.editMemory.Blur()
			m.editPort.Focus()
			return m, textinput.Blink
		}
		current, hasCurrent := m.current()
		if !hasCurrent {
			m.mode = modeServer
			return m, nil
		}
		port := 0
		if s := strings.TrimSpace(m.editPort.Value()); s != "" {
			p, err := strconv.Atoi(s)
			if err != nil {
				m.editErr = "port must be a number"
				return m, nil
			}
			port = p
		}
		memory := m.editMemory.Value()
		name := current.Name
		m.mode = modeServer
		m.busy = "updating " + name
		return m, func() tea.Msg {
			return actionDoneMsg{verb: "updated", name: name, err: m.deps.Edit(name, memory, port)}
		}
	}

	var cmd tea.Cmd
	if m.editOnPort {
		m.editPort, cmd = m.editPort.Update(msg)
	} else {
		m.editMemory, cmd = m.editMemory.Update(msg)
	}
	return m, cmd
}

// handlePropertiesKey drives the server.properties picker opened by "p".
func (m dashboardModel) handlePropertiesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.mode = modeServer
		return m, nil
	}
	if msg.Type == tea.KeyEnter {
		item, ok := m.propsPicker.selected()
		if !ok {
			return m, nil
		}
		m.mode = modePropertyEdit
		m.propKey = item.Title
		m.propValueInput = newDashboardInput("")
		m.propValueInput.SetValue(item.Desc)
		m.propValueInput.CursorEnd()
		m.propValueInput.Focus()
		return m, textinput.Blink
	}
	m.propsPicker.update(msg)
	return m, nil
}

// handlePropertyEditKey drives the text input for the property Enter opened.
func (m dashboardModel) handlePropertyEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeProperties
		return m, nil
	case tea.KeyEnter:
		current, hasCurrent := m.current()
		if !hasCurrent {
			m.mode = modeServer
			return m, nil
		}
		name, key, value := current.Name, m.propKey, m.propValueInput.Value()
		m.mode = modeProperties
		m.busy = "setting " + key
		return m, func() tea.Msg {
			return propSetMsg{name: name, key: key, err: m.deps.SetProperty(name, key, value)}
		}
	}
	var cmd tea.Cmd
	m.propValueInput, cmd = m.propValueInput.Update(msg)
	return m, cmd
}

// handleBackupsKey drives the backups picker opened by "b".
func (m dashboardModel) handleBackupsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	current, hasCurrent := m.current()
	switch msg.String() {
	case "esc":
		m.mode = modeServer
		return m, nil
	case "c":
		if !hasCurrent {
			return m, nil
		}
		name := current.Name
		m.busy = "backing up " + name
		return m, func() tea.Msg {
			return backupCreatedMsg{name: name, err: m.deps.CreateBackup(name)}
		}
	case "r", "enter":
		item, ok := m.backupsPicker.selected()
		if !ok {
			return m, nil
		}
		m.mode = modeConfirmRestore
		m.restoreID = item.Title
		return m, nil
	}
	m.backupsPicker.update(msg)
	return m, nil
}

// handleConfirmRestoreKey is the single-key confirmation before a restore
// overwrites the server's current world data, matching the same "choosing an
// option is the confirmation" pattern the remove flow uses.
func (m dashboardModel) handleConfirmRestoreKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	current, hasCurrent := m.current()
	switch msg.String() {
	case "y":
		if !hasCurrent {
			m.mode = modeBackups
			return m, nil
		}
		if current.Running {
			m.failure = current.Name + " is running — stop it first"
			m.mode = modeBackups
			return m, nil
		}
		name, id := current.Name, m.restoreID
		m.mode = modeServer
		m.busy = "restoring " + name
		return m, func() tea.Msg {
			return actionDoneMsg{verb: "restored", name: name, err: m.deps.RestoreBackup(name, id)}
		}
	case "esc", "n", "ctrl+c":
		m.mode = modeBackups
	}
	return m, nil
}

// handlePluginsKey drives the installed-plugins picker opened by "P".
func (m dashboardModel) handlePluginsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	current, hasCurrent := m.current()
	switch msg.String() {
	case "esc":
		m.mode = modeServer
		return m, nil
	case "i":
		if !hasCurrent {
			return m, nil
		}
		m.mode = modePluginSearch
		m.pluginQuery = newDashboardInput("e.g. luckperms")
		m.pluginQuery.Focus()
		return m, textinput.Blink
	case "u":
		item, ok := m.pluginsPicker.selected()
		if !ok || !hasCurrent {
			return m, nil
		}
		name, slug := current.Name, item.Title
		m.busy = "updating " + slug
		return m, func() tea.Msg {
			_, changed, err := m.deps.UpdatePlugin(name, slug)
			return pluginUpdatedMsg{name: name, slug: slug, changed: changed, err: err}
		}
	case "d":
		item, ok := m.pluginsPicker.selected()
		if !ok || !hasCurrent {
			return m, nil
		}
		name, slug := current.Name, item.Title
		m.busy = "removing " + slug
		return m, func() tea.Msg {
			return pluginRemovedMsg{name: name, slug: slug, err: m.deps.RemovePlugin(name, slug)}
		}
	}
	m.pluginsPicker.update(msg)
	return m, nil
}

// handlePluginSearchKey drives the search query input opened by "i".
func (m dashboardModel) handlePluginSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modePlugins
		return m, nil
	case tea.KeyEnter:
		query := strings.TrimSpace(m.pluginQuery.Value())
		if query == "" {
			return m, nil
		}
		m.busy = "searching modrinth for " + query
		return m, m.searchPlugins(query)
	}
	var cmd tea.Cmd
	m.pluginQuery, cmd = m.pluginQuery.Update(msg)
	return m, cmd
}

// handlePluginResultsKey drives the search-results picker opened once a
// search comes back, installing whichever hit is selected.
func (m dashboardModel) handlePluginResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	current, hasCurrent := m.current()
	switch msg.String() {
	case "esc":
		m.mode = modePluginSearch
		return m, nil
	case "enter":
		item, ok := m.resultsPicker.selected()
		if !ok || !hasCurrent {
			return m, nil
		}
		name, project := current.Name, item.Title
		m.busy = "installing " + project
		return m, func() tea.Msg {
			_, err := m.deps.InstallPlugin(name, project)
			return pluginInstalledMsg{name: name, err: err}
		}
	}
	m.resultsPicker.update(msg)
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

	if len(m.rows) == 0 && m.mode == modeList {
		b.WriteString(" " + ui.Body.Render("No servers yet.") + "\n")
		b.WriteString(" " + ui.Subtle.Render("Press ") + ui.Key.Render("n") +
			ui.Subtle.Render(" to set one up.") + "\n")
		b.WriteString("\n " + ui.HelpBar("n", "new server", "q", "quit") + "\n")
		return b.String()
	}

	if m.busy != "" {
		b.WriteString(" " + m.spinner.View() + ui.Subtle.Render(" "+m.busy) + "\n")
		return b.String()
	}

	switch m.mode {
	case modeServer:
		b.WriteString(m.serverView())
		return b.String()
	case modeEdit:
		b.WriteString(m.editView())
		return b.String()
	case modeProperties:
		b.WriteString(m.propertiesView())
		return b.String()
	case modePropertyEdit:
		b.WriteString(m.propertyEditView())
		return b.String()
	case modeBackups:
		b.WriteString(m.backupsView())
		return b.String()
	case modeConfirmRestore:
		b.WriteString(m.confirmRestoreView())
		return b.String()
	case modePlugins:
		b.WriteString(m.pluginsView())
		return b.String()
	case modePluginSearch:
		b.WriteString(m.pluginSearchView())
		return b.String()
	case modePluginResults:
		b.WriteString(m.pluginResultsView())
		return b.String()
	}

	b.WriteString(m.table())
	b.WriteString("\n")

	if m.confirming {
		b.WriteString(m.confirmView())
		return b.String()
	}

	switch {
	case m.failure != "":
		b.WriteString(" " + ui.Failure.Render(ui.GlyphFail+" "+m.failure) + "\n")
	case m.status != "":
		b.WriteString(" " + ui.Success.Render(ui.GlyphOK+" "+m.status) + "\n")
	default:
		b.WriteString("\n")
	}

	b.WriteString("\n " + ui.HelpBar(
		"↑↓", "select",
		"enter", "open",
		"s", "start",
		"x", "stop",
		"r", "restart",
		"d", "remove",
		"n", "new",
		"q", "quit",
	) + "\n")
	return b.String()
}

// serverView is one server's own dashboard, opened by "enter" from the main
// list — everything specific to that server lives behind a key here instead
// of crowding the list's help bar.
func (m dashboardModel) serverView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Title.Render(current.Name) + "  " + ui.StatusGlyph(current.Running) + "\n\n")

	b.WriteString(" " + ui.Subtle.Render("Type") + "     " + ui.Body.Render(current.Type) + "\n")
	b.WriteString(" " + ui.Subtle.Render("Version") + "  " + ui.Body.Render(current.Version) + "\n")
	if current.Port != 0 {
		b.WriteString(" " + ui.Subtle.Render("Port") + "     " + ui.Body.Render(strconv.Itoa(current.Port)) + "\n")
	}
	memory := current.Memory
	if memory == "" {
		memory = "default"
	}
	b.WriteString(" " + ui.Subtle.Render("Memory") + "   " + ui.Body.Render(memory) + "\n")
	if current.Group != "" {
		b.WriteString(" " + ui.Subtle.Render("Group") + "    " + ui.Body.Render(current.Group) + "\n")
	}
	if current.Running {
		b.WriteString(" " + ui.Subtle.Render("Uptime") + "   " + ui.Body.Render(ui.Duration(current.Uptime)) + "\n")
	}
	b.WriteString(" " + ui.Subtle.Render("Path") + "     " + ui.Subtle.Render(current.Path) + "\n")

	switch {
	case m.failure != "":
		b.WriteString("\n " + ui.Failure.Render(ui.GlyphFail+" "+m.failure) + "\n")
	case m.status != "":
		b.WriteString("\n " + ui.Success.Render(ui.GlyphOK+" "+m.status) + "\n")
	default:
		b.WriteString("\n")
	}

	helps := []string{
		"esc", "back",
		"s", "start",
		"x", "stop",
		"r", "restart",
		"c", "console",
		"l", "logs",
		"e", "edit",
		"p", "properties",
		"b", "backups",
	}
	if current.Type == "paper" {
		helps = append(helps, "P", "plugins")
	}
	b.WriteString("\n " + ui.HelpBar(helps...) + "\n")
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

// editView shows the memory/port form opened by "e".
func (m dashboardModel) editView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Body.Render("Edit "+current.Name) + "\n\n")
	b.WriteString(" " + ui.Subtle.Render("Memory") + "\n")
	b.WriteString(" " + m.editMemory.View() + "\n\n")
	b.WriteString(" " + ui.Subtle.Render("Port") + "\n")
	b.WriteString(" " + m.editPort.View() + "\n")
	if m.editErr != "" {
		b.WriteString("\n " + ui.Failure.Render(ui.GlyphFail+" "+m.editErr) + "\n")
	}
	b.WriteString("\n " + ui.HelpBar("tab", "switch field", "enter", "save", "esc", "cancel") + "\n")
	return b.String()
}

// propertiesView shows the server.properties picker opened by "p".
func (m dashboardModel) propertiesView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Body.Render(current.Name+"'s server.properties") + "\n\n")
	b.WriteString(m.propsPicker.view(m.width))
	b.WriteString("\n " + ui.HelpBar("↑↓", "select", "type", "filter", "enter", "edit value", "esc", "back") + "\n")
	return b.String()
}

// propertyEditView shows the text input for the property Enter opened.
func (m dashboardModel) propertyEditView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Body.Render(current.Name+" — "+m.propKey) + "\n\n")
	b.WriteString(" " + m.propValueInput.View() + "\n")
	b.WriteString("\n " + ui.HelpBar("enter", "save", "esc", "cancel") + "\n")
	return b.String()
}

// backupsView shows the backups picker opened by "b".
func (m dashboardModel) backupsView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Body.Render(current.Name+"'s backups") + "\n\n")
	if len(m.backupRows) == 0 {
		b.WriteString(" " + ui.Subtle.Render("No backups yet.") + "\n")
	} else {
		b.WriteString(m.backupsPicker.view(m.width))
	}
	b.WriteString("\n " + ui.HelpBar("↑↓", "select", "c", "create", "enter", "restore", "esc", "back") + "\n")
	return b.String()
}

// confirmRestoreView is the single-key confirmation before a restore
// overwrites the server's current world data.
func (m dashboardModel) confirmRestoreView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Warning.Render("Restore "+current.Name+" from "+m.restoreID+"?") + "\n")
	b.WriteString(" " + ui.Subtle.Render("Anything played since that backup was taken is gone.") + "\n")
	b.WriteString("\n " + ui.HelpBar("y", "restore", "esc", "cancel") + "\n")
	return b.String()
}

// pluginsView shows the installed-plugins picker opened by "P".
func (m dashboardModel) pluginsView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Body.Render(current.Name+"'s plugins") + "\n\n")
	if len(m.pluginsPicker.items) == 0 {
		b.WriteString(" " + ui.Subtle.Render("No plugins installed.") + "\n")
	} else {
		b.WriteString(m.pluginsPicker.view(m.width))
	}
	b.WriteString("\n " + ui.HelpBar("↑↓", "select", "i", "install", "u", "update", "d", "remove", "esc", "back") + "\n")
	return b.String()
}

// pluginSearchView shows the search query input opened by "i".
func (m dashboardModel) pluginSearchView() string {
	current, _ := m.current()
	var b strings.Builder
	b.WriteString(" " + ui.Body.Render("Search Modrinth for a plugin to install on "+current.Name) + "\n\n")
	b.WriteString(" " + m.pluginQuery.View() + "\n")
	b.WriteString("\n " + ui.HelpBar("enter", "search", "esc", "back") + "\n")
	return b.String()
}

// pluginResultsView shows the search-results picker opened once a search
// comes back.
func (m dashboardModel) pluginResultsView() string {
	var b strings.Builder
	b.WriteString(" " + ui.Body.Render("Results for “"+m.pluginQuery.Value()+"”") + "\n\n")
	if len(m.pluginResults) == 0 {
		b.WriteString(" " + ui.Subtle.Render("No plugins found.") + "\n")
	} else {
		b.WriteString(m.resultsPicker.view(m.width))
	}
	b.WriteString("\n " + ui.HelpBar("↑↓", "select", "enter", "install", "esc", "back") + "\n")
	return b.String()
}

// table lays the servers out in fixed columns sized to the content, so the
// status column stays put as servers come up and down.
func (m dashboardModel) table() string {
	nameW, typeW, verW, groupW := 4, 4, 7, 5
	showGroup := false
	for _, r := range m.rows {
		nameW = max(nameW, ui.TextWidth(r.Name))
		typeW = max(typeW, ui.TextWidth(r.Type))
		verW = max(verW, ui.TextWidth(r.Version))
		if r.Group != "" {
			showGroup = true
			groupW = max(groupW, ui.TextWidth(r.Group))
		}
	}

	var b strings.Builder
	header := ui.Pad("NAME", nameW+2) + ui.Pad("TYPE", typeW+2) + ui.Pad("VERSION", verW+2)
	if showGroup {
		header += ui.Pad("GROUP", groupW+2)
	}
	header += ui.Pad("STATUS", 12) + "UPTIME"
	b.WriteString("  " + ui.Header.Render(header) + "\n")

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

		line := marker + name +
			ui.Subtle.Render(ui.Pad(r.Type, typeW+2)) +
			ui.Subtle.Render(ui.Pad(r.Version, verW+2))
		if showGroup {
			line += ui.Subtle.Render(ui.Pad(r.Group, groupW+2))
		}
		line += ui.PadStyled(ui.StatusGlyph(r.Running), 12) + uptime
		b.WriteString(line + "\n")
	}
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
