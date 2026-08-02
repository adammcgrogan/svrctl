// logs_test covers which log file gets shown and how much of it.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func writeLog(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLogFileForPrefersTheServersOwnLog(t *testing.T) {
	dir := t.TempDir()
	serverLog := filepath.Join(dir, "logs", "latest.log")
	writeLog(t, serverLog, "[12:00:00] [Server thread/INFO]: Done\n")
	writeLog(t, process.ConsoleLogPath(dir), "raw process output\n")

	got, ok := logFileFor(dir)
	if !ok || got != serverLog {
		t.Errorf("got %q (ok=%v), want the server's own log", got, ok)
	}
}

func TestLogFileForFallsBackToTheCapturedConsole(t *testing.T) {
	dir := t.TempDir()
	consoleLog := process.ConsoleLogPath(dir)
	// A server that dies before its logging starts — a bad jar, or the JVM
	// refusing the heap size — never writes logs/latest.log at all, and the
	// captured output is the only record of why.
	writeLog(t, consoleLog, "Error: Could not create the Java Virtual Machine\n")

	got, ok := logFileFor(dir)
	if !ok || got != consoleLog {
		t.Errorf("got %q (ok=%v), want the captured console log", got, ok)
	}
}

func TestLogFileForIgnoresAnEmptyServerLog(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, filepath.Join(dir, "logs", "latest.log"), "")
	writeLog(t, process.ConsoleLogPath(dir), "Exception in thread main\n")

	got, _ := logFileFor(dir)
	if got != process.ConsoleLogPath(dir) {
		t.Errorf("got %q, want the console log when the server log is empty", got)
	}
}

func TestLogFileForReportsNothingWhenNeverStarted(t *testing.T) {
	if _, ok := logFileFor(t.TempDir()); ok {
		t.Error("reported a log file for a server that has never run")
	}
}

func TestPrintTailShowsOnlyTheLastLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	var content strings.Builder
	for i := 1; i <= 200; i++ {
		content.WriteString("line " + strconv.Itoa(i) + "\n")
	}
	writeLog(t, path, content.String())

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var out bytes.Buffer
	if err := printTail(&out, f, 10); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if strings.Contains(got, "line 190\n") {
		t.Errorf("showed more than the requested 10 lines:\n%s", got)
	}
	if !strings.Contains(got, "line 191") || !strings.Contains(got, "line 200") {
		t.Errorf("did not show the last 10 lines:\n%s", got)
	}
	if n := strings.Count(got, "\n"); n != 10 {
		t.Errorf("printed %d lines, want 10", n)
	}
}

func TestPrintTailZeroMeansWholeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	writeLog(t, path, "one\ntwo\nthree\n")

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var out bytes.Buffer
	if err := printTail(&out, f, 0); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "one\ntwo\nthree\n" {
		t.Errorf("got %q, want the whole file", got)
	}
}
