//go:build !windows

// setDetached (unix) puts the child in its own session so it survives the
// parent CLI process exiting.
package process

import (
	"os/exec"
	"syscall"
)

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
