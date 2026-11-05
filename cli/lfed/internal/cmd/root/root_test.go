package root

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/config"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/contexts"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/pluginmgr"
	sdkplugin "github.com/LF-Certification/p3-forge/cli/lfed/sdk/plugin"
)

func newFactory(out *bytes.Buffer) *cmdutil.Factory {
	return &cmdutil.Factory{
		IOStreams: cmdutil.IOStreams{In: strings.NewReader(""), Out: out, ErrOut: out},
		Version:   "0.1.0",
		Context:   func() (contexts.Context, error) { return contexts.Context{}, contexts.ErrNoContext },
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		ok       bool
		plugin   string
		rest     []string
		complete bool
		output   string
		context  string
		err      string
	}{
		{name: "no args", args: nil},
		{name: "core command", args: []string{"version"}},
		{name: "core after CLI flags", args: []string{"-o", "json", "context", "list"}},
		{name: "help", args: []string{"help"}},
		{name: "unknown flag before the command is an error", args: []string{"--nope", "placement"}, err: "unknown flag: --nope"},
		{name: "unknown flag alone is an error", args: []string{"--nope", "context", "list"}, err: "unknown flag: --nope"},
		{name: "help flag goes to cobra", args: []string{"-h"}},
		{name: "version flag goes to cobra", args: []string{"--version"}},
		{name: "CLI flag missing its value goes to cobra", args: []string{"-c"}},
		{name: "help of a core command goes to cobra", args: []string{"help", "context"}},
		{
			name: "help of a plugin becomes --help",
			args: []string{"help", "placement", "targets"},
			ok:   true, plugin: "placement", rest: []string{"targets", "--help"}, output: "table",
		},
		{name: "complete core", args: []string{"__complete", "context", ""}},
		{name: "complete plugin name goes to cobra", args: []string{"__complete", "pla"}},
		{
			name: "plugin with flags before and after",
			args: []string{"-o", "json", "placement", "targets", "list", "--context=dev", "--limit", "5"},
			ok:   true, plugin: "placement",
			rest:   []string{"targets", "list", "--context=dev", "--limit", "5"},
			output: "json", context: "dev",
		},
		{
			name: "short inline flag before the plugin",
			args: []string{"-ojson", "-cprod", "placement", "targets"},
			ok:   true, plugin: "placement", rest: []string{"targets"},
			output: "json", context: "prod",
		},
		{
			name: "shorthands after the plugin belong to the plugin",
			args: []string{"placement", "-c", "prod", "-o", "json", "--jq=.x"},
			ok:   true, plugin: "placement", rest: []string{"-c", "prod", "-o", "json", "--jq=.x"},
			output: "table",
		},
		{
			name: "long flags after the plugin apply",
			args: []string{"placement", "targets", "--output", "json", "--context", "staging"},
			ok:   true, plugin: "placement", rest: []string{"targets", "--output", "json", "--context", "staging"},
			output: "json", context: "staging",
		},
		{
			name: "flags after -- belong to the plugin",
			args: []string{"placement", "exec", "--", "--context=prod"},
			ok:   true, plugin: "placement", rest: []string{"exec", "--", "--context=prod"},
			output: "table",
		},
		{
			name: "complete plugin",
			args: []string{"__complete", "placement", "targets", ""},
			ok:   true, plugin: "placement", rest: []string{"targets", ""}, complete: true,
			output: "table",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			f := newFactory(&out)
			cmd := NewCmd(f)
			d, ok, err := split(cmd, tc.args)
			if (err != nil) != (tc.err != "") || (err != nil && err.Error() != tc.err) {
				t.Fatalf("err = %v, want %q", err, tc.err)
			}
			var usage *cmdutil.UsageError
			if err != nil && !errors.As(err, &usage) {
				t.Fatalf("err = %T, want *cmdutil.UsageError", err)
			}
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (dispatch %+v)", ok, tc.ok, d)
			}
			if !ok {
				return
			}
			if err := applyLfedFlags(cmd.PersistentFlags(), d.rest); err != nil {
				t.Fatal(err)
			}
			if d.name != tc.plugin || d.complete != tc.complete || strings.Join(d.rest, "\x00") != strings.Join(tc.rest, "\x00") {
				t.Errorf("dispatch = %+v, want name %q rest %q complete %v", d, tc.plugin, tc.rest, tc.complete)
			}
			if f.OutputFlag != tc.output {
				t.Errorf("OutputFlag = %q, want %q", f.OutputFlag, tc.output)
			}
			if f.ContextFlag != tc.context {
				t.Errorf("ContextFlag = %q, want %q", f.ContextFlag, tc.context)
			}
		})
	}
}

