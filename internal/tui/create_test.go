// create_test walks the wizard model the way a user would, checking that the
// values it collects match the keys that were pressed.
package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

func testCreateDeps() CreateDeps {
	return CreateDeps{
		Kinds: []Item{
			{Title: "vanilla", Desc: "Official Mojang server"},
			{Title: "paper", Desc: "PaperMC"},
		},
		ListVersions: func(string) ([]string, error) {
			return []string{"1.21.4", "1.21.1", "1.20.6"}, nil
		},
		ValidateName: func(name string) error {
			if name == "taken" {
				return errors.New("a server named \"taken\" already exists")
			}
			if name == "" {
				return errors.New("name cannot be empty")
			}
			return nil
		},
		DefaultPath: func(name string) string { return "/servers/" + name },
		EULAURL:     "https://example.invalid/eula",
		Provision:   func(CreateValues, func(string), func(int64, int64)) error { return nil },
	}
}

func newTestWizard(deps CreateDeps) createModel {
	m := newCreateModel(deps, CreateValues{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	return next.(createModel)
}

// typeText feeds each rune of s to the model as a key press.
func typeText(m createModel, s string) createModel {
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(createModel)
	}
	return m
}

func send(m createModel, msgs ...tea.Msg) createModel {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(createModel)
	}
	return m
}

// sendRunning is send, but it also runs whatever command the model returns and
// feeds the resulting message back in — the job Bubble Tea's runtime does.
func sendRunning(m createModel, msg tea.Msg) createModel {
	next, cmd := m.Update(msg)
	m = next.(createModel)
	return runCmd(m, cmd)
}

// runCmd executes a command and applies its message, unwrapping batches the
// way the real event loop would.
func runCmd(m createModel, cmd tea.Cmd) createModel {
	if cmd == nil {
		return m
	}
	switch out := cmd().(type) {
	case nil:
		return m
	case tea.BatchMsg:
		for _, inner := range out {
			m = runCmd(m, inner)
		}
		return m
	default:
		next, _ := m.Update(out)
		return next.(createModel)
	}
}

var (
	enter    = tea.KeyMsg{Type: tea.KeyEnter}
	down     = tea.KeyMsg{Type: tea.KeyDown}
	shiftTab = tea.KeyMsg{Type: tea.KeyShiftTab}
)

func TestWizardCollectsEveryAnswer(t *testing.T) {
	deps := testCreateDeps()
	var provisioned CreateValues
	deps.Provision = func(v CreateValues, _ func(string), _ func(int64, int64)) error {
		provisioned = v
		return nil
	}

	m := newTestWizard(deps)
	m = typeText(m, "survival")
	m = send(m, enter)       // name
	m = send(m, down, enter) // type: second entry, paper
	m = send(m, versionsMsg{versions: []string{"1.21.4", "1.21.1"}})
	m = send(m, down, enter)       // version: second entry, 1.21.1
	m = send(m, down, down, enter) // memory: third entry, 4G
	m = typeText(m, "25570")
	m = send(m, enter) // port

	if m.step != stepReview {
		t.Fatalf("expected the review step, got %v", m.step)
	}
	review := ui.StripANSI(m.View())
	for _, want := range []string{"survival", "paper", "1.21.1", "4G", "25570", "/servers/survival"} {
		if !strings.Contains(review, want) {
			t.Errorf("review missing %q:\n%s", want, review)
		}
	}
	// The EULA is settled before any download starts, not after.
	if !strings.Contains(review, deps.EULAURL) {
		t.Errorf("review does not mention the EULA:\n%s", review)
	}

	// Confirming kicks off the real work; run the command the model hands back
	// so the fake Provision actually sees the collected values.
	m = sendRunning(m, enter)

	if !m.created {
		t.Error("wizard did not report the server as created")
	}
	want := CreateValues{Name: "survival", Kind: "paper", Version: "1.21.1", Memory: "4G", Port: 25570}
	if provisioned != want {
		t.Errorf("provisioned %+v, want %+v", provisioned, want)
	}
}

func TestWizardRejectsTakenNameBeforeAnyWork(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m = typeText(m, "taken")
	m = send(m, enter)

	if m.step != stepName {
		t.Errorf("wizard advanced past a name it cannot use")
	}
	if !strings.Contains(m.warning, "already exists") {
		t.Errorf("got warning %q, want it to explain the clash", m.warning)
	}
}

func TestWizardRejectsBadPort(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m.step = stepPort
	m = typeText(m, "99999")
	m = send(m, enter)

	if m.step != stepPort {
		t.Error("wizard accepted an out-of-range port")
	}
	if !strings.Contains(m.warning, "between 1 and 65535") {
		t.Errorf("got warning %q, want the valid range", m.warning)
	}
}

