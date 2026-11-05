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

const (
	exitError       = 1
	exitUsage       = 2
	exitInterrupted = 130
)

var CancelGrace = 3 * time.Second

func Serve(root *cobra.Command, m Manifest) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
		}

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

type flagError struct{ err error }

func (e *flagError) Error() string { return e.err.Error() }

func (e *flagError) Unwrap() error { return e.err }

func wrapFlagError(_ *cobra.Command, err error) error { return &flagError{err: err} }

func mount(root *cobra.Command, name string, args []string) (*cobra.Command, []string) {
	root.Use = name
	if !root.Runnable() {

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

func rootDefines(root *cobra.Command, name string) bool {
	return root.PersistentFlags().Lookup(name) != nil || root.Flags().Lookup(name) != nil
}

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

const sdkModule = "github.com/LF-Certification/p3-forge/cli/lfed"

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

func Output(cmd *cobra.Command) (output.Mode, error) {
	flag := cmd.Flags().Lookup("output")
	if flag == nil {
		return output.Table, errors.New("output flag not registered; call plugin.Serve")
	}
	return output.ParseMode(flag.Value.String())
}
