// create is the interactive new-server wizard.
//
// Creating a server used to mean knowing, up front and exactly, a server type
// and a Minecraft version string — with no way to discover either, and a
// network round trip before a typo was reported. The wizard asks one question
// at a time, offers the real published versions to choose from, and shows what
// the long downloads are actually doing.
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

// CreateValues is what the wizard collects.
type CreateValues struct {
	Name    string
	Kind    string
	Version string
	Memory  string
	Port    int
	Path    string
}

// CreateDeps are the callbacks the wizard needs from its caller. They are
// injected rather than imported so this package stays independent of the
// command layer and can be driven by a fake in tests.
type CreateDeps struct {
	// Kinds lists the offerable server flavors as picker items.
	Kinds []Item
	// ListVersions returns published versions for a flavor, newest first.
	ListVersions func(kind string) ([]string, error)
	// ValidateName reports why a name cannot be used, if it cannot.
	ValidateName func(name string) error
	// DefaultPath gives the directory a named server would live in.
	DefaultPath func(name string) string
	// Provision does the real work, calling phase and progress as it goes.
	Provision func(v CreateValues, phase func(string), progress func(done, total int64)) error
	// EULAURL is shown at the acceptance step.
	EULAURL string
}

type step int

const (
	stepName step = iota
	stepKind
	stepVersion
	stepMemory
	stepPort
	stepReview
	stepWorking
	stepDone
)

// memoryChoices are the heap sizes worth offering. "Default" leaves the JVM to
// decide, which is right for a server nobody has measured yet.
var memoryChoices = []Item{
	{Title: "Default", Desc: "let the JVM choose"},
	{Title: "2G", Desc: "a few players, no mods"},
	{Title: "4G", Desc: "comfortable for most survival servers"},
	{Title: "6G", Desc: "many players or plugins"},
	{Title: "8G", Desc: "large modded or busy servers"},
}

// RunCreate runs the wizard and reports whether a server was created, along
// with the values used.
func RunCreate(deps CreateDeps, prefill CreateValues) (CreateValues, bool, error) {
	final, err := tea.NewProgram(newCreateModel(deps, prefill)).Run()
	if err != nil {
		return CreateValues{}, false, err
	}
	m := final.(createModel)
	if m.err != nil {
		return CreateValues{}, false, m.err
	}
	return m.values, m.created, nil
}

// newCreateModel builds the wizard's initial state. Kept separate from
// RunCreate so tests can drive the model without a terminal.
func newCreateModel(deps CreateDeps, prefill CreateValues) createModel {
	name := textinput.New()
	name.Placeholder = "survival"
	name.Prompt = ui.Running.Render(ui.GlyphPrompt + " ")
	name.CharLimit = 64
	name.SetValue(prefill.Name)
	name.CursorEnd()
	name.Focus()

	port := textinput.New()
	port.Placeholder = "25565"
	port.Prompt = ui.Running.Render(ui.GlyphPrompt + " ")
	port.CharLimit = 5
	if prefill.Port != 0 {
		port.SetValue(strconv.Itoa(prefill.Port))
	}
	// Focus both inputs up front. Only one is ever rendered at a time, and an
	// unfocused textinput silently swallows every keystroke — which is a very
	// confusing way for a question to appear broken.
	port.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.Running

	m := createModel{
		deps:      deps,
		values:    prefill,
		step:      stepName,
		nameInput: name,
		portInput: port,
		spinner:   sp,
		bar:       progress.New(progress.WithSolidFill("#5BBF63"), progress.WithoutPercentage()),
		kindPick:  newPicker(deps.Kinds, len(deps.Kinds), false),
		memPick:   newPicker(memoryChoices, len(memoryChoices), false),
		verPick:   newPicker(nil, 10, true),
		events:    make(chan tea.Msg, 64),
		width:     80,
	}
	m.versionsLoading = prefill.Kind != "" && prefill.Version == ""

	m.step = firstUnansweredStep(deps, prefill)
	// Preselect anything given by flag. Steps are skipped only up to the first
	// unanswered one, so a later question may still be asked — it should at
	// least open on the value the user already chose rather than the default.
	if m.values.Kind != "" {
		m.kindPick.selectTitle(m.values.Kind)
	}
	if m.values.Memory != "" {
		m.memPick.selectTitle(m.values.Memory)
	}
	return m
}

