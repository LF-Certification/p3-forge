package root

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/jq"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/pluginmgr"
)

// Run executes argv for the lfed CLI.
//
// It pre-scans args before Cobra sees them. CLI flags before the first
// word are recognised by the root persistent flag set; the first word that
// is not a flag is the command. A core command, "help" of a core command,
// "completion" or the Cobra completion hooks go to Cobra unchanged. Any
// other word is a plugin name: the remaining args are forwarded raw, after
// CLI flags in them (-c, -o, --jq, --yes, --insecure-storage) are also
// applied to the lfed CLI. Cobra would otherwise strip or reject flags it
// does not own, so the plugin would not see its own flags.
//
// Stdout is wrapped in an OutputCapture so --jq can filter the JSON the
// command or plugin printed once it has finished.
func Run(ctx context.Context, f *cmdutil.Factory, args []string) error {
	capture := &cmdutil.OutputCapture{Real: f.IOStreams.Out}
	f.IOStreams.Out = capture
	f.Capture = capture
	err := run(ctx, f, args)
	data, captured := capture.Stop()
	if !captured {
		return err
	}
	if err != nil {
		// The command failed after printing; show what it printed.
		_, _ = capture.Real.Write(data)
		return err
	}
	return jq.Filter(ctx, capture.Real, f.JQ, data)
}

func run(ctx context.Context, f *cmdutil.Factory, args []string) error {
	cmd := NewCmd(f)
	d, ok, err := split(cmd, args)
	if err != nil {
		return err
	}
	if !ok {
		cmd.SetArgs(args)
		return cmd.ExecuteContext(ctx)
	}
	if err := applyLfedFlags(cmd.PersistentFlags(), d.rest); err != nil {
		return err
	}
	if d.complete {
		return pluginmgr.Complete(ctx, f, d.name, d.rest)
	}
	if f.JQ != "" {
		f.Capture.Start()
	}
	err = pluginmgr.Run(ctx, f, d.name, d.rest)
	var notFound *pluginmgr.NotFoundError
	if errors.As(err, &notFound) {
		notFound.Suggestions = suggest(d.name, commandWords(cmd))
	}
	return err
}

// dispatch is a resolved plugin invocation.
type dispatch struct {
	name     string
	rest     []string
	complete bool
}

// split finds the plugin invocation in args. ok is false when Cobra must
// handle args: a core command, help for one, no command word, or a flag
// Cobra itself defines (--help, --version). A flag nobody defines is an
// error here: left to Cobra it would be read as a flag with a value and
// turn into a misleading "unknown command". CLI flags before the command
// word are applied.
func split(cmd *cobra.Command, args []string) (d dispatch, ok bool, err error) {
	pf := cmd.PersistentFlags()
	i := 0
	for i < len(args) {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			break
		}
		fl, value, n, known := hostFlag(pf, args, i)
		if !known {
			if isCobraFlag(a) || definedFlag(pf, a) {
				// --help/--version, or a CLI flag missing its value:
				// Cobra reports both correctly.
				return dispatch{}, false, nil
			}
			return dispatch{}, false, fmt.Errorf("unknown flag: %s", a)
		}
		if err := setFlag(fl, value); err != nil {
			return dispatch{}, false, err
		}
		i += n
	}
	if i >= len(args) {
		return dispatch{}, false, nil
	}
	word := args[i]
	switch word {
	case cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		// The last arg is the word being completed. A plugin name still
		// being typed is completed by Cobra from the root ValidArgsFunction.
		if i+2 >= len(args) || isCore(cmd, args[i+1]) {
			return dispatch{}, false, nil
		}
		return dispatch{name: args[i+1], rest: args[i+2:], complete: true}, true, nil
	case "help":
		// `lfed help <plugin> [sub...]` is `lfed <plugin> [sub...] --help`.
		if i+1 < len(args) && !isCore(cmd, args[i+1]) && !strings.HasPrefix(args[i+1], "-") {
			return dispatch{name: args[i+1], rest: append(slices.Clone(args[i+2:]), "--help")}, true, nil
		}
		return dispatch{}, false, nil
	}
	if isCore(cmd, word) {
		return dispatch{}, false, nil
	}
	return dispatch{name: word, rest: args[i+1:]}, true, nil
}

