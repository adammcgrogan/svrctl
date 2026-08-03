// run implements the hidden `svrctl __run` subcommand: the entrypoint used
// by the detached runner process spawned by `svrctl server start`. Not meant
// to be invoked directly by users.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/adammcgrogan/svrctl/internal/process"
)

func newHiddenRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "__run <name>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveServer(args[0])
			if err != nil {
				return err
			}
			// The runner is detached with no terminal to draw on; by the time
			// it starts, `svrctl start` has already ensured the JDK anyway.
			javaPath, err := resolveJavaPath(s, nil)
			if err != nil {
				process.WriteRunError(s.Path, err)
				return err
			}
			if err := process.Run(process.RunOptions{
				ServerDir: s.Path,
				JavaPath:  javaPath,
				Memory:    s.Memory,
			}); err != nil {
				process.WriteRunError(s.Path, err)
				return err
			}
			return nil
		},
	}
	return cmd
}
