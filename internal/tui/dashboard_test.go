// dashboard_test drives the dashboard model through Update/View directly,
// which exercises the same code path a terminal would without needing one.
package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

func testRows() []ServerRow {
	return []ServerRow{
		{Name: "creative", Type: "vanilla", Version: "1.21.1", Port: 25565},
		{Name: "survival", Type: "paper", Version: "1.21.4", Port: 25570,
			Running: true, PID: 4242, Uptime: 90 * time.Minute},
	}
}

// newTestDashboard returns a model already sized and populated, as it would be
// after its first render and refresh.
func newTestDashboard(deps DashboardDeps) dashboardModel {
	m := dashboardModel{deps: deps, width: 100, height: 24}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	next, _ = next.(dashboardModel).Update(rowsMsg{rows: testRows()})
	return next.(dashboardModel)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func press(m dashboardModel, keys ...string) dashboardModel {
	for _, k := range keys {
		next, _ := m.Update(key(k))
		m = next.(dashboardModel)
	}
	return m
}

func TestDashboardListsEveryServerWithStatus(t *testing.T) {
	view := ui.StripANSI(newTestDashboard(DashboardDeps{}).View())

	for _, want := range []string{"creative", "survival", "vanilla", "paper", "1.21.1", "2 servers"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, ui.GlyphRunning+" running") {
		t.Errorf("running server not marked as running:\n%s", view)
	}
	if !strings.Contains(view, ui.GlyphStopped+" stopped") {
		t.Errorf("stopped server not marked as stopped:\n%s", view)
	}
	if !strings.Contains(view, "1h 30m") {
		t.Errorf("uptime not shown:\n%s", view)
	}
}

func TestDashboardEmptyStatePointsAtCreate(t *testing.T) {
	m := dashboardModel{width: 100, height: 24}
	view := ui.StripANSI(m.View())

	if !strings.Contains(view, "No servers yet") {
		t.Errorf("expected an empty-state message:\n%s", view)
	}
	// An empty tool must say what to do next rather than showing a bare table.
	if !strings.Contains(view, "n") || !strings.Contains(view, "set one up") {
		t.Errorf("empty state does not suggest creating a server:\n%s", view)
	}
}

func TestDashboardConsoleOnStoppedServerExplainsWhyNot(t *testing.T) {
	// cursor starts on "creative", stopped; drill in with "enter" first.
	m := press(newTestDashboard(DashboardDeps{}), "enter", "c")

	if m.outcome.Action != ActionQuit {
		t.Errorf("should not have tried to attach to a stopped server")
	}
	if !strings.Contains(m.failure, "not running") {
		t.Errorf("expected an explanation, got %q", m.failure)
	}
}

func TestDashboardConsoleOnRunningServerRequestsAttach(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "down", "enter", "c")

	if m.outcome.Action != ActionConsole {
		t.Fatalf("got action %v, want ActionConsole", m.outcome.Action)
	}
	if m.outcome.Server != "survival" {
		t.Errorf("got server %q, want survival", m.outcome.Server)
	}
}

func TestDashboardStartRunsActionAndReportsIt(t *testing.T) {
	var started string
	deps := DashboardDeps{
		List:  func() ([]ServerRow, error) { return testRows(), nil },
		Start: func(name string) error { started = name; return nil },
	}

	m := newTestDashboard(deps)
	next, cmd := m.Update(key("s"))
	m = next.(dashboardModel)
	if m.busy == "" {
		t.Error("expected the dashboard to show that it is working")
	}
	if cmd == nil {
		t.Fatal("expected a command to run the start")
	}

	// Run the command the model returned, then feed its message back in.
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	if started != "creative" {
		t.Errorf("started %q, want creative", started)
	}
	if m.busy != "" {
		t.Error("busy indicator should clear once the action finishes")
	}
	if !strings.Contains(m.status, "started creative") {
		t.Errorf("got status %q, want it to mention the start", m.status)
	}
}

