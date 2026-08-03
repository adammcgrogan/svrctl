// dashboard adapts the command layer's registry and process operations into
// the callbacks the dashboard TUI drives.
package cli

import (
	"io"

	"github.com/adammcgrogan/svrctl/internal/tui"
)

// dashboardDeps builds the dependency set for the dashboard. Lifecycle output
// is discarded: the dashboard reports success or failure in its own status
// line, and stray writes would corrupt the alternate screen it is drawing on.
func dashboardDeps() tui.DashboardDeps {
	return tui.DashboardDeps{
		List:    dashboardRows,
		Start:   func(name string) error { return startServer(io.Discard, name) },
		Stop:    stopServer,
		Restart: func(name string) error { return restartServer(io.Discard, name) },
		Remove:  removeServer,

		Edit: func(name, memory string, port int) error {
			_, err := editServer(name, &memory, nil, &port)
			return err
		},

		Properties:  serverProperties,
		SetProperty: func(name, key, value string) error { _, err := setServerProperty(name, key, value); return err },

		Backups:      dashboardBackups,
		CreateBackup: func(name string) error { _, err := createServerBackup(name, nil); return err },
		RestoreBackup: func(name, id string) error {
			_, err := restoreServerBackup(name, id)
			return err
		},
	}
}

func dashboardRows() ([]tui.ServerRow, error) {
	views, err := viewAll()
	if err != nil {
		return nil, err
	}
	rows := make([]tui.ServerRow, 0, len(views))
	for _, v := range views {
		row := tui.ServerRow{
			Name:    v.Name,
			Type:    v.Server.Type,
			Version: v.Server.Version,
			Path:    v.Server.Path,
			Port:    v.port(),
			Memory:  v.Server.Memory,
			Group:   v.Server.Group,
			Running: v.running(),
			Uptime:  v.Uptime,
		}
		if v.running() {
			row.PID = v.State.PID
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func dashboardBackups(name string) ([]tui.BackupRow, error) {
	backups, err := listBackups(name)
	if err != nil {
		return nil, err
	}
	rows := make([]tui.BackupRow, len(backups))
	for i, b := range backups {
		rows[i] = tui.BackupRow{ID: b.ID, SizeBytes: b.SizeBytes, CreatedAt: b.CreatedAt}
	}
	return rows, nil
}
