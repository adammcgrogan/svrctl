package tui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain pins a colour profile for the whole package. Lipgloss otherwise
// detects that `go test` output is not a terminal and renders everything
// unstyled, which would quietly turn every assertion about highlighting into a
// test that passes for the wrong reason.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}
