package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// plainForced is set by the global --plain flag. It suppresses every
// interactive screen and all colour, so svrctl behaves the same in a script as
// it does when a human runs it with output piped somewhere.
var plainForced bool

// SetPlain forces plain, non-interactive output for the rest of the process.
func SetPlain(plain bool) {
	plainForced = plain
	if plain {
		// termenv.Ascii, not the zero value — Profile 0 is TrueColor, so
		// passing 0 here forces colour *on*, which is the exact opposite.
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// Interactive reports whether it is safe to take over the terminal with a full
// screen Bubble Tea program: both ends of the pipe must be a real terminal,
// the terminal must be capable, and the user must not have opted out.
//
// Every command that can open a TUI checks this first and has a plain
// equivalent for when it returns false, so svrctl stays scriptable.
func Interactive() bool {
	if plainForced {
		return false
	}
	if os.Getenv("SVRCTL_NO_TUI") != "" || os.Getenv("CI") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return IsTerminal(os.Stdin) && IsTerminal(os.Stdout)
}

// IsTerminal reports whether f is attached to a terminal.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// Width returns the terminal's column count, or a sensible default when the
// output is not a terminal (piped output should not wrap at 80 arbitrarily).
func Width() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 100
	}
	return w
}
