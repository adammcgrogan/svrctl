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
	m := press(newTestDashboard(DashboardDeps{}), "c") // cursor starts on "creative", stopped

	if m.outcome.Action != ActionQuit {
		t.Errorf("should not have tried to attach to a stopped server")
	}
	if !strings.Contains(m.failure, "not running") {
		t.Errorf("expected an explanation, got %q", m.failure)
	}
}

func TestDashboardConsoleOnRunningServerRequestsAttach(t *testing.T) {
	m := press(newTestDashboard(DashboardDeps{}), "down", "c")

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

func TestDashboardRemovePurgeRequiresTypedName(t *testing.T) {
	var purged bool
	deps := DashboardDeps{
		List:   func() ([]ServerRow, error) { return testRows(), nil },
		Remove: func(name string, purge bool) error { purged = purge; return nil },
	}

	// Typing the wrong name refuses, with nothing sent to Remove.
	m := press(newTestDashboard(deps), "d", "p")
	if !m.purging {
		t.Fatal("expected the dashboard to be waiting on a typed name")
	}
	m = press(m, "n", "o", "p", "e")
	next, _ := m.Update(key("enter"))
	m = next.(dashboardModel)
	if m.purging {
		t.Error("purge state should have cleared after enter")
	}
	if !strings.Contains(m.failure, "did not match") {
		t.Errorf("got failure %q, want a name-mismatch message", m.failure)
	}
	if purged {
		t.Error("Remove should not have been called with a wrong name")
	}

	// Typing the right name runs the purge.
	m = press(newTestDashboard(deps), "d", "p")
	for _, r := range "creative" {
		next, _ := m.Update(key(string(r)))
		m = next.(dashboardModel)
	}
	next, cmd := m.Update(key("enter"))
	m = next.(dashboardModel)
	if cmd == nil {
		t.Fatal("expected a command to run the removal")
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)

	if !purged {
		t.Error("expected Remove to be called with purge=true")
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

	m = press(newTestDashboard(DashboardDeps{}), "d", "p")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(dashboardModel)
	if m.purging {
		t.Error("esc should have cancelled the purge step")
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
