// Package version implements `lfed version`.
package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
)

// NewCmd builds `lfed version`.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the lfed version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(f.IOStreams.Out, "lfed %s\n", f.Version)
			return err
		},
	}
}
