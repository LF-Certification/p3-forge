package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

func tree() *cobra.Command {
	root := &cobra.Command{Use: "lfed-demo"}
	targets := &cobra.Command{Use: "targets", Short: "Targets"}
	targets.AddCommand(
		&cobra.Command{Use: "list", Short: "List targets", RunE: func(*cobra.Command, []string) error { return nil }},
		&cobra.Command{Use: "secret", Hidden: true, RunE: func(*cobra.Command, []string) error { return nil }},
	)
	root.AddCommand(targets, &cobra.Command{Use: "capabilities", Short: "Caps", RunE: func(*cobra.Command, []string) error { return nil }})
	return root
}

func TestMetadataGeneratesCommands(t *testing.T) {
	root := tree()
	var out bytes.Buffer
	root.SetOut(&out)
	if code := run(context.Background(), root, Manifest{Name: "demo", Version: "0.1.0", RequiresLfed: ">=0.1.0 <1", Summary: "Demo"}, []string{"__metadata"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var m Manifest
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != ManifestSchemaVersion || m.Name != "demo" {
		t.Errorf("manifest %+v", m)
	}
	want := []Command{{Path: []string{"capabilities"}, Summary: "Caps"}, {Path: []string{"targets", "list"}, Summary: "List targets"}}
	got, _ := json.Marshal(m.Commands)
	exp, _ := json.Marshal(want)
	if string(got) != string(exp) {
		t.Errorf("commands %s", got)
	}
}

func TestMetadataKeepsExplicitCommands(t *testing.T) {
	root := tree()
	var out bytes.Buffer
	root.SetOut(&out)
	run(context.Background(), root, Manifest{Name: "demo", Commands: []Command{}}, []string{"__metadata"})
	if !strings.Contains(out.String(), `"commands": []`) {
		t.Errorf("got %s", out.String())
	}
}

func TestForwardedHostFlagsAreAccepted(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "jq", args: []string{"probe", "--jq", ".x"}},
		{name: "jq inline", args: []string{"probe", "--jq=.x"}},
		{name: "context", args: []string{"probe", "--context", "prod"}},
		{name: "yes", args: []string{"probe", "--yes"}},
		{name: "insecure storage", args: []string{"probe", "--insecure-storage"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var seen output.Mode
			root := probeTree(&seen)
			var errOut bytes.Buffer
			root.SetErr(&errOut)
			if code := run(context.Background(), root, Manifest{Name: "demo"}, tc.args); code != 0 {
				t.Fatalf("exit %d: %s", code, errOut.String())
			}
		})
	}
}

func TestForwardedFlagsStayHidden(t *testing.T) {
	root := tree()
	Prepare(root, Manifest{Name: "demo"})
	for _, name := range []string{"context", "jq", "yes", "insecure-storage"} {
		fl := root.PersistentFlags().Lookup(name)
		if fl == nil || !fl.Hidden {
			t.Errorf("flag %q: %+v, want hidden persistent flag", name, fl)
		}
	}
}

func TestLfedYesSetsEveryYesFlag(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want bool
	}{
		{name: "per-command flag", env: "1", args: []string{"apply"}, want: true},
		{name: "per-command flag without LFED_YES", env: "", args: []string{"apply"}, want: false},
		{name: "explicit false wins", env: "1", args: []string{"apply", "--yes=false"}, want: false},
		{name: "root flag", env: "1", args: []string{"probe"}, want: true},
		{name: "root flag without LFED_YES", env: "", args: []string{"probe"}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LFED_YES", tc.env)
			var rootYes, applyYes bool
			root := tree()
			root.PersistentFlags().BoolVar(&rootYes, "yes", false, "Skip prompts")
			root.AddCommand(&cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }})

			plain := tree()
			apply := &cobra.Command{Use: "apply", RunE: func(cmd *cobra.Command, _ []string) error {
				applyYes, _ = cmd.Flags().GetBool("yes")
				return nil
			}}
			apply.Flags().BoolP("yes", "y", false, "Skip prompts")
			plain.AddCommand(apply)

			target, got := root, &rootYes
			if tc.args[0] == "apply" {
				target, got = plain, &applyYes
			}
			if code := run(context.Background(), target, Manifest{Name: "demo"}, tc.args); code != 0 {
				t.Fatalf("exit %d", code)
			}
			if *got != tc.want {
				t.Errorf("yes = %v, want %v", *got, tc.want)
			}
		})
	}
}

