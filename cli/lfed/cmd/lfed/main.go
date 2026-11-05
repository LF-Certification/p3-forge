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

const exitInterrupted = 130

func main() {
	os.Exit(run())
}

func run() int {

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
