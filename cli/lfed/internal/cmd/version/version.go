// Package version implements `lfed version`.
package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

// result is the -o json|yaml document of `lfed version`.
type result struct {
	Version string `json:"version"`
}

// NewCmd builds `lfed version`.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the lfed version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := f.Output()
			if err != nil {
				return err
			}
			if mode.Structured() {
				return output.PrintValue(f.IOStreams.Out, mode, result{Version: f.Version})
			}
			_, err = fmt.Fprintf(f.IOStreams.Out, "lfed %s\n", f.Version)
			return err
		},
	}
}