func TestFlagErrorExits2(t *testing.T) {
	for _, reexec := range []string{"", "/usr/local/bin/lfed"} {
		t.Setenv("LFED_REEXEC", reexec)
		root := tree()
		var errOut bytes.Buffer
		root.SetErr(&errOut)
		code := run(context.Background(), root, Manifest{Name: "demo"}, []string{"targets", "list", "--bogus"})
		if code != 2 || !strings.Contains(errOut.String(), "unknown flag: --bogus") {
			t.Fatalf("LFED_REEXEC=%q: exit %d stderr %q", reexec, code, errOut.String())
		}
	}
}

func TestCanceledRunExits130(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := tree()
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(&cobra.Command{Use: "wait", RunE: func(cmd *cobra.Command, _ []string) error {
		<-cmd.Context().Done()
		return cmd.Context().Err()
	}})
	if code := run(ctx, root, Manifest{Name: "demo"}, []string{"wait"}); code != 130 {
		t.Fatalf("exit %d, want 130", code)
	}
}

func TestOutputFlagDefaultsFromEnv(t *testing.T) {
	t.Setenv("LFED_OUTPUT", "json")
	var seen output.Mode
	if code := run(context.Background(), probeTree(&seen), Manifest{Name: "demo"}, []string{"probe"}); code != 0 || seen != output.JSON {
		t.Fatalf("env default: exit %d mode %q", code, seen)
	}
	if code := run(context.Background(), probeTree(&seen), Manifest{Name: "demo"}, []string{"probe", "-o", "table"}); code != 0 || seen != output.Table {
		t.Fatalf("flag override: exit %d mode %q", code, seen)
	}
	if code := run(context.Background(), probeTree(&seen), Manifest{Name: "demo"}, []string{"probe", "-o", "xml"}); code != 1 {
		t.Fatalf("invalid mode accepted: exit %d", code)
	}
}

func probeTree(seen *output.Mode) *cobra.Command {
	root := tree()
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(&cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, _ []string) error {
		m, err := Output(cmd)
		*seen = m
		return err
	}})
	return root
}

func TestRunPrintsPrefixedErrorAndExits1(t *testing.T) {
	root := tree()
	var errOut bytes.Buffer
	root.SetErr(&errOut)
	code := run(context.Background(), root, Manifest{Name: "demo"}, []string{"nosuch"})
	if code != 1 || !strings.HasPrefix(errOut.String(), "lfed-demo: ") {
		t.Fatalf("exit %d stderr %q", code, errOut.String())
	}
}

func TestUnderLfedUsageAndErrorsReadLfedName(t *testing.T) {
	t.Setenv("LFED_REEXEC", "/usr/local/bin/lfed")
	root := tree()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	if code := run(context.Background(), root, Manifest{Name: "demo"}, []string{"targets", "list", "--help"}); code != 0 {
		t.Fatalf("help exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "lfed demo targets list") {
		t.Fatalf("usage does not read as typed:\n%s", out.String())
	}
	if strings.Contains(out.String(), "lfed-demo") {
		t.Fatalf("binary name leaked into help:\n%s", out.String())
	}

	root = tree()
	root.SetErr(&errOut)
	errOut.Reset()
	if code := run(context.Background(), root, Manifest{Name: "demo"}, []string{"nosuch"}); code != 1 || !strings.HasPrefix(errOut.String(), "lfed demo: ") {
		t.Fatalf("exit %d stderr %q", code, errOut.String())
	}

	root = tree()
	out.Reset()
	root.SetOut(&out)
	if code := run(context.Background(), root, Manifest{Name: "demo"}, []string{cobra.ShellCompNoDescRequestCmd, "targets", ""}); code != 0 {
		t.Fatalf("completion exit %d", code)
	}
	if !strings.Contains(out.String(), "list") {
		t.Fatalf("completion lost the subtree:\n%s", out.String())
	}
}
