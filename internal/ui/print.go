package ui

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Okf prints a completed action: "✓ Started "survival"".
func Okf(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, Success.Render(GlyphOK)+" "+fmt.Sprintf(format, args...))
}

// Warnf prints a non-fatal caveat the user should notice.
func Warnf(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, Warning.Render("!")+" "+fmt.Sprintf(format, args...))
}

// Stepf prints an in-progress step of a longer operation.
func Stepf(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, Subtle.Render(GlyphArrow+" "+fmt.Sprintf(format, args...)))
}

// Hintf prints the command the user most likely wants to run next. Every
// command that leaves the user mid-workflow ends with one of these.
func Hintf(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, Subtle.Render("  next: ")+Strong.Render(fmt.Sprintf(format, args...)))
}

// Field prints an aligned "label  value" line for detail views.
func Field(w io.Writer, label, value string) {
	fmt.Fprintf(w, "%s  %s\n", Subtle.Render(fmt.Sprintf("%-9s", label)), value)
}

// Bytes formats a byte count for humans: 4.2 MB, 812 KB.
func Bytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// Duration formats an uptime as the largest two useful units: "3d 4h", "12m".
func Duration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// Truncate shortens s to width, ending in an ellipsis when it had to cut.
func Truncate(s string, width int) string {
	r := []rune(s)
	if width <= 0 || len(r) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}

// Pad right-pads s to width with spaces (no-op if it is already longer).
// Width is counted in characters, not bytes, so a name with an accent in it
// does not shunt every column after it out of line.
func Pad(s string, width int) string {
	n := len([]rune(s))
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// TextWidth is the number of characters in s, which is what column sizing
// needs — len() would count UTF-8 bytes instead.
func TextWidth(s string) int {
	return len([]rune(s))
}

// PadStyled pads an already-styled string to width using its *visible* length.
// Padding on the raw length would count colour escapes as characters and pull
// every column after it out of alignment.
func PadStyled(s string, width int) string {
	visible := len([]rune(StripANSI(s)))
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// StripANSI removes SGR escape sequences, for measuring rendered width.
func StripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}
