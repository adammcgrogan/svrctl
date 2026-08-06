// root_test covers what the root command does with arguments it does not
// recognise, since that is the first thing a mistyped invocation hits.
package cli

import (
	"io"
	"strings"
	"testing"
)

// execRoot runs the root command with args, discarding its output, and returns
// whatever error came back.
func execRoot(t *testing.T, args ...string) error {
	t.Helper()
	root := NewRoot()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	return root.Execute()
}

func TestUnknownCommandSuggestsTheLikelyTypo(t *testing.T) {
	err := execRoot(t, "statu", "survival")
	if err == nil {
		t.Fatal("mistyped command was accepted")
	}

	msg := err.Error()
	if !strings.Contains(msg, `unknown command "statu"`) {
		t.Errorf("got %q, want it to name the unknown command", msg)
	}
	if !strings.Contains(msg, "Did you mean this?") || !strings.Contains(msg, "status") {
		t.Errorf("got %q, want a suggestion of `status`", msg)
	}
}

func TestUnknownCommandDoesNotGuessWildly(t *testing.T) {
	err := execRoot(t, "wibble")
	if err == nil {
		t.Fatal("unknown command was accepted")
	}
	if strings.Contains(err.Error(), "Did you mean this?") {
		t.Errorf("suggested an unrelated command: %q", err)
	}
}

func TestBareInvocationTakesNoArgumentsButIsNotAnError(t *testing.T) {
	// With no terminal, the no-argument form prints help instead of opening
	// the dashboard, so this exercises the path without a Bubble Tea program.
	if err := execRoot(t); err != nil {
		t.Errorf("bare invocation failed: %v", err)
	}
}
