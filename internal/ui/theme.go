// Package ui holds svrctl's shared terminal presentation layer: the colour
// palette and text styles used by both the plain command output and the
// interactive Bubble Tea screens, so the two never drift apart.
package ui

import "github.com/charmbracelet/lipgloss"

// The palette is deliberately small. Servers are either up, down, or busy, so
// the colours that carry meaning are green, grey, and amber; everything else
// is structure. Adaptive pairs keep it legible on light terminals too.
var (
	ColorAccent = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#5BBF63"}
	ColorText   = lipgloss.AdaptiveColor{Light: "#1C1C1C", Dark: "#E4E4E4"}
	ColorMuted  = lipgloss.AdaptiveColor{Light: "#6E6E6E", Dark: "#8A8A8A"}
	ColorFaint  = lipgloss.AdaptiveColor{Light: "#9A9A9A", Dark: "#5A5A5A"}
	ColorWarn   = lipgloss.AdaptiveColor{Light: "#B26A00", Dark: "#E0A040"}
	ColorError  = lipgloss.AdaptiveColor{Light: "#C02A2A", Dark: "#E06C6C"}
	ColorRule   = lipgloss.AdaptiveColor{Light: "#D8D8D8", Dark: "#3A3A3A"}
)

// Status glyphs. Solid means running, hollow means stopped; both are one cell
// wide so columns line up whatever the state.
const (
	GlyphRunning = "●"
	GlyphStopped = "○"
	GlyphOK      = "✓"
	GlyphFail    = "✗"
	GlyphArrow   = "→"
	GlyphPrompt  = "›"
)

var (
	// Title is the screen heading used at the top of interactive views.
	Title = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	// Subtle is for supporting text that should recede: paths, hints, counts.
	Subtle = lipgloss.NewStyle().Foreground(ColorMuted)
	// Faint is for text that is present but almost never read: separators.
	Faint = lipgloss.NewStyle().Foreground(ColorFaint)
	// Body is ordinary foreground text.
	Body = lipgloss.NewStyle().Foreground(ColorText)
	// Strong emphasises a value inside a line of body text.
	Strong = lipgloss.NewStyle().Foreground(ColorText).Bold(true)

	// Running / Stopped colour the status glyph and word in listings.
	Running = lipgloss.NewStyle().Foreground(ColorAccent)
	Stopped = lipgloss.NewStyle().Foreground(ColorMuted)

	// Success / Warning / Failure prefix one-line command results.
	Success = lipgloss.NewStyle().Foreground(ColorAccent)
	Warning = lipgloss.NewStyle().Foreground(ColorWarn)
	Failure = lipgloss.NewStyle().Foreground(ColorError)

	// Header is the column header row of a table.
	Header = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)
	// Selected marks the highlighted row of an interactive list.
	Selected = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	// Key and Help render the keybinding footer: "s start · x stop · q quit".
	Key  = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	Help = lipgloss.NewStyle().Foreground(ColorMuted)
)

// StatusGlyph returns the coloured glyph plus label for a server's state.
func StatusGlyph(running bool) string {
	if running {
		return Running.Render(GlyphRunning + " running")
	}
	return Stopped.Render(GlyphStopped + " stopped")
}

// Rule draws a horizontal separator of the given width.
func Rule(width int) string {
	if width <= 0 {
		return ""
	}
	line := make([]rune, width)
	for i := range line {
		line[i] = '─'
	}
	return lipgloss.NewStyle().Foreground(ColorRule).Render(string(line))
}

// HelpBar joins key/description pairs into a footer hint line. Pairs are given
// flat: "s", "start", "q", "quit".
func HelpBar(pairs ...string) string {
	out := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			out += Faint.Render("  ·  ")
		}
		out += Key.Render(pairs[i]) + Help.Render(" "+pairs[i+1])
	}
	return out
}
