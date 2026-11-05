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

	err := Exec(context.Background(), f, p, []string{"kill"})
	if ExitCode(err) != 137 {
		t.Errorf("kill: err = %v, want exit 137", err)
	}

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
