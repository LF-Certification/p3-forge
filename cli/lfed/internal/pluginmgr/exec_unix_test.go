//go:build unix

package pluginmgr

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"syscall"
	"testing"
)

func TestExecSignalExitCodes(t *testing.T) {
	setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	p := installHello(t, f)

	// A plugin killed by SIGKILL (9) maps to 137.
	err := Exec(context.Background(), f, p, []string{"kill"})
	if ExitCode(err) != 137 {
		t.Errorf("kill: err = %v, want exit 137", err)
	}

	// A relayed SIGTERM (15) reaches the plugin: it maps to 143. run has
	// Notify registered before the plugin prints "ready", so the test
	// process survives the signal it sends to itself.
	pr, pw := io.Pipe()
	f.IOStreams.Out = pw
	done := make(chan error, 1)
	go func() { done <- Exec(context.Background(), f, p, []string{"wait"}) }()
	line, err := bufio.NewReader(pr).ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("read ready: %q, %v", line, err)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	err = <-done
	pw.Close()
	if ExitCode(err) != 143 {
		t.Errorf("relay: err = %v, want exit 143", err)
	}
}