// isCobraFlag reports the flags Cobra adds to the root itself.
func isCobraFlag(a string) bool {
	switch strings.SplitN(a, "=", 2)[0] {
	case "-h", "--help", "-v", "--version":
		return true
	}
	return false
}

// definedFlag reports whether a names a flag in pf, regardless of value.
func definedFlag(pf *pflag.FlagSet, a string) bool {
	name := strings.SplitN(a, "=", 2)[0]
	if strings.HasPrefix(name, "--") {
		return pf.Lookup(name[2:]) != nil
	}
	return len(name) >= 2 && pf.ShorthandLookup(name[1:2]) != nil
}

// isCore reports whether word names a core command, an alias, or a Cobra
// built-in. Cobra adds help and completion only inside Execute, so they
// are listed here.
func isCore(cmd *cobra.Command, word string) bool {
	switch word {
	case "help", "completion":
		return true
	}
	for _, c := range cmd.Commands() {
		if c.Name() == word || c.HasAlias(word) {
			return true
		}
	}
	return false
}

// commandWords lists every word that can follow `lfed`: core commands and
// installed plugins.
func commandWords(cmd *cobra.Command) []string {
	var words []string
	for _, c := range cmd.Commands() {
		if !c.Hidden {
			words = append(words, c.Name())
		}
	}
	installed, err := pluginmgr.List()
	if err == nil {
		for _, p := range installed {
			words = append(words, p.Manifest.Name)
		}
	}
	return words
}

// suggest returns the candidates close to word: sharing a prefix of two
// or more characters, or within an edit distance of two. Sorted for
// stable messages.
func suggest(word string, candidates []string) []string {
	var out []string
	lower := strings.ToLower(word)
	for _, c := range candidates {
		lc := strings.ToLower(c)
		if (len(lower) >= 2 && strings.HasPrefix(lc, lower)) || editDistance(lower, lc) <= 2 {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// applyLfedFlags sets CLI flags that appear among plugin args. The args
// stay unchanged so the plugin also receives them. Words the lfed CLI does
// not own are skipped.
func applyLfedFlags(pf *pflag.FlagSet, args []string) error {
	for i := 0; i < len(args); {
		if args[i] == "--" {
			return nil
		}
		fl, value, n, known := hostFlag(pf, args, i)
		if !known {
			i++
			continue
		}
		if err := setFlag(fl, value); err != nil {
			return err
		}
		i += n
	}
	return nil
}

// hostFlag parses the flag at args[i] against pf. n is the number of args
// consumed. known is false for non-flags, flags pf does not define, and
// value flags with no value left.
func hostFlag(pf *pflag.FlagSet, args []string, i int) (fl *pflag.Flag, value string, n int, known bool) {
	a := args[i]
	if !strings.HasPrefix(a, "-") || a == "-" || a == "--" {
		return nil, "", 0, false
	}
	var inline *string
	if strings.HasPrefix(a, "--") {
		name := a[2:]
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			v := name[eq+1:]
			inline = &v
			name = name[:eq]
		}
		fl = pf.Lookup(name)
	} else {
		fl = pf.ShorthandLookup(a[1:2])
		if len(a) > 2 {
			v := strings.TrimPrefix(a[2:], "=")
			inline = &v
		}
	}
	if fl == nil {
		return nil, "", 0, false
	}
	switch {
	case inline != nil:
		return fl, *inline, 1, true
	case fl.NoOptDefVal != "":
		return fl, fl.NoOptDefVal, 1, true
	case i+1 < len(args):
		return fl, args[i+1], 2, true
	}
	return nil, "", 0, false
}

func setFlag(fl *pflag.Flag, value string) error {
	if err := fl.Value.Set(value); err != nil {
		return err
	}
	fl.Changed = true
	return nil
}
