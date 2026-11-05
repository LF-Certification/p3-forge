//go:build !unix

package pluginmgr

import "os/exec"

func signalExitCode(*exec.ExitError) int {
	return 1
}
