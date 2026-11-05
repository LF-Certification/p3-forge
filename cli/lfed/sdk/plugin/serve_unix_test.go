//go:build unix

package plugin

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// TestServeCancelGrace runs Serve in a child process whose command blocks
// on a read that does not watch the context. After SIGTERM the child MUST
// exit 130 within CancelGrace, not wait for input.
func TestServeCancelGrace(t *testing.T) {
	if os.Getenv("LFED_SDK_TEST_CHILD") == "1" {
		CancelGrace = 200 * time.Millisecond
		root := &cobra.Command{Use: "lfed-demo", RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "ready")
			_, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			return err
		}}
		root.SetIn(os.Stdin)
		root.SetOut(os.Stdout)
		Serve(root, Manifest{Name: "demo"})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestServeCancelGrace$")
	cmd.Env = append(os.Environ(), "LFED_SDK_TEST_CHILD=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("read ready: %q, %v", line, err)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != exitInterrupted {
			t.Fatalf("child: %v, want exit %d", err, exitInterrupted)
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child did not exit after SIGTERM; the cancel grace did not end it")
	}
}
