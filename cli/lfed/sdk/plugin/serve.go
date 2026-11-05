package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

// Exit codes of Serve.
const (
	exitError       = 1
	exitUsage       = 2
	exitInterrupted = 130
)

// CancelGrace is how long Serve waits for the command to return after
// SIGINT or SIGTERM cancelled its context. A command that does not watch
// the context, for example one blocked in a prompt read, is then ended
// with exit 130. The timer starts at the first signal; a second signal
// ends the process at once with the default action.
var CancelGrace = 3 * time.Second

// Serve runs root as an lfed plugin and exits the process. It adds the
// hidden `__metadata` command, a persistent -o/--output flag defaulting to
// LFED_OUTPUT and the hidden pass-through flags of the lfed CLI (see
// Prepare). It fills the manifest's schema version, SDK version and command
// list from the Cobra tree when the caller left Commands nil. Errors print
// as `lfed-<name>: <err>`. A flag error exits 2, an interrupt (SIGINT or
// SIGTERM) exits 130, other errors exit 1.
func Serve(root *cobra.Command, m Manifest) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
		}
		// A second signal kills the process with the default action.
		stop()
		select {
		case <-done:
		case <-time.After(CancelGrace):
			os.Exit(exitInterrupted)
		}
	}()
	code := run(ctx, root, m, os.Args[1:])
	close(done)
	stop()
	os.Exit(code)
}

// run is Serve without the process exit, for tests.
//
// Under the lfed CLI (LFED_REEXEC set) the tree is mounted below a synthetic
// `lfed` command, so usage lines and errors read `lfed <name> ...`, the way
// the operator typed them. Standalone, the binary is its own root.
func run(ctx context.Context, root *cobra.Command, m Manifest, args []string) int {
	Prepare(root, m)
	exec, prefix := root, "lfed-"+m.Name
	if os.Getenv("LFED_REEXEC") != "" {
		exec, args = mount(root, m.Name, args)
		prefix = "lfed " + m.Name
	}
	exec.SetArgs(args)
	err := exec.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return exitInterrupted
	}
	fmt.Fprintf(root.ErrOrStderr(), "%s: %v\n", prefix, err)
	var usage *flagError
	if errors.As(err, &usage) {
		return exitUsage
	}
	if ctx.Err() != nil {
		return exitInterrupted
	}
	return exitError
}

// flagError is a flag parse error. run maps it to exit code 2.
type flagError struct{ err error }

func (e *flagError) Error() string { return e.err.Error() }

func (e *flagError) Unwrap() error { return e.err }

func wrapFlagError(_ *cobra.Command, err error) error { return &flagError{err: err} }

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
	parent.SetFlagErrorFunc(wrapFlagError)
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
//
// The lfed CLI forwards the plugin args unchanged, so they can contain its
// long-form flags (--context, --jq, --yes, --insecure-storage). The CLI
// already applied them and exports the result as LFED_* variables. Prepare
// registers each flag as a hidden persistent flag when root does not define
// it, so the plugin parser accepts it. When LFED_YES is set, Prepare sets
// every bool `yes` flag in the tree to true, on root and on each command.
func Prepare(root *cobra.Command, m Manifest) {
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.SetFlagErrorFunc(wrapFlagError)
	if root.Use == "" {
		root.Use = "lfed-" + m.Name
	}
	mode, err := output.ParseMode(os.Getenv("LFED_OUTPUT"))
	if err != nil {
		mode = output.Table
	}
	pf := root.PersistentFlags()
	if pf.Lookup("output") == nil {
		pf.StringP("output", "o", string(mode), "Output format: "+output.Modes)
	}
	if !rootDefines(root, "context") {
		pf.String("context", "", "Context of the lfed CLI (read LFED_CONTEXT)")
		_ = pf.MarkHidden("context")
	}
	if !rootDefines(root, "jq") {
		pf.String("jq", "", "jq filter of the lfed CLI (the CLI applies it)")
		_ = pf.MarkHidden("jq")
	}
	if !rootDefines(root, "insecure-storage") {
		pf.Bool("insecure-storage", false, "Credential storage of the lfed CLI (read LFED_INSECURE_STORAGE)")
		_ = pf.MarkHidden("insecure-storage")
	}
	if !rootDefines(root, "yes") {
		pf.Bool("yes", false, "Skip confirmation prompts (read LFED_YES)")
		_ = pf.MarkHidden("yes")
	}
	if os.Getenv("LFED_YES") != "" {
		setYes(root)
	}
	root.AddCommand(newMetadataCmd(root, m))
}

// rootDefines reports whether root defines the flag name, persistent or
// local.
func rootDefines(root *cobra.Command, name string) bool {
	return root.PersistentFlags().Lookup(name) != nil || root.Flags().Lookup(name) != nil
}

// setYes sets the value of every bool `yes` flag in the tree of c to true.
// Explicit flags on the command line still apply after it.
func setYes(c *cobra.Command) {
	for _, fs := range []*pflag.FlagSet{c.PersistentFlags(), c.Flags()} {
		if fl := fs.Lookup("yes"); fl != nil && fl.Value.Type() == "bool" {
			_ = fl.Value.Set("true")
		}
	}
	for _, sub := range c.Commands() {
		setYes(sub)
	}
}

func newMetadataCmd(root *cobra.Command, m Manifest) *cobra.Command {
	return &cobra.Command{
		Use:    "__metadata",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m.SchemaVersion = ManifestSchemaVersion
			if m.SDKVersion == "" {
				m.SDKVersion = sdkVersion()
			}
			if m.Commands == nil {
				m.Commands = commandsOf(root)
			}
			return output.PrintJSON(cmd.OutOrStdout(), m)
		},
	}
}

// sdkVersion returns the version of the lfed module the plugin binary was
// built with. It is empty when the build used a go.work workspace or a
// replace directive, or has no build information.
func sdkVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path != sdkModule {
			continue
		}
		if dep.Replace != nil || dep.Version == "(devel)" {
			return ""
		}
		return dep.Version
	}
	return ""
}

// sdkModule is the module path of the lfed CLI and its SDK.
const sdkModule = "github.com/LF-Certification/p3-forge/cli/lfed"

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