func TestDashboardSurfacesActionFailures(t *testing.T) {
	deps := DashboardDeps{
		List:  func() ([]ServerRow, error) { return testRows(), nil },
		Start: func(string) error { return errors.New("port already in use") },
	}

	m := newTestDashboard(deps)
	_, cmd := m.Update(key("s"))
	next, _ := m.Update(cmd())
	m = next.(dashboardModel)

	if !strings.Contains(m.failure, "port already in use") {
		t.Errorf("got failure %q, want the underlying error", m.failure)
	}
}

func TestDashboardIgnoresKeysWhileBusy(t *testing.T) {
	m := newTestDashboard(DashboardDeps{})
	m.busy = "starting creative"

	// Starting a second action, or quitting, while one is still in flight
	// would strand it, so keys are dropped until it reports back.
	after := press(m, "down", "s")
	if after.cursor != 0 {
		t.Errorf("cursor moved while an action was in flight")
	}
	if after.busy != "starting creative" {
		t.Errorf("in-flight action was replaced: got %q", after.busy)
	}
}

func TestDashboardCursorStaysInBounds(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "down", "down", "down")
	if m.cursor != 1 {
		t.Errorf("cursor ran past the last row: got %d, want 1", m.cursor)
	}
	m = press(m, "up", "up", "up")
	if m.cursor != 0 {
		t.Errorf("cursor ran past the first row: got %d, want 0", m.cursor)
	}
}

func TestDashboardEnterOpensServerDashboard(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "down", "enter")
	if m.mode != modeServer {
		t.Fatalf("expected modeServer, got %v", m.mode)
	}
	view := ui.StripANSI(m.View())
	for _, want := range []string{"survival", "paper", "1.21.4", "25570"} {
		if !strings.Contains(view, want) {
			t.Errorf("server view missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardFocusReopensServerDashboard(t *testing.T) {
	// After attachConsole/attachLogs hand the terminal back, RunDashboard is
	// called again with focus set to the server the user was just looking
	// at, so they land back on it instead of the main list.
	m := dashboardModel{deps: DashboardDeps{}, width: 100, height: 24, focus: "survival"}
	next, _ := m.Update(rowsMsg{rows: testRows()})
	m = next.(dashboardModel)

	if m.mode != modeServer {
		t.Fatalf("expected modeServer, got %v", m.mode)
	}
	current, ok := m.current()
	if !ok || current.Name != "survival" {
		t.Fatalf("expected cursor on survival, got %+v (ok=%v)", current, ok)
	}
	if m.focus != "" {
		t.Errorf("expected focus to be cleared after use, got %q", m.focus)
	}
}

func TestDashboardFocusOnUnknownServerStaysOnList(t *testing.T) {
	m := dashboardModel{deps: DashboardDeps{}, width: 100, height: 24, focus: "gone"}
	next, _ := m.Update(rowsMsg{rows: testRows()})
	m = next.(dashboardModel)

	if m.mode != modeList {
		t.Errorf("expected modeList when the focused server is gone, got %v", m.mode)
	}
}

func TestDashboardServerEscReturnsToList(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "enter", "esc")
	if m.mode != modeList {
		t.Errorf("expected esc to return to modeList, got %v", m.mode)
	}
}

func TestDashboardListMenuOmitsPerServerActions(t *testing.T) {
	// The whole point of splitting the dashboard in two is that the main list
	// only shows coarse lifecycle actions — console/logs/edit/properties/
	// backups/plugins live behind "enter" instead.
	view := ui.StripANSI(newTestDashboard(DashboardDeps{}).View())
	if !strings.Contains(view, "open") {
		t.Errorf("expected the list help bar to mention opening a server:\n%s", view)
	}
	for _, unwanted := range []string{"console", "logs", "properties", "backups", "plugins"} {
		if strings.Contains(view, unwanted) {
			t.Errorf("expected the list help bar not to mention %q:\n%s", unwanted, view)
		}
	}
}

func TestDashboardLogsWorkOnAStoppedServer(t *testing.T) {
	// Cursor starts on "creative", which is stopped — unlike console, logs
	// should not be blocked by that.
	m := press(newTestDashboard(DashboardDeps{}), "enter", "l")

	if m.outcome.Action != ActionLogs {
		t.Fatalf("got action %v, want ActionLogs", m.outcome.Action)
	}
	if m.outcome.Server != "creative" {
		t.Errorf("got server %q, want creative", m.outcome.Server)
	}
}

func TestDashboardLogsWorkOnARunningServer(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "down", "enter", "l")

	if m.outcome.Action != ActionLogs {
		t.Fatalf("got action %v, want ActionLogs", m.outcome.Action)
	}
	if m.outcome.Server != "survival" {
		t.Errorf("got server %q, want survival", m.outcome.Server)
	}
}