// firstUnansweredStep skips past everything the caller already supplied on the
// command line. Asking someone to re-enter a name they just typed is the kind
// of thing that makes a wizard feel like an obstacle rather than a shortcut.
func firstUnansweredStep(deps CreateDeps, v CreateValues) step {
	return nextUnansweredStep(deps, v, stepName)
}

// nextUnansweredStep walks the steps starting at from and returns the first
// one not already satisfied by v, so a flag-supplied answer is skipped no
// matter where in the sequence it falls — not just when every step before it
// also happens to be answered.
func nextUnansweredStep(deps CreateDeps, v CreateValues, from step) step {
	for s := from; s <= stepPort; s++ {
		switch s {
		case stepName:
			if v.Name == "" || deps.ValidateName(v.Name) != nil {
				return stepName
			}
		case stepKind:
			if v.Kind == "" {
				return stepKind
			}
		case stepVersion:
			if v.Version == "" {
				return stepVersion
			}
		case stepMemory:
			if v.Memory == "" {
				return stepMemory
			}
		case stepPort:
			if v.Port == 0 {
				return stepPort
			}
		}
	}
	return stepReview
}

type createModel struct {
	deps   CreateDeps
	values CreateValues
	step   step

	nameInput textinput.Model
	portInput textinput.Model
	kindPick  picker
	memPick   picker
	verPick   picker

	spinner spinner.Model
	bar     progress.Model

	versionsLoading bool
	phase           string
	done, total     int64

	events  chan tea.Msg
	width   int
	warning string
	err     error
	created bool
}

// Messages flowing back from the loading and provisioning goroutines.
type versionsMsg struct {
	kind     string
	versions []string
	err      error
}
type phaseMsg string
type progressMsg struct{ done, total int64 }
type finishedMsg struct{ err error }

func (m createModel) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink, m.spinner.Tick}
	// When --type was supplied, the wizard opens straight on the version
	// question, so the fetch its picker needs has to start here rather than on
	// the keystroke that would normally have got us there.
	if m.step == stepVersion {
		cmds = append(cmds, m.loadVersions(m.values.Kind))
	}
	return tea.Batch(cmds...)
}

// waitForEvent drains one message from the provisioning goroutine.
func (m createModel) waitForEvent() tea.Cmd {
	return func() tea.Msg { return <-m.events }
}

// loadVersions fetches the version list for a flavor off the UI goroutine.
func (m createModel) loadVersions(kind string) tea.Cmd {
	return func() tea.Msg {
		versions, err := m.deps.ListVersions(kind)
		return versionsMsg{kind: kind, versions: versions, err: err}
	}
}