func TestWizardBlankPortMeansMinecraftDefault(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m.step = stepPort
	m = send(m, enter)

	if m.step != stepReview {
		t.Fatalf("blank port should be accepted, got step %v", m.step)
	}
	if m.values.Port != 0 {
		t.Errorf("got port %d, want 0 (meaning leave it to Minecraft)", m.values.Port)
	}
	if view := ui.StripANSI(m.View()); !strings.Contains(view, "25565") {
		t.Errorf("review should spell out the default port:\n%s", view)
	}
}

func TestWizardShiftTabGoesBack(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m = typeText(m, "survival")
	m = send(m, enter, shiftTab)

	if m.step != stepName {
		t.Errorf("shift+tab did not return to the previous question, got %v", m.step)
	}
}

func TestWizardMarksTheNewestVersion(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m.step = stepVersion
	m = send(m, versionsMsg{versions: []string{"1.21.4", "1.21.1"}})

	view := ui.StripANSI(m.View())
	if !strings.Contains(view, "latest") {
		t.Errorf("newest version is not flagged:\n%s", view)
	}
	if got := m.verPick.items[0]; got.Badge != "latest" || got.Title != "1.21.4" {
		t.Errorf("expected 1.21.4 flagged as latest, got %+v", got)
	}
}

func TestWizardFiltersVersions(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m.step = stepVersion
	m = send(m, versionsMsg{versions: []string{"1.21.4", "1.21.1", "1.20.6"}})
	m = typeText(m, "1.20")

	if got := len(m.verPick.filtered); got != 1 {
		t.Fatalf("filter matched %d versions, want 1", got)
	}
	sel, _ := m.verPick.selected()
	if sel.Title != "1.20.6" {
		t.Errorf("got %q, want 1.20.6", sel.Title)
	}
}

func TestWizardReportsVersionFetchFailure(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m.step = stepVersion
	m = send(m, versionsMsg{err: errors.New("no network")})

	if m.step != stepKind {
		t.Errorf("should return to the type question when versions cannot load")
	}
	if !strings.Contains(m.warning, "no network") {
		t.Errorf("got warning %q, want the underlying cause", m.warning)
	}
}

func TestWizardIgnoresKeysWhileProvisioning(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m.step = stepWorking

	// Quitting mid-download is what used to leave half-built directories on
	// disk, so the wizard refuses to act on keys until provisioning returns.
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc}, tea.KeyMsg{Type: tea.KeyCtrlC})
	if m.step != stepWorking {
		t.Errorf("provisioning was interrupted by a key press")
	}
}

func TestWizardShowsDownloadProgress(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m.step = stepWorking
	m = send(m, phaseMsg("Downloading server.jar"), progressMsg{done: 5 << 20, total: 50 << 20})

	view := ui.StripANSI(m.View())
	for _, want := range []string{"Downloading server.jar", "5.0 MB", "50.0 MB"} {
		if !strings.Contains(view, want) {
			t.Errorf("progress view missing %q:\n%s", want, view)
		}
	}
}

func TestWizardPropagatesProvisionFailure(t *testing.T) {
	m := newTestWizard(testCreateDeps())
	m.step = stepWorking
	m = send(m, finishedMsg{err: errors.New("disk full")})

	if m.created {
		t.Error("a failed provision must not report success")
	}
	if m.err == nil || !strings.Contains(m.err.Error(), "disk full") {
		t.Errorf("got err %v, want it to carry the cause", m.err)
	}
}

func TestWizardSkipsQuestionsAlreadyAnsweredByFlag(t *testing.T) {
	deps := testCreateDeps()

	// Re-asking for a name the user just typed on the command line makes the
	// wizard feel like an obstacle instead of a shortcut.
	m := newCreateModel(deps, CreateValues{Name: "survival"})
	if m.step != stepKind {
		t.Errorf("with a name supplied, got step %v, want stepKind", m.step)
	}

	m = newCreateModel(deps, CreateValues{Name: "survival", Kind: "paper"})
	if m.step != stepVersion {
		t.Errorf("with a type supplied, got step %v, want stepVersion", m.step)
	}
	if !m.versionsLoading {
		t.Error("opening on the version question should start fetching versions")
	}

	m = newCreateModel(deps, CreateValues{
		Name: "survival", Kind: "paper", Version: "1.21.1", Memory: "4G", Port: 25570,
	})
	if m.step != stepReview {
		t.Errorf("with everything supplied, got step %v, want stepReview", m.step)
	}
}

func TestWizardStillAsksForAnUnusablePrefilledName(t *testing.T) {
	m := newCreateModel(testCreateDeps(), CreateValues{Name: "taken", Kind: "paper"})

	if m.step != stepName {
		t.Errorf("got step %v, want stepName so the clash can be resolved", m.step)
	}
}

func TestWizardShowsAPrefilledTypeAsSelected(t *testing.T) {
	m := newCreateModel(testCreateDeps(), CreateValues{Name: "survival", Kind: "paper"})
	m.step = stepKind

	sel, ok := m.kindPick.selected()
	if !ok || sel.Title != "paper" {
		t.Errorf("got %+v (ok=%v), want paper preselected", sel, ok)
	}
}
