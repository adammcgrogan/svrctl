// Command svrctl is a CLI for managing local Minecraft servers: creating,
// starting, stopping, and monitoring Vanilla and Paper servers.
package main

import (
	"fmt"
	"os"

	"github.com/adammcgrogan/svrctl/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
