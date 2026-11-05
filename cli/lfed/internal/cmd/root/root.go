// Package root builds the `lfed` root command and dispatches unknown
// commands to plugins. See Run for the dispatch mechanism.
package root

import (
	"github.com/spf13/cobra"

	authcmd "github.com/LF-Certification/p3-forge/cli/lfed/internal/cmd/auth"
	contextcmd "github.com/LF-Certification/p3-forge/cli/lfed/internal/cmd/context"
	plugincmd "github.com/LF-Certification/p3-forge/cli/lfed/internal/cmd/plugin"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmd/version"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

// NewCmd builds the root command with its global flags and core commands.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "lfed",
		Short:         "Education platform CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       f.Version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := f.Output(); err != nil {
				return err
			}
			if f.JQ != "" && f.Capture != nil {
				f.Capture.Start()
			}
			return nil
		},
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return plugincmd.Names(), cobra.ShellCompDirectiveNoFileComp
		},
	}
	cmd.SetIn(f.IOStreams.In)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	cmd.SetUsageTemplate(usageTemplate(cmd))
	cmd.SetVersionTemplate("lfed {{.Version}}\n")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &cmdutil.UsageError{Err: err}
	})

	pf := cmd.PersistentFlags()
	pf.StringVarP(&f.ContextFlag, "context", "c", "", "Context to use: dev, staging or prod (default from LFED_CONTEXT or the config file)")
	pf.StringVarP(&f.OutputFlag, "output", "o", "table", "Output format: "+output.Modes)
	pf.StringVar(&f.JQ, "jq", "", "Filter the JSON output with a jq expression (implies -o json)")
	pf.BoolVar(&f.Yes, "yes", false, "Skip confirmation prompts")
	pf.BoolVar(&f.InsecureStorage, "insecure-storage", false, "Store credentials in a plain file instead of the system keychain")

	cmd.AddCommand(
		version.NewCmd(f),
		contextcmd.NewCmd(f),
		authcmd.NewCmd(f),
		plugincmd.NewCmd(f),
	)
	usageArgs(cmd)
	return cmd
}

// usageArgs wraps the Args validator of every command below cmd so a bad
// argument count or an unknown subcommand returns a *cmdutil.UsageError.
func usageArgs(cmd *cobra.Command) {
	for _, c := range cmd.Commands() {
		if validate := c.Args; validate != nil {
			c.Args = func(c *cobra.Command, args []string) error {
				if err := validate(c, args); err != nil {
					return &cmdutil.UsageError{Err: err}
				}
				return nil
			}
		}
		usageArgs(c)
	}
}
