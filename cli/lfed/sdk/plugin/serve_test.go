package plugin

import (
	"bytes"
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
	if code := run(root, Manifest{Name: "demo", Version: "0.1.0", RequiresLfed: ">=0.1.0 <1", Summary: "Demo"}, []string{"__metadata"}); code != 0 {
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
	run(root, Manifest{Name: "demo", Commands: []Command{}}, []string{"__metadata"})
	if !strings.Contains(out.String(), `"commands": []`) {
		t.Errorf("got %s", out.String())
	}
}
func TestOutputFlagDefaultsFromEnv(t *testing.T) {
	t.Setenv("LFED_OUTPUT", "json")
	var seen output.Mode
	if code := run(probeTree(&seen), Manifest{Name: "demo"}, []string{"probe"}); code != 0 || seen != output.JSON {
		t.Fatalf("env default: exit %d mode %q", code, seen)
	}
	if code := run(probeTree(&seen), Manifest{Name: "demo"}, []string{"probe", "-o", "table"}); code != 0 || seen != output.Table {
		t.Fatalf("flag override: exit %d mode %q", code, seen)
	}
	if code := run(probeTree(&seen), Manifest{Name: "demo"}, []string{"probe", "-o", "xml"}); code != 1 {
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
	code := run(root, Manifest{Name: "demo"}, []string{"nosuch"})
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
	if code := run(root, Manifest{Name: "demo"}, []string{"targets", "list", "--help"}); code != 0 {
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
	if code := run(root, Manifest{Name: "demo"}, []string{"nosuch"}); code != 1 || !strings.HasPrefix(errOut.String(), "lfed demo: ") {
		t.Fatalf("exit %d stderr %q", code, errOut.String())
	}

	// Completion requests from the CLI omit the plugin name; mount puts it back.
	root = tree()
	out.Reset()
	root.SetOut(&out)
	if code := run(root, Manifest{Name: "demo"}, []string{cobra.ShellCompNoDescRequestCmd, "targets", ""}); code != 0 {
		t.Fatalf("completion exit %d", code)
	}
	if !strings.Contains(out.String(), "list") {
		t.Fatalf("completion lost the subtree:\n%s", out.String())
	}
}
