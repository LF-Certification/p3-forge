//go:build unix

package pluginmgr

import (
	"os/exec"
	"syscall"
)

func signalExitCode(exit *exec.ExitError) int {
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return 1
}