func TestDashboardRemoveOffersBothModes(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "d")

	if !m.confirming {
		t.Fatal("expected the dashboard to ask which remove mode is meant")
	}
	view := ui.StripANSI(m.View())
	for _, want := range []string{"unregister", "purge"} {
		if !strings.Contains(view, want) {
			t.Errorf("confirm view missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardRemoveUnregisterNeedsNoTypedName(t *testing.T) {
	var removed string
	var purged bool
	deps := DashboardDeps{
		List:   func() ([]ServerRow, error) { return testRows(), nil },
		Remove: func(name string, purge bool) error { removed, purged = name, purge; return nil },
	}

	m := press(newTestDashboard(deps), "d")
	next, cmd := m.Update(key("u"))
	m = next.(dashboardModel)
	if m.busy == "" {
		t.Fatal("expected the dashboard to show that it is working")
	}
	if m.confirming {
		t.Error("confirm state should have cleared once a mode was chosen")
	}
	if cmd == nil {
		t.Fatal("expected a command to run the removal")
	}

	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	if removed != "creative" || purged {
		t.Errorf("got removed=%q purged=%v, want creative/false", removed, purged)
	}
	if !strings.Contains(m.status, "removed creative") {
		t.Errorf("got status %q, want it to mention the removal", m.status)
	}
}

func TestDashboardRemovePurgeRunsOnSelection(t *testing.T) {
	var removed string
	var purged bool
	deps := DashboardDeps{
		List:   func() ([]ServerRow, error) { return testRows(), nil },
		Remove: func(name string, purge bool) error { removed, purged = name, purge; return nil },
	}

	m := press(newTestDashboard(deps), "d")
	next, cmd := m.Update(key("p"))
	m = next.(dashboardModel)
	if m.confirming {
		t.Error("confirm state should have cleared once a mode was chosen")
	}
	if cmd == nil {
		t.Fatal("expected a command to run the removal")
	}

	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	if removed != "creative" || !purged {
		t.Errorf("got removed=%q purged=%v, want creative/true", removed, purged)
	}
	if !strings.Contains(m.status, "removed creative") {
		t.Errorf("got status %q, want it to mention the removal", m.status)
	}
}

func TestDashboardRemoveConfirmCanBeCancelled(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "d")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(dashboardModel)
	if m.confirming {
		t.Error("esc should have cancelled the confirm step")
	}
}

func TestDashboardShrinksCursorWhenServersDisappear(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "down")
	next, _ := m.Update(rowsMsg{rows: testRows()[:1]})
	m = next.(dashboardModel)

	if m.cursor != 0 {
		t.Errorf("cursor left pointing past the end: got %d", m.cursor)
	}
	if view := m.View(); strings.Contains(view, "survival") {
		t.Errorf("removed server still shown:\n%s", view)
	}
}

func TestDashboardShowsGroupColumnWhenAnyServerHasOne(t *testing.T) {
	m := dashboardModel{width: 100, height: 24}
	next, _ := m.Update(rowsMsg{rows: []ServerRow{
		{Name: "lobby", Type: "paper", Version: "1.21.1", Group: "network"},
		{Name: "solo", Type: "vanilla", Version: "1.21.1"},
	}})
	m = next.(dashboardModel)

	view := ui.StripANSI(m.View())
	if !strings.Contains(view, "GROUP") || !strings.Contains(view, "network") {
		t.Errorf("expected a GROUP column showing \"network\":\n%s", view)
	}
}

