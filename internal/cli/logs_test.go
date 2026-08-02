// logs_test covers which log file gets shown and how much of it.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestLogFollowerBlocksThenReadsAppendedData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	writeLog(t, path, "one\n")

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tailLines(f, 10); err != nil {
		t.Fatal(err)
	}

	follower := &logFollower{f: f, done: make(chan struct{})}
	defer follower.Close()

	// Nothing new yet: Read must block rather than reporting EOF, since a
	// stopped follow would otherwise look like the log had ended.
	readDone := make(chan struct{})
	go func() {
		buf := make([]byte, 64)
		n, err := follower.Read(buf)
		if err != nil {
			t.Errorf("Read returned an error instead of blocking: %v", err)
		}
		if got := string(buf[:n]); got != "two\n" {
			t.Errorf("got %q, want the appended line", got)
		}
		close(readDone)
	}()

	select {
	case <-readDone:
		t.Fatal("Read returned before there was anything new to read")
	default:
	}

	appended, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appended.WriteString("two\n"); err != nil {
		t.Fatal(err)
	}
	appended.Close()

	<-readDone
}

func TestLogFollowerCloseUnblocksRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	writeLog(t, path, "")

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	follower := &logFollower{f: f, done: make(chan struct{})}

	done := make(chan error, 1)
	go func() {
		_, err := follower.Read(make([]byte, 64))
		done <- err
	}()

	follower.Close()

	// Read must unblock promptly once closed. Whether it reports io.EOF or a
	// "file already closed" from a Read that was in flight when Close ran the
	// underlying os.File.Close doesn't matter — the scanner reading it treats
	// any error as the end of the stream.
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected an error once closed, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read did not unblock after Close")
	}
}
