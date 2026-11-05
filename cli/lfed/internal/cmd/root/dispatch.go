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

type dispatch struct {
	name     string
	rest     []string
	complete bool
}

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

				return dispatch{}, false, nil
			}
			return dispatch{}, false, &cmdutil.UsageError{Err: fmt.Errorf("unknown flag: %s", a)}
		}
		if err := setFlag(fl, value); err != nil {
			return dispatch{}, false, &cmdutil.UsageError{Err: err}
		}
		i += n
	}
	if i >= len(args) {
		return dispatch{}, false, nil
	}
	word := args[i]
	switch word {
	case cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:

		if i+2 >= len(args) || isCore(cmd, args[i+1]) {
			return dispatch{}, false, nil
		}
		return dispatch{name: args[i+1], rest: args[i+2:], complete: true}, true, nil
	case "help":

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

func isCobraFlag(a string) bool {
	switch strings.SplitN(a, "=", 2)[0] {
	case "-h", "--help", "-v", "--version":
		return true
	}
	return false
}

func definedFlag(pf *pflag.FlagSet, a string) bool {
	name := strings.SplitN(a, "=", 2)[0]
	if strings.HasPrefix(name, "--") {
		return pf.Lookup(name[2:]) != nil
	}
	return len(name) >= 2 && pf.ShorthandLookup(name[1:2]) != nil
}

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

func applyLfedFlags(pf *pflag.FlagSet, args []string) error {
	for i := 0; i < len(args); {
		if args[i] == "--" {
			return nil
		}
		if !strings.HasPrefix(args[i], "--") {
			i++
			continue
		}
		fl, value, n, known := hostFlag(pf, args, i)
		if !known {
			i++
			continue
		}
		if err := setFlag(fl, value); err != nil {
			return &cmdutil.UsageError{Err: err}
		}
		i += n
	}
	return nil
}

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
