// logs implements `svrctl logs`: the server's recent output, optionally
// followed live.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

// defaultLogLines is how much history `svrctl logs` shows. Dumping the whole
// file, as it used to, buries the part anyone actually wanted: the end.
const defaultLogLines = 50

func newLogsCmd() *cobra.Command {
	var follow bool
	var lines int

	cmd := &cobra.Command{
		Use:               "logs <name>",
		Short:             "Show a server's recent output",
		Example:           "  svrctl logs survival\n  svrctl logs survival -f\n  svrctl logs survival -n 500",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := viewServer(args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			logPath, ok := logFileFor(v.Server.Path)
			if !ok {
				return fmt.Errorf("%q has no logs yet — it has not been started", v.Name)
			}

			f, err := os.Open(logPath)
			if err != nil {
				return fmt.Errorf("opening log file: %w", err)
			}
			defer f.Close()

			if err := printTail(out, f, lines); err != nil {
				return err
			}
			if !follow {
				if !v.running() {
					ui.Warnf(out, "%s is not running, so this log ends where it last stopped", v.Name)
				}
				return nil
			}
			return followLog(out, f)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing new output as it arrives")
	cmd.Flags().IntVarP(&lines, "lines", "n", defaultLogLines, "how many recent lines to show (0 for the whole file)")
	return cmd
}

// logFileFor picks which log to show. The server's own logs/latest.log is the
// readable one and is what an admin expects to see, but a server that died
// before its logging started never writes it — in that case the runner's
// capture of the raw process output is the only record of what went wrong.
func logFileFor(serverDir string) (string, bool) {
	serverLog := filepath.Join(serverDir, "logs", "latest.log")
	if info, err := os.Stat(serverLog); err == nil && info.Size() > 0 {
		return serverLog, true
	}
	consoleLog := process.ConsoleLogPath(serverDir)
	if _, err := os.Stat(consoleLog); err == nil {
		return consoleLog, true
	}
	return "", false
}

// printTail writes the last n lines of f and leaves the file offset at the end
// so a subsequent follow picks up from there.
func printTail(out io.Writer, f *os.File, n int) error {
	if n <= 0 {
		_, err := io.Copy(out, f)
		return err
	}

	// Server logs are small enough (they rotate per run) to scan in one pass
	// rather than seeking backwards for line breaks.
	var ring []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		ring = append(ring, scanner.Text())
		if len(ring) > n {
			ring = ring[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading log file: %w", err)
	}
	for _, line := range ring {
		fmt.Fprintln(out, colorizeLog(line))
	}
	return nil
}

// followLog streams appended output until interrupted.
func followLog(out io.Writer, f *os.File) error {
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			fmt.Fprint(out, colorizeLog(strings.TrimRight(line, "\n"))+"\n")
		}
		if err == io.EOF {
			// Nothing new; wait rather than spinning on the file.
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
	}
}

// colorizeLog tints a log line by severity, matching the console view.
func colorizeLog(line string) string {
	switch {
	case strings.Contains(line, "/ERROR]"), strings.Contains(line, "/FATAL]"):
		return ui.Failure.Render(line)
	case strings.Contains(line, "/WARN]"):
		return ui.Warning.Render(line)
	default:
		return line
	}
}
