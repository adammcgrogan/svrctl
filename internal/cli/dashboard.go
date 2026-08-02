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
