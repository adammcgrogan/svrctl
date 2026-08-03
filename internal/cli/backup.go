// backup implements `svrctl backup`: snapshotting and restoring a server's
// world data.
package cli

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/paths"
	"github.com/adammcgrogan/svrctl/internal/process"
	"github.com/adammcgrogan/svrctl/internal/ui"
)

// backupTimeFormat sorts lexically the same as chronologically, so filenames
// double as a "list, newest last" order with no extra parsing.
const backupTimeFormat = "2006-01-02T15-04-05"

func newBackupCmd() *cobra.Command {
	backup := &cobra.Command{
		Use:   "backup",
		Short: "Snapshot and restore a server's world data",
	}
	backup.AddCommand(newBackupCreateCmd(), newBackupListCmd(), newBackupRestoreCmd())
	return backup
}

func newBackupCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "create <name>",
		Short:             "Snapshot a server's world data",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			out := cmd.OutOrStdout()

			flushed := false
			info, err := createServerBackup(name, func() {
				flushed = true
				ui.Stepf(out, "Flushing world data before snapshotting…")
			})
			if err != nil {
				return err
			}

			ui.Okf(out, "Backed up %s (%s)", ui.Strong.Render(name), ui.Bytes(info.SizeBytes))
			if flushed {
				ui.Hintf(out, "svrctl backup list %s", name)
			}
			return nil
		},
	}
	return cmd
}

func newBackupListCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:               "list <name>",
		Short:             "List a server's backups",
		Args:              requireArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if _, err := resolveServer(name); err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			backups, err := listBackups(name)
			if err != nil {
				return err
			}

			if asJSON {
				return writeJSON(out, backups)
			}
			if len(backups) == 0 {
				fmt.Fprintln(out, ui.Subtle.Render("No backups yet."))
				ui.Hintf(out, "svrctl backup create %s", name)
				return nil
			}
			for _, b := range backups {
				fmt.Fprintln(out, ui.Body.Render(ui.Pad(b.ID, 24))+
					ui.Subtle.Render(ui.Pad(ui.Bytes(b.SizeBytes), 10)+b.CreatedAt.Format(time.RFC822)))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON instead of a table")
	return cmd
}

func newBackupRestoreCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "restore <name> <backup>",
		Short: "Restore a server's world data from a backup",
		Long: "Replaces the server's current world data with a snapshot taken by\n" +
			"`svrctl backup create`. The server must be stopped first. Pass \"latest\"\n" +
			"instead of a backup ID to restore the most recent one.",
		Example:           "  svrctl backup restore survival latest\n  svrctl backup restore survival 2026-01-02T15-04-05",
		Args:              requireArgs(2),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, id := args[0], args[1]
			out := cmd.OutOrStdout()

			s, err := resolveServer(name)
			if err != nil {
				return err
			}
			if _, ok := process.IsRunning(s.Path); ok {
				return fmt.Errorf("%q is running — stop it first with `svrctl stop %s`", name, name)
			}
			if _, _, err := resolveBackup(name, id); err != nil {
				return err
			}

			if !yes {
				if !ui.Interactive() {
					return fmt.Errorf("refusing to overwrite %s's current world without confirmation; pass --yes to proceed", name)
				}
				if err := confirmRestore(cmd.InOrStdin(), out, name); err != nil {
					return err
				}
			}

			resolvedID, err := restoreServerBackup(name, id)
			if err != nil {
				return err
			}

			ui.Okf(out, "Restored %s from %s", ui.Strong.Render(name), resolvedID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

// restoreServerBackup replaces name's current world data with the backup
// identified by id ("latest" or a specific ID), shared by the `backup
// restore` command and the dashboard's backups sub-mode. Callers are
// responsible for confirming with the user and checking the server is
// stopped before calling this — it does neither itself.
func restoreServerBackup(name, id string) (resolvedID string, err error) {
	s, err := resolveServer(name)
	if err != nil {
		return "", err
	}
	backupPath, resolvedID, err := resolveBackup(name, id)
	if err != nil {
		return "", err
	}

	worldDirs, err := worldDirectories(s.Path)
	if err != nil {
		return "", err
	}
	for _, dir := range worldDirs {
		if err := os.RemoveAll(filepath.Join(s.Path, dir)); err != nil {
			return "", fmt.Errorf("clearing current world data: %w", err)
		}
	}
	if err := extractWorldArchive(backupPath, s.Path); err != nil {
		return "", err
	}
	return resolvedID, nil
}

// confirmRestore requires the user to type the server's name before its
// current world data is discarded — restoring is unrecoverable for whatever
// wasn't in the backup.
func confirmRestore(in io.Reader, out io.Writer, name string) error {
	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.Warning.Render("This replaces "+name+"'s current world data with the backup"))
	fmt.Fprintln(out, ui.Subtle.Render("Anything played since that backup was taken is gone."))
	fmt.Fprint(out, ui.Body.Render("Type the server's name to confirm: "))

	line, _ := bufio.NewReader(in).ReadString('\n')
	if strings.TrimSpace(line) != name {
		return fmt.Errorf("names did not match, so nothing was restored")
	}
	return nil
}

