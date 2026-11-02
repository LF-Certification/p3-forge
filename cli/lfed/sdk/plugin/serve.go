package plugin

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

// Serve runs root as an lfed plugin and exits the process. It adds the
// hidden `__metadata` command, a persistent -o/--output flag defaulting to
// LFED_OUTPUT, and fills the manifest's schema version and command list
// from the Cobra tree when the caller left Commands nil. Errors print as
// `lfed-<name>: <err>` and exit 1.
func Serve(root *cobra.Command, m Manifest) {
	os.Exit(run(root, m, os.Args[1:]))
}

// run is Serve without the process exit, for tests.
//
// Under the lfed CLI (LFED_REEXEC set) the tree is mounted below a synthetic
// `lfed` command, so usage lines and errors read `lfed <name> ...`, the way
// the operator typed them. Standalone, the binary is its own root.
func run(root *cobra.Command, m Manifest, args []string) int {
	Prepare(root, m)
	exec, prefix := root, "lfed-"+m.Name
	if os.Getenv("LFED_REEXEC") != "" {
		exec, args = mount(root, m.Name, args)
		prefix = "lfed " + m.Name
	}
	exec.SetArgs(args)
	if err := exec.Execute(); err != nil {
		fmt.Fprintf(root.ErrOrStderr(), "%s: %v\n", prefix, err)
		return 1
	}
	return 0
}

// mount places root under an `lfed` parent named like the plugin and
// rewrites args to address it. Cobra's completion hooks live on the
// outermost command, so `__complete <args>` becomes
// `__complete <name> <args>`.
func mount(root *cobra.Command, name string, args []string) (*cobra.Command, []string) {
	root.Use = name
	if !root.Runnable() {
		// Cobra rejects unknown words only on the outermost command and
		// shows help for a bare non-runnable one. Keep both for the plugin
		// root now that it has a parent.
		if root.Args == nil {
			root.Args = cobra.NoArgs
		}
		root.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	}
	parent := &cobra.Command{Use: "lfed", SilenceUsage: true, SilenceErrors: true}
	parent.SetIn(root.InOrStdin())
	parent.SetOut(root.OutOrStdout())
	parent.SetErr(root.ErrOrStderr())
	parent.AddCommand(root)
	if len(args) > 0 && (args[0] == cobra.ShellCompRequestCmd || args[0] == cobra.ShellCompNoDescRequestCmd) {
		return parent, append([]string{args[0], name}, args[1:]...)
	}
	return parent, append([]string{name}, args...)
}

// Prepare wires root for the plugin contract without executing it. Serve
// calls it; tests call it directly and then root.Execute.
func Prepare(root *cobra.Command, m Manifest) {
	root.SilenceUsage = true
	root.SilenceErrors = true
	if root.Use == "" {
		root.Use = "lfed-" + m.Name
	}
	mode, err := output.ParseMode(os.Getenv("LFED_OUTPUT"))
	if err != nil {
		mode = output.Table
	}
	if root.PersistentFlags().Lookup("output") == nil {
		root.PersistentFlags().StringP("output", "o", string(mode), "Output format: "+output.Modes)
	}
	root.AddCommand(newMetadataCmd(root, m))
}

func newMetadataCmd(root *cobra.Command, m Manifest) *cobra.Command {
	return &cobra.Command{
		Use:    "__metadata",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m.SchemaVersion = ManifestSchemaVersion
			if m.Commands == nil {
				m.Commands = commandsOf(root)
			}
			return output.PrintJSON(cmd.OutOrStdout(), m)
		},
	}
}

// commandsOf lists runnable, visible leaf commands below root. Paths omit
// the root name so `lfed help` can prefix the plugin name.
func commandsOf(root *cobra.Command) []Command {
	var out []Command
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			p := append(append([]string(nil), path...), sub.Name())
			if sub.HasSubCommands() {
				walk(sub, p)
				continue
			}
			if sub.Runnable() {
				out = append(out, Command{Path: p, Summary: sub.Short})
			}
		}
	}
	walk(root, nil)
	if out == nil {
		out = []Command{}
	}
	return out
}

// Output returns the validated -o mode of cmd's tree.
func Output(cmd *cobra.Command) (output.Mode, error) {
	flag := cmd.Flags().Lookup("output")
	if flag == nil {
		return output.Table, errors.New("output flag not registered; call plugin.Serve")
	}
	return output.ParseMode(flag.Value.String())
}