// startProvision runs the real creation, funnelling its progress into the
// event channel. Progress updates are dropped rather than blocking if the UI
// falls behind, since a stale frame matters less than a stalled download.
func (m createModel) startProvision() tea.Cmd {
	values := m.values
	events := m.events
	provision := m.deps.Provision
	return func() tea.Msg {
		go func() {
			err := provision(values,
				func(label string) { events <- phaseMsg(label) },
				func(done, total int64) {
					select {
					case events <- progressMsg{done, total}:
					default:
					}
				},
			)
			events <- finishedMsg{err: err}
		}()
		return <-events
	}
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.bar.Width = min(max(msg.Width-20, 20), 60)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case versionsMsg:
		m.versionsLoading = false
		if msg.err != nil {
			m.warning = "could not fetch versions: " + msg.err.Error()
			m.step = stepKind
			return m, nil
		}
		m.verPick.setItems(versionItems(msg.versions))
		return m, nil

	case phaseMsg:
		m.phase = string(msg)
		m.done, m.total = 0, 0
		return m, tea.Batch(m.waitForEvent(), m.bar.SetPercent(0))

	case progressMsg:
		m.done, m.total = msg.done, msg.total
		cmds := []tea.Cmd{m.waitForEvent()}
		if msg.total > 0 {
			cmds = append(cmds, m.bar.SetPercent(float64(msg.done)/float64(msg.total)))
		}
		return m, tea.Batch(cmds...)

	case progress.FrameMsg:
		bar, cmd := m.bar.Update(msg)
		m.bar = bar.(progress.Model)
		return m, cmd

	case finishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.created = true
		m.step = stepDone
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m createModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Provisioning is not interruptible: quitting mid-download would leave a
	// half-written directory behind, which is exactly what this rewrite fixes.
	if m.step == stepWorking {
		return m, nil
	}

	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyShiftTab:
		m.warning = ""
		m.step = m.previousStep()
		return m, nil
	}

	m.warning = ""

	switch m.step {
	case stepName:
		if msg.Type == tea.KeyEnter {
			name := strings.TrimSpace(m.nameInput.Value())
			if err := m.deps.ValidateName(name); err != nil {
				m.warning = err.Error()
				return m, nil
			}
			m.values.Name = name
			m.step = nextUnansweredStep(m.deps, m.values, stepKind)
			return m, nil
		}
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd

	case stepKind:
		if m.kindPick.update(msg) {
			return m, nil
		}
		if msg.Type == tea.KeyEnter {
			it, ok := m.kindPick.selected()
			if !ok {
				return m, nil
			}
			m.values.Kind = it.Title
			next := nextUnansweredStep(m.deps, m.values, stepVersion)
			m.step = next
			if next != stepVersion {
				return m, nil
			}
			m.versionsLoading = true
			m.verPick.setItems(nil)
			return m, m.loadVersions(it.Title)
		}
		return m, nil

	case stepVersion:
		if m.versionsLoading {
			return m, nil
		}
		if m.verPick.update(msg) {
			return m, nil
		}
		if msg.Type == tea.KeyEnter {
			it, ok := m.verPick.selected()
			if !ok {
				return m, nil
			}
			m.values.Version = it.Title
			m.step = nextUnansweredStep(m.deps, m.values, stepMemory)
			return m, nil
		}
		return m, nil

	case stepMemory:
		if m.memPick.update(msg) {
			return m, nil
		}
		if msg.Type == tea.KeyEnter {
			it, ok := m.memPick.selected()
			if !ok {
				return m, nil
			}
			if it.Title == "Default" {
				m.values.Memory = ""
			} else {
				m.values.Memory = it.Title
			}
			next := nextUnansweredStep(m.deps, m.values, stepPort)
			m.step = next
			if next != stepPort {
				return m, nil
			}
			m.portInput.Focus()
			return m, textinput.Blink
		}
		return m, nil

	case stepPort:
		if msg.Type == tea.KeyEnter {
			text := strings.TrimSpace(m.portInput.Value())
			if text == "" {
				m.values.Port = 0 // Minecraft's own default, 25565
			} else {
				n, err := strconv.Atoi(text)
				if err != nil || n < 1 || n > 65535 {
					m.warning = "port must be a number between 1 and 65535"
					return m, nil
				}
				m.values.Port = n
			}
			m.step = stepReview
			return m, nil
		}
		var cmd tea.Cmd
		m.portInput, cmd = m.portInput.Update(msg)
		return m, cmd

	case stepReview:
		if msg.Type == tea.KeyEnter {
			m.step = stepWorking
			m.phase = "Starting"
			return m, tea.Batch(m.startProvision(), m.spinner.Tick)
		}
		return m, nil
	}
	return m, nil
}

func (m createModel) previousStep() step {
	switch m.step {
	case stepKind:
		return stepName
	case stepVersion:
		return stepKind
	case stepMemory:
		return stepVersion
	case stepPort:
		return stepMemory
	case stepReview:
		return stepPort
	}
	return m.step
}

// versionItems marks the newest release so the common case — "just give me the
// current version" — is one keystroke away.
func versionItems(versions []string) []Item {
	items := make([]Item, 0, len(versions))
	for i, v := range versions {
		it := Item{Title: v}
		if i == 0 {
			it.Badge = "latest"
		}
		items = append(items, it)
	}
	return items
}