// createServerBackup snapshots name's world data into a new timestamped
// archive, shared by the `backup create` command and the dashboard's backups
// sub-mode. If the server is running, onFlush (which may be nil) is called
// right before autosave is paused and the data flushed, so a caller can
// report that the operation is briefly touching a live server.
func createServerBackup(name string, onFlush func()) (backupInfo, error) {
	s, err := resolveServer(name)
	if err != nil {
		return backupInfo{}, err
	}

	worldDirs, err := worldDirectories(s.Path)
	if err != nil {
		return backupInfo{}, err
	}
	if len(worldDirs) == 0 {
		return backupInfo{}, fmt.Errorf("%q has no world data yet — start it at least once first", name)
	}

	backupsDir, err := paths.BackupsDir(name)
	if err != nil {
		return backupInfo{}, err
	}
	id := time.Now().Format(backupTimeFormat)
	backupPath := filepath.Join(backupsDir, id+".tar.gz")

	if _, ok := process.IsRunning(s.Path); ok {
		if onFlush != nil {
			onFlush()
		}
		if err := prepareLiveBackup(s.Path); err != nil {
			return backupInfo{}, err
		}
		defer process.SendCommand(s.Path, "save-on")
	}

	if err := createWorldArchive(backupPath, s.Path, worldDirs); err != nil {
		os.Remove(backupPath)
		return backupInfo{}, err
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return backupInfo{}, err
	}
	return backupInfo{ID: id, CreatedAt: time.Now(), SizeBytes: info.Size(), backupPath: backupPath}, nil
}

// prepareLiveBackup asks a running server to flush its world data and pause
// autosaving, so the archive created right after is consistent. The caller
// is responsible for sending "save-on" again once it's done.
func prepareLiveBackup(serverDir string) error {
	if err := process.SendCommand(serverDir, "save-off"); err != nil {
		return fmt.Errorf("pausing autosave: %w", err)
	}
	if err := process.SendCommand(serverDir, "save-all flush"); err != nil {
		return fmt.Errorf("flushing world data: %w", err)
	}
	// save-all is asynchronous; give the write a moment to land before tar
	// starts reading the region files.
	time.Sleep(1 * time.Second)
	return nil
}

// worldDirectories returns the world save directories that exist under
// serverDir, derived from server.properties' level-name (default "world").
// Vanilla and Paper both lay dimensions out as <level-name>, <level-name>_nether,
// and <level-name>_the_end.
func worldDirectories(serverDir string) ([]string, error) {
	props, _, err := readProperties(filepath.Join(serverDir, "server.properties"))
	if err != nil {
		return nil, err
	}
	levelName := props["level-name"]
	if levelName == "" {
		levelName = "world"
	}

	var dirs []string
	for _, candidate := range []string{levelName, levelName + "_nether", levelName + "_the_end"} {
		if info, err := os.Stat(filepath.Join(serverDir, candidate)); err == nil && info.IsDir() {
			dirs = append(dirs, candidate)
		}
	}
	return dirs, nil
}

// createWorldArchive tars and gzips the given directories (relative to
// serverDir) into destPath.
func createWorldArchive(destPath, serverDir string, dirs []string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	for _, dir := range dirs {
		root := filepath.Join(serverDir, dir)
		err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(serverDir, path)
			if err != nil {
				return err
			}
			hdr, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				return err
			}
			hdr.Name = filepath.ToSlash(rel)
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if fi.IsDir() {
				return nil
			}
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(tw, in)
			return err
		})
		if err != nil {
			tw.Close()
			gz.Close()
			return fmt.Errorf("archiving %s: %w", dir, err)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing backup archive: %w", err)
	}
	return gz.Close()
}

// extractWorldArchive extracts a backup created by createWorldArchive back
// into serverDir, guarding against zip-slip the same way javamgr's JDK
// extraction does.
func extractWorldArchive(archivePath, serverDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening backup: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("opening backup: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading backup: %w", err)
		}

		target := filepath.Join(serverDir, hdr.Name)
		if !strings.HasPrefix(target, filepath.Clean(serverDir)+string(os.PathSeparator)) {
			return fmt.Errorf("backup entry %q escapes the server directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

// backupInfo describes one snapshot on disk, for `svrctl backup list`.
type backupInfo struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	SizeBytes  int64     `json:"size_bytes"`
	backupPath string
}

// listBackups returns a server's backups, newest first.
func listBackups(name string) ([]backupInfo, error) {
	dir, err := paths.BackupsDir(name)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing backups: %w", err)
	}

	var backups []backupInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".tar.gz")
		createdAt, err := time.ParseInLocation(backupTimeFormat, id, time.Local)
		if err != nil {
			createdAt = info.ModTime()
		}
		backups = append(backups, backupInfo{
			ID:         id,
			CreatedAt:  createdAt,
			SizeBytes:  info.Size(),
			backupPath: filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].ID > backups[j].ID })
	return backups, nil
}

// resolveBackup finds the archive path for id, which may be "latest" or a
// specific backup ID as printed by `svrctl backup list`.
func resolveBackup(name, id string) (path string, resolvedID string, err error) {
	backups, err := listBackups(name)
	if err != nil {
		return "", "", err
	}
	if len(backups) == 0 {
		return "", "", fmt.Errorf("%q has no backups yet (run `svrctl backup create %s`)", name, name)
	}
	if id == "latest" {
		return backups[0].backupPath, backups[0].ID, nil
	}
	for _, b := range backups {
		if b.ID == id {
			return b.backupPath, b.ID, nil
		}
	}
	return "", "", fmt.Errorf("no backup %q for %q (run `svrctl backup list %s`)", id, name, name)
}
