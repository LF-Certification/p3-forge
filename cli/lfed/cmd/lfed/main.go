// Command lfed is the education platform CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/auth"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/build"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmd/root"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/pluginmgr"
)

func main() {
	f := cmdutil.New(build.Version)
	f.Token = auth.TokenResolver(f)
	if err := root.Run(context.Background(), f, os.Args[1:]); err != nil {
		// A plugin's exit code passes through; the plugin already reported.
		var exit *pluginmgr.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.Code)
		}
		fmt.Fprintf(f.IOStreams.ErrOut, "lfed: %v\n", err)
		os.Exit(pluginmgr.ExitCode(err))
	}
}
