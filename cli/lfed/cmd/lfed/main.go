// Command lfed is the education platform CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/auth"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/build"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmd/root"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/pluginmgr"
)

// exitInterrupted is the exit code after SIGINT or SIGTERM cancels the run.
const exitInterrupted = 130

func main() {
	os.Exit(run())
}

// run returns the process exit code. It is separate from main so the
// deferred stop runs before os.Exit.
func run() int {
	// SIGINT and SIGTERM cancel ctx. Commands then unwind and run their
	// deferred cleanup, for example the removal of the plugin staging
	// directory. A second signal kills the process with the default action.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()

	f := cmdutil.New(build.Version)
	f.Token = auth.TokenResolver(f)
	err := root.Run(ctx, f, os.Args[1:])
	if err == nil {
		return 0
	}
	// A plugin's exit code passes through; the plugin already reported.
	var exit *pluginmgr.ExitError
	if errors.As(err, &exit) {
		return exit.Code
	}
	if ctx.Err() != nil {
		if !errors.Is(err, context.Canceled) {
			fmt.Fprintf(f.IOStreams.ErrOut, "lfed: %v\n", err)
		}
		return exitInterrupted
	}
	fmt.Fprintf(f.IOStreams.ErrOut, "lfed: %v\n", err)
	return pluginmgr.ExitCode(err)
}
