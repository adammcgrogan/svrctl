package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// requireArgs enforces exactly n positional args, reporting the command's
// usage (and example, if set) instead of Cobra's terse "accepts N arg(s),
// received M" when the count is wrong.
func requireArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return usageError(cmd)
		}
		return nil
	}
}

// requireMinArgs is requireArgs for a minimum instead of exact count.
func requireMinArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return usageError(cmd)
		}
		return nil
	}
}

func usageError(cmd *cobra.Command) error {
	msg := fmt.Sprintf("missing required argument\n\nUsage: %s", cmd.UseLine())
	if cmd.Example != "" {
		msg += "\nExample:\n" + cmd.Example
	}
	return errors.New(msg)
}
