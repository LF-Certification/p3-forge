package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/pluginmgr"
	sdkplugin "github.com/LF-Certification/p3-forge/cli/lfed/sdk/plugin"
)

func writeFake(t *testing.T, dir, name, version, summary string) {
	t.Helper()
	writeFakeRequiring(t, dir, name, version, summary, ">=0.1.0 <1")
}

// writeFakeRequiring writes a plugin whose manifest pins requires_lfed.
func writeFakeRequiring(t *testing.T, dir, name, version, summary, requires string) {
	t.Helper()
	pdir := filepath.Join(dir, name)
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, pluginmgr.BinaryName(name)), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := sdkplugin.Manifest{SchemaVersion: "1", Name: name, Version: version, Summary: summary, RequiresLfed: requires}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(pdir, pluginmgr.ManifestFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, outputFlag string, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	f := &cmdutil.Factory{
		IOStreams:  cmdutil.IOStreams{In: strings.NewReader(""), Out: &out, ErrOut: &out},
		Version:    "0.1.0",
		OutputFlag: outputFlag,
	}
	cmd := NewCmd(f)
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return out.String()
}

func TestListEmpty(t *testing.T) {
	t.Setenv("LFED_PLUGIN_DIR", t.TempDir())
	got := run(t, "table", "list")
	if strings.TrimSpace(got) != "NAME  VERSION  STATUS  SOURCE  SUMMARY" {
		t.Errorf("table = %q", got)
	}
	got = run(t, "json", "list")
	if strings.TrimSpace(got) != "[]" {
		t.Errorf("json = %q", got)
	}
}

func TestListTwo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFED_PLUGIN_DIR", dir)
	writeFakeRequiring(t, dir, "placement", "0.3.0", "Placement reference data", ">=9.0.0")
	writeFake(t, dir, "hello", "0.0.1", "Say hello")

	got := run(t, "table", "list")
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 3 {
		t.Fatalf("table = %q", got)
	}
	if !strings.HasPrefix(lines[1], "hello ") || !strings.Contains(lines[1], "0.0.1") || !strings.Contains(lines[1], " ok ") || !strings.Contains(lines[1], "Say hello") {
		t.Errorf("row 1 = %q", lines[1])
	}
	// An incompatible plugin stays listed and shows what it needs.
	if !strings.HasPrefix(lines[2], "placement ") || !strings.Contains(lines[2], "needs lfed >=9.0.0") {
		t.Errorf("row 2 = %q", lines[2])
	}

	var rows []struct {
		sdkplugin.Manifest
		Compatible bool `json:"compatible"`
	}
	if err := json.Unmarshal([]byte(run(t, "json", "list")), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Name != "hello" || !rows[0].Compatible || rows[1].RequiresLfed != ">=9.0.0" || rows[1].Compatible {
		t.Errorf("json = %+v", rows)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFED_PLUGIN_DIR", dir)
	writeFake(t, dir, "hello", "0.0.1", "Say hello")

	got := run(t, "table", "remove", "hello")
	if strings.TrimSpace(got) != "removed hello" {
		t.Errorf("output = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "hello")); !os.IsNotExist(err) {
		t.Errorf("plugin dir still exists: %v", err)
	}

	var out bytes.Buffer
	f := &cmdutil.Factory{IOStreams: cmdutil.IOStreams{Out: &out, ErrOut: &out}, Version: "0.1.0"}
	cmd := NewCmd(f)
	cmd.SetArgs([]string{"remove", "nosuch"})
	err := cmd.ExecuteContext(context.Background())
	if pluginmgr.ExitCode(err) != 2 {
		t.Errorf("remove nosuch: err = %v, want exit 2", err)
	}
}