func TestDashboardEditPrefillsCurrentMemoryAndPort(t *testing.T) {
	m := newTestDashboard(DashboardDeps{})
	m = press(m, "enter", "e") // cursor starts on "creative": no memory, port 25565

	if m.mode != modeEdit {
		t.Fatalf("expected modeEdit, got %v", m.mode)
	}
	if m.editMemory.Value() != "" {
		t.Errorf("got memory %q, want empty", m.editMemory.Value())
	}
	if m.editPort.Value() != "25565" {
		t.Errorf("got port %q, want 25565", m.editPort.Value())
	}
}

func TestDashboardEditSubmitsMemoryAndPort(t *testing.T) {
	var gotName, gotMemory string
	var gotPort int
	deps := DashboardDeps{
		List: func() ([]ServerRow, error) { return testRows(), nil },
		Edit: func(name, memory string, port int) error {
			gotName, gotMemory, gotPort = name, memory, port
			return nil
		},
	}

	m := press(newTestDashboard(deps), "enter", "e", "4G", "tab")
	next, cmd := m.Update(key("enter"))
	m = next.(dashboardModel)
	if m.mode != modeServer {
		t.Errorf("expected to return to modeServer on submit, got %v", m.mode)
	}
	if cmd == nil {
		t.Fatal("expected a command to run the edit")
	}

	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	if gotName != "creative" || gotMemory != "4G" || gotPort != 25565 {
		t.Errorf("got Edit(%q, %q, %d), want (creative, 4G, 25565)", gotName, gotMemory, gotPort)
	}
	if !strings.Contains(m.status, "updated creative") {
		t.Errorf("got status %q, want it to mention the update", m.status)
	}
}

func TestDashboardEditRejectsNonNumericPort(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "enter", "e", "tab",
		"backspace", "backspace", "backspace", "backspace", "backspace", "abc")
	m = press(m, "enter")

	if m.mode != modeEdit {
		t.Errorf("expected to stay in modeEdit after an invalid port, got %v", m.mode)
	}
	if m.editErr == "" {
		t.Error("expected an error message for a non-numeric port")
	}
}

func TestDashboardEditCancelReturnsToList(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "enter", "e", "esc")
	if m.mode != modeServer {
		t.Errorf("expected esc to return to modeServer, got %v", m.mode)
	}
}

func testProps() (map[string]string, []string) {
	return map[string]string{"motd": "Hello", "max-players": "20"}, []string{"motd", "max-players"}
}