func (m createModel) View() string {
	var b strings.Builder
	b.WriteString("\n" + ui.Title.Render("  New Minecraft server") + "\n")
	b.WriteString("  " + ui.Subtle.Render(m.breadcrumb()) + "\n\n")

	switch m.step {
	case stepName:
		b.WriteString(m.question("What should this server be called?",
			"Used for the commands you'll type: svrctl start <name>"))
		b.WriteString("  " + m.nameInput.View() + "\n")

	case stepKind:
		b.WriteString(m.question("Which kind of server?", ""))
		b.WriteString(m.kindPick.view(m.width))

	case stepVersion:
		if m.versionsLoading {
			b.WriteString(m.question("Which Minecraft version?", ""))
			b.WriteString("  " + m.spinner.View() + ui.Subtle.Render(" fetching published versions…") + "\n")
			break
		}
		hint := "type to filter"
		if m.verPick.filter != "" {
			hint = "filter: " + m.verPick.filter
		}
		b.WriteString(m.question("Which Minecraft version?", hint))
		b.WriteString(m.verPick.view(m.width))

	case stepMemory:
		b.WriteString(m.question("How much memory should it get?",
			"You can change this later by editing the registry"))
		b.WriteString(m.memPick.view(m.width))

	case stepPort:
		b.WriteString(m.question("Which port should it listen on?",
			"Leave blank for Minecraft's default, 25565"))
		b.WriteString("  " + m.portInput.View() + "\n")

	case stepReview:
		b.WriteString(m.review())

	case stepWorking:
		b.WriteString(m.working())
	}

	if m.warning != "" {
		b.WriteString("\n  " + ui.Failure.Render(ui.GlyphFail+" "+m.warning) + "\n")
	}
	b.WriteString("\n  " + m.help() + "\n")
	return b.String()
}

func (m createModel) question(title, hint string) string {
	out := "  " + ui.Strong.Render(title) + "\n"
	if hint != "" {
		out += "  " + ui.Subtle.Render(hint) + "\n"
	}
	return out + "\n"
}

func (m createModel) breadcrumb() string {
	parts := []string{}
	if m.values.Name != "" {
		parts = append(parts, m.values.Name)
	}
	if m.values.Kind != "" {
		parts = append(parts, m.values.Kind)
	}
	if m.values.Version != "" {
		parts = append(parts, m.values.Version)
	}
	if len(parts) == 0 {
		return "step 1 of 6"
	}
	return strings.Join(parts, " · ")
}

func (m createModel) review() string {
	path := m.values.Path
	if path == "" {
		path = m.deps.DefaultPath(m.values.Name)
	}
	memory := m.values.Memory
	if memory == "" {
		memory = "JVM default"
	}
	port := "25565 (default)"
	if m.values.Port != 0 {
		port = strconv.Itoa(m.values.Port)
	}

	rows := [][2]string{
		{"Name", m.values.Name},
		{"Type", m.values.Kind},
		{"Version", m.values.Version},
		{"Memory", memory},
		{"Port", port},
		{"Location", path},
	}
	var b strings.Builder
	b.WriteString("  " + ui.Strong.Render("Ready to create") + "\n\n")
	for _, r := range rows {
		b.WriteString("  " + ui.Subtle.Render(ui.Pad(r[0], 10)) + ui.Body.Render(r[1]) + "\n")
	}
	// The EULA is settled here, before the downloads, rather than after them.
	b.WriteString("\n  " + ui.Subtle.Render("Continuing accepts Mojang's EULA:") + "\n")
	b.WriteString("  " + ui.Subtle.Render(m.deps.EULAURL) + "\n")
	return b.String()
}

func (m createModel) working() string {
	var b strings.Builder
	b.WriteString("  " + m.spinner.View() + ui.Strong.Render(" "+m.phase) + "\n\n")
	if m.total > 0 {
		b.WriteString("  " + m.bar.View() + "  " +
			ui.Subtle.Render(fmt.Sprintf("%s / %s", ui.Bytes(m.done), ui.Bytes(m.total))) + "\n")
	} else if m.done > 0 {
		b.WriteString("  " + ui.Subtle.Render(ui.Bytes(m.done)+" downloaded") + "\n")
	}
	return b.String()
}

func (m createModel) help() string {
	switch m.step {
	case stepWorking:
		return ui.Help.Render("this can take a few minutes the first time — Java is downloaded once and cached")
	case stepReview:
		return ui.HelpBar("enter", "create", "shift+tab", "back", "esc", "cancel")
	case stepName, stepPort:
		return ui.HelpBar("enter", "continue", "shift+tab", "back", "esc", "cancel")
	default:
		return ui.HelpBar("↑↓", "choose", "enter", "continue", "shift+tab", "back", "esc", "cancel")
	}
}
