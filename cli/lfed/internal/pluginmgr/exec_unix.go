//go:build unix

package pluginmgr

import (
	"os/exec"
	"syscall"
)

// signalExitCode returns 128+N for a plugin killed by signal N, as shells
// do. Any other state returns 1.
func signalExitCode(exit *exec.ExitError) int {
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return 1
}