func TestDashboardPropertiesListsThenEditsSelectedValue(t *testing.T) {
	var gotName, gotKey, gotValue string
	deps := DashboardDeps{
		List: func() ([]ServerRow, error) { return testRows(), nil },
		Properties: func(name string) (map[string]string, []string, error) {
			props, order := testProps()
			return props, order, nil
		},
		SetProperty: func(name, key, value string) error {
			gotName, gotKey, gotValue = name, key, value
			return nil
		},
	}

	m := press(newTestDashboard(deps), "enter")
	next, cmd := m.Update(key("p"))
	m = next.(dashboardModel)
	if m.mode != modeProperties {
		t.Fatalf("expected modeProperties, got %v", m.mode)
	}
	if cmd == nil {
		t.Fatal("expected a command to load properties")
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	view := ui.StripANSI(m.View())
	if !strings.Contains(view, "motd") {
		t.Errorf("expected the properties list to show motd:\n%s", view)
	}

	// Enter on the first entry ("motd") opens its value for editing.
	next, _ = m.Update(key("enter"))
	m = next.(dashboardModel)
	if m.mode != modePropertyEdit {
		t.Fatalf("expected modePropertyEdit, got %v", m.mode)
	}
	if m.propValueInput.Value() != "Hello" {
		t.Errorf("got prefilled value %q, want Hello", m.propValueInput.Value())
	}

	next, _ = m.Update(key("!"))
	m = next.(dashboardModel)
	next, cmd = m.Update(key("enter"))
	m = next.(dashboardModel)
	if m.mode != modeProperties {
		t.Errorf("expected to return to modeProperties on submit, got %v", m.mode)
	}
	if cmd == nil {
		t.Fatal("expected a command to save the property")
	}

	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	if gotName != "creative" || gotKey != "motd" || gotValue != "Hello!" {
		t.Errorf("got SetProperty(%q, %q, %q), want (creative, motd, Hello!)", gotName, gotKey, gotValue)
	}
	if !strings.Contains(m.status, "set motd") {
		t.Errorf("got status %q, want it to mention motd", m.status)
	}
}

func TestDashboardPropertiesEscReturnsToList(t *testing.T) {
	deps := DashboardDeps{
		List: func() ([]ServerRow, error) { return testRows(), nil },
		Properties: func(name string) (map[string]string, []string, error) {
			props, order := testProps()
			return props, order, nil
		},
	}
	next, cmd := press(newTestDashboard(deps), "enter").Update(key("p"))
	m := next.(dashboardModel)
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	next, _ = m.Update(key("esc"))
	m = next.(dashboardModel)
	if m.mode != modeServer {
		t.Errorf("expected esc to return to modeServer, got %v", m.mode)
	}
}

func testBackups() []BackupRow {
	return []BackupRow{{ID: "2026-01-02T00-00-00", SizeBytes: 1024, CreatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}
}

func TestDashboardBackupsListsAndCreates(t *testing.T) {
	var created string
	deps := DashboardDeps{
		List:         func() ([]ServerRow, error) { return testRows(), nil },
		Backups:      func(name string) ([]BackupRow, error) { return testBackups(), nil },
		CreateBackup: func(name string) error { created = name; return nil },
	}

	next, cmd := press(newTestDashboard(deps), "enter").Update(key("b"))
	m := next.(dashboardModel)
	if m.mode != modeBackups {
		t.Fatalf("expected modeBackups, got %v", m.mode)
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	view := ui.StripANSI(m.View())
	if !strings.Contains(view, "2026-01-02T00-00-00") {
		t.Errorf("expected the backup list to show the backup ID:\n%s", view)
	}

	next, cmd = m.Update(key("c"))
	m = next.(dashboardModel)
	if cmd == nil {
		t.Fatal("expected a command to create the backup")
	}
	m.Update(cmd())

	if created != "creative" {
		t.Errorf("got created %q, want creative", created)
	}
}

func TestDashboardBackupsRestoreRequiresConfirmation(t *testing.T) {
	var gotName, gotID string
	deps := DashboardDeps{
		List:    func() ([]ServerRow, error) { return testRows(), nil },
		Backups: func(name string) ([]BackupRow, error) { return testBackups(), nil },
		RestoreBackup: func(name, id string) error {
			gotName, gotID = name, id
			return nil
		},
	}

	next, cmd := press(newTestDashboard(deps), "enter").Update(key("b"))
	m := next.(dashboardModel)
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	// Enter on the selected backup asks for confirmation rather than
	// restoring immediately.
	next, _ = m.Update(key("enter"))
	m = next.(dashboardModel)
	if m.mode != modeConfirmRestore {
		t.Fatalf("expected modeConfirmRestore, got %v", m.mode)
	}

	// esc backs out without restoring.
	next, _ = m.Update(key("esc"))
	m = next.(dashboardModel)
	if m.mode != modeBackups {
		t.Errorf("expected esc to cancel back to modeBackups, got %v", m.mode)
	}

	// Ask again and confirm with "y".
	next, _ = m.Update(key("enter"))
	m = next.(dashboardModel)
	next, cmd = m.Update(key("y"))
	m = next.(dashboardModel)
	if m.mode != modeServer {
		t.Errorf("expected to return to modeServer after confirming, got %v", m.mode)
	}
	if cmd == nil {
		t.Fatal("expected a command to run the restore")
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	if gotName != "creative" || gotID != "2026-01-02T00-00-00" {
		t.Errorf("got RestoreBackup(%q, %q), want (creative, 2026-01-02T00-00-00)", gotName, gotID)
	}
	if !strings.Contains(m.status, "restored creative") {
		t.Errorf("got status %q, want it to mention the restore", m.status)
	}
}

func TestDashboardPluginsRejectVanillaServer(t *testing.T) {
	// Cursor starts on "creative", a vanilla server — plugins don't apply.
	m := press(newTestDashboard(DashboardDeps{}), "enter", "P")
	if m.mode != modeServer {
		t.Fatalf("expected to stay in modeServer, got %v", m.mode)
	}
	if !strings.Contains(m.failure, "vanilla") {
		t.Errorf("got failure %q, want it to explain plugins need paper", m.failure)
	}
}

func TestDashboardPluginsListsSearchesAndInstalls(t *testing.T) {
	var installedName, installedProject string
	deps := DashboardDeps{
		List: func() ([]ServerRow, error) { return testRows(), nil },
		Plugins: func(name string) ([]PluginRow, error) {
			return []PluginRow{{Slug: "luckperms", VersionNumber: "5.4"}}, nil
		},
		SearchPlugins: func(query string) ([]PluginHit, error) {
			return []PluginHit{{Slug: "essentialsx", Title: "EssentialsX", Description: "The essentials suite"}}, nil
		},
		InstallPlugin: func(name, project string) (PluginRow, error) {
			installedName, installedProject = name, project
			return PluginRow{Slug: project, VersionNumber: "2.21"}, nil
		},
	}

	// "survival" (paper) is the second row.
	next, cmd := press(newTestDashboard(deps), "down", "enter").Update(key("P"))
	m := next.(dashboardModel)
	if m.mode != modePlugins {
		t.Fatalf("expected modePlugins, got %v", m.mode)
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	view := ui.StripANSI(m.View())
	if !strings.Contains(view, "luckperms") {
		t.Errorf("expected the installed plugin to show up:\n%s", view)
	}

	next, _ = m.Update(key("i"))
	m = next.(dashboardModel)
	if m.mode != modePluginSearch {
		t.Fatalf("expected modePluginSearch, got %v", m.mode)
	}

	next, _ = m.Update(key("essentialsx"))
	m = next.(dashboardModel)
	next, cmd = m.Update(key("enter"))
	m = next.(dashboardModel)
	if cmd == nil {
		t.Fatal("expected a command to run the search")
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)
	if m.mode != modePluginResults {
		t.Fatalf("expected modePluginResults, got %v", m.mode)
	}
	if view := ui.StripANSI(m.View()); !strings.Contains(view, "essentialsx") {
		t.Errorf("expected the search hit to show up:\n%s", view)
	}

	next, cmd = m.Update(key("enter"))
	m = next.(dashboardModel)
	if cmd == nil {
		t.Fatal("expected a command to install the selected hit")
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	if installedName != "survival" || installedProject != "essentialsx" {
		t.Errorf("got InstallPlugin(%q, %q), want (survival, essentialsx)", installedName, installedProject)
	}
	if m.mode != modePlugins {
		t.Errorf("expected to return to modePlugins after installing, got %v", m.mode)
	}
}

func TestDashboardPluginsUpdateAndRemove(t *testing.T) {
	var updatedSlug, removedSlug string
	deps := DashboardDeps{
		List: func() ([]ServerRow, error) { return testRows(), nil },
		Plugins: func(name string) ([]PluginRow, error) {
			return []PluginRow{{Slug: "luckperms", VersionNumber: "5.4"}}, nil
		},
		UpdatePlugin: func(name, slug string) (PluginRow, bool, error) {
			updatedSlug = slug
			return PluginRow{Slug: slug, VersionNumber: "5.5"}, true, nil
		},
		RemovePlugin: func(name, slug string) error {
			removedSlug = slug
			return nil
		},
	}

	next, cmd := press(newTestDashboard(deps), "down", "enter").Update(key("P"))
	m := next.(dashboardModel)
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	next, cmd = m.Update(key("u"))
	m = next.(dashboardModel)
	if cmd == nil {
		t.Fatal("expected a command to update the selected plugin")
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)
	if updatedSlug != "luckperms" {
		t.Errorf("got UpdatePlugin slug %q, want luckperms", updatedSlug)
	}
	if !strings.Contains(m.status, "luckperms") {
		t.Errorf("got status %q, want it to mention luckperms", m.status)
	}

	next, cmd = m.Update(key("d"))
	m = next.(dashboardModel)
	if cmd == nil {
		t.Fatal("expected a command to remove the selected plugin")
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)
	if removedSlug != "luckperms" {
		t.Errorf("got RemovePlugin slug %q, want luckperms", removedSlug)
	}
}
