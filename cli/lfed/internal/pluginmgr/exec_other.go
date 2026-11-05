//go:build !unix

package pluginmgr

import "os/exec"

// signalExitCode returns 1: this platform reports no terminating signal.
func signalExitCode(*exec.ExitError) int {
	return 1
}