func TestRunUnknownCommandExits2(t *testing.T) {
	t.Setenv("LFED_PLUGIN_DIR", t.TempDir())
	var out bytes.Buffer
	err := Run(context.Background(), newFactory(&out), []string{"nosuch", "--flag"})
	if err == nil || err.Error() != `unknown command "nosuch" for "lfed"; run "lfed plugin list"` {
		t.Fatalf("err = %v", err)
	}
	if pluginmgr.ExitCode(err) != 2 {
		t.Errorf("exit = %d, want 2", pluginmgr.ExitCode(err))
	}

	err = Run(context.Background(), newFactory(&out), []string{"contxt"})
	if err == nil || !strings.Contains(err.Error(), "did you mean context?") {
		t.Fatalf("suggestion: %v", err)
	}
}

func TestUsageErrorsExit2(t *testing.T) {
	t.Setenv("LFED_PLUGIN_DIR", t.TempDir())
	tests := [][]string{
		{"--bogus"},
		{"context", "nosuchcmd"},
		{"context", "list", "--bogus"},
		{"context", "use"},
		{"-c"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out bytes.Buffer
			err := Run(context.Background(), newFactory(&out), args)
			var usage *cmdutil.UsageError
			if !errors.As(err, &usage) {
				t.Fatalf("err = %v (%T), want *cmdutil.UsageError", err, err)
			}
			if code := pluginmgr.ExitCode(err); code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
		})
	}
}

func TestVersionFlagMatchesVersionCommand(t *testing.T) {
	var flag, cmd bytes.Buffer
	if err := Run(context.Background(), newFactory(&flag), []string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), newFactory(&cmd), []string{"version"}); err != nil {
		t.Fatal(err)
	}
	if flag.String() != cmd.String() || !strings.HasPrefix(flag.String(), "lfed ") {
		t.Fatalf("--version %q, version %q", flag.String(), cmd.String())
	}
}

func TestJQFiltersCoreCommandOutput(t *testing.T) {
	var out bytes.Buffer
	if err := Run(context.Background(), newFactory(&out), []string{"context", "list", "--jq", ".[].name"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "dev\nstaging\nprod\n" {
		t.Fatalf("got %q", out.String())
	}
	out.Reset()
	err := Run(context.Background(), newFactory(&out), []string{"-o", "yaml", "--jq", ".", "context", "list"})
	if err == nil || !strings.Contains(err.Error(), "--jq needs JSON") {
		t.Fatalf("yaml with jq: %v", err)
	}
}

func TestContextCommandsHonourOutput(t *testing.T) {
	t.Setenv("LFED_CONFIG_DIR", t.TempDir())
	dev, err := contexts.Lookup("dev")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "current table", args: []string{"context", "current"}, want: "dev\n"},
		{name: "current json", args: []string{"context", "current", "-o", "json"}, want: "{\n  \"name\": \"dev\"\n}\n"},
		{name: "current jq", args: []string{"context", "current", "--jq", ".name"}, want: "dev\n"},
		{name: "use table prints nothing on stdout", args: []string{"context", "use", "staging"}, want: ""},
		{name: "use json", args: []string{"-o", "json", "context", "use", "prod"}, want: "{\n  \"current_context\": \"prod\"\n}\n"},
		{name: "use jq", args: []string{"context", "use", "dev", "--jq", ".current_context"}, want: "dev\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			f := newFactory(&out)
			f.IOStreams.ErrOut = &errOut
			f.Config = config.Load
			f.Context = func() (contexts.Context, error) { return dev, nil }
			if err := Run(context.Background(), f, tc.args); err != nil {
				t.Fatalf("%v: %s", err, errOut.String())
			}
			if out.String() != tc.want {
				t.Errorf("stdout = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestHelpListsPlugins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFED_PLUGIN_DIR", dir)
	for _, p := range []struct{ name, summary string }{
		{"placement", "Manage placement reference data"},
		{"secret", "Hidden plugin"},
	} {
		pdir := filepath.Join(dir, p.name)
		if err := os.MkdirAll(pdir, 0o755); err != nil {
			t.Fatal(err)
		}
		m := sdkplugin.Manifest{SchemaVersion: "1", Name: p.name, Version: "1.0.0", Summary: p.summary, RequiresLfed: ">=0.1.0 <1", Hidden: p.name == "secret"}
		data, _ := json.Marshal(m)
		if err := os.WriteFile(filepath.Join(pdir, pluginmgr.ManifestFile), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	if err := Run(context.Background(), newFactory(&out), []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	cmds := strings.Index(help, "Available Commands:")
	plugins := strings.Index(help, "Plugins:\n  placement")
	flags := strings.Index(help, "Flags:")
	if cmds < 0 || plugins < 0 || flags < 0 || !(cmds < plugins && plugins < flags) {
		t.Fatalf("help sections out of order or missing:\n%s", help)
	}
	if !strings.Contains(help, "Manage placement reference data") {
		t.Errorf("summary missing:\n%s", help)
	}
	if strings.Contains(help, "secret") {
		t.Errorf("hidden plugin listed:\n%s", help)
	}

	out.Reset()
	if err := Run(context.Background(), newFactory(&out), []string{"context", "--help"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Plugins:") {
		t.Errorf("subcommand help lists plugins:\n%s", out.String())
	}
}
