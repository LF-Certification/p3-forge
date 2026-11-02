package pluginmgr

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
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/contexts"
	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
	sdkplugin "github.com/LF-Certification/p3-forge/cli/lfed/sdk/plugin"
)

func setPluginDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LFED_PLUGIN_DIR", dir)
	return dir
}

// writeFake creates <dir>/<name>/lfed-<name> and a manifest without a real
// binary. Tests that only read use it.
func writeFake(t *testing.T, dir, name, summary string) {
	t.Helper()
	pdir := filepath.Join(dir, name)
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, BinaryName(name)), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := sdkplugin.Manifest{SchemaVersion: "1", Name: name, Version: "1.2.3", Summary: summary, RequiresLfed: ">=0.1.0 <1"}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(pdir, ManifestFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func testFactory(t *testing.T, out, errOut *bytes.Buffer) *cmdutil.Factory {
	t.Helper()
	f := &cmdutil.Factory{
		IOStreams:  cmdutil.IOStreams{In: strings.NewReader(""), Out: out, ErrOut: errOut},
		Version:    "0.1.0",
		OutputFlag: "json",
	}
	f.Context = func() (contexts.Context, error) { return contexts.Lookup("staging") }
	f.Token = func(context.Context) (string, error) { return "tok-123", nil }
	f.Executable = func() (string, error) { return "/opt/lfed/bin/lfed", nil }
	return f
}

func TestLookup(t *testing.T) {
	dir := setPluginDir(t)
	writeFake(t, dir, "alpha", "Alpha plugin")

	p, err := Lookup("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "alpha", "lfed-alpha"); p.Binary != want {
		t.Errorf("Binary = %q, want %q", p.Binary, want)
	}
	if p.Manifest.Summary != "Alpha plugin" {
		t.Errorf("Summary = %q", p.Manifest.Summary)
	}

	_, err = Lookup("nosuch")
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("err = %v, want *NotFoundError", err)
	}
	if notFound.Name != "nosuch" {
		t.Errorf("name = %q", notFound.Name)
	}
	if ExitCode(err) != 2 {
		t.Errorf("ExitCode = %d, want 2", ExitCode(err))
	}
}

func TestEnv(t *testing.T) {
	c, err := contexts.Lookup("dev")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(c.Audience, "/") {
		t.Fatalf("test needs an audience with a trailing slash, got %q", c.Audience)
	}
	got := Env("0.1.0", c, "tok", output.JSON, "/usr/bin/lfed")
	want := []string{
		"LFED_CLI_VERSION=0.1.0",
		"LFED_CONTEXT=dev",
		"LFED_API_BASE_URL=" + strings.TrimSuffix(c.Audience, "/"),
		"LFED_TOKEN=tok",
		"LFED_OUTPUT=json",
		"LFED_REEXEC=/usr/bin/lfed",
	}
	if len(got) != len(want) {
		t.Fatalf("Env = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Env[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCheckLfedVersion(t *testing.T) {
	err := CheckLfedVersion(">=2", "0.1.0")
	var incompatible *IncompatibleError
	if !errors.As(err, &incompatible) {
		t.Fatalf("err = %v, want *IncompatibleError", err)
	}
	if !strings.Contains(err.Error(), ">=2") || !strings.Contains(err.Error(), "0.1.0") {
		t.Errorf("message %q lacks both versions", err.Error())
	}
	if ExitCode(err) != 2 {
		t.Errorf("ExitCode = %d, want 2", ExitCode(err))
	}
	if err := CheckLfedVersion(">=0.1.0 <1", "0.1.0"); err != nil {
		t.Errorf("0.1.0 rejected: %v", err)
	}
	if err := CheckLfedVersion(">=0.1.0 <1", "0.1.0-dev"); err != nil {
		t.Errorf("dev build rejected: %v", err)
	}
	if err := CheckLfedVersion("not a range", "0.1.0"); err == nil {
		t.Error("invalid range accepted")
	}
}

func TestInstalled(t *testing.T) {
	dir := setPluginDir(t)
	got, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("List = %v, want none", got)
	}

	writeFake(t, dir, "zeta", "Zeta")
	writeFake(t, dir, "alpha", "Alpha")
	got, err = List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Manifest.Name != "alpha" || got[1].Manifest.Name != "zeta" {
		t.Fatalf("List = %+v, want alpha then zeta", got)
	}
}

func installHello(t *testing.T, f *cmdutil.Factory) Installed {
	t.Helper()
	p, err := Install(context.Background(), f, "./testdata/helloplugin", Options{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	return p
}

func TestInstallFromDirectory(t *testing.T) {
	dir := setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	p := installHello(t, f)

	if want := filepath.Join(dir, "hello", "lfed-hello"); p.Binary != want {
		t.Errorf("Binary = %q, want %q", p.Binary, want)
	}
	info, err := os.Stat(p.Binary)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("binary mode %v is not executable", info.Mode())
	}
	data, err := os.ReadFile(filepath.Join(dir, "hello", ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var m sdkplugin.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "hello" || m.Version != "0.0.1" || m.Summary == "" || len(m.Commands) != 1 {
		t.Errorf("manifest = %+v", m)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".install-") {
			t.Errorf("staging dir %s left behind", e.Name())
		}
	}

	// Reinstall replaces the existing install without error.
	installHello(t, f)
	if _, err := Lookup("hello"); err != nil {
		t.Fatal(err)
	}

	if err := Remove("hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := Lookup("hello"); err == nil {
		t.Error("plugin still present after Remove")
	}
}

func TestInstallRejectsIncompatibleHost(t *testing.T) {
	setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	f.Version = "5.0.0"
	_, err := Install(context.Background(), f, "./testdata/helloplugin", Options{})
	var incompatible *IncompatibleError
	if !errors.As(err, &incompatible) {
		t.Fatalf("err = %v, want *IncompatibleError", err)
	}
	if incompatible.Name != "hello" {
		t.Errorf("Name = %q, want hello", incompatible.Name)
	}
}

func TestExec(t *testing.T) {
	setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	p := installHello(t, f)

	f.Yes = true
	if err := Exec(context.Background(), f, p, []string{"greet", "world"}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	for _, want := range []string{
		"LFED_CLI_VERSION=0.1.0\n",
		"LFED_CONTEXT=staging\n",
		"LFED_API_BASE_URL=https://api.staging.ed.linuxfoundation.org\n",
		"LFED_TOKEN=tok-123\n",
		"LFED_OUTPUT=json\n",
		"LFED_REEXEC=/opt/lfed/bin/lfed\n",
		"args: greet world --yes\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	err := Exec(context.Background(), f, p, []string{"fail"})
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != 3 {
		t.Fatalf("err = %v, want ExitError{3}", err)
	}
	if ExitCode(err) != 3 {
		t.Errorf("ExitCode = %d, want 3", ExitCode(err))
	}

	// Run checks requires_lfed before executing.
	f.Version = "9.0.0"
	err = Run(context.Background(), f, "hello", nil)
	if ExitCode(err) != 2 {
		t.Errorf("Run with lfed CLI 9.0.0: err = %v, want exit 2", err)
	}
}

func TestCompletePassesThrough(t *testing.T) {
	setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	installHello(t, f)
	if err := Complete(context.Background(), f, "hello", []string{"gr", ""}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "args: __complete gr \n") {
		t.Errorf("output = %q", out.String())
	}
}

func TestExecLoggedOutExportsEmptyToken(t *testing.T) {
	setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	p := installHello(t, f)
	f.Token = func(context.Context) (string, error) { return "", errors.New("not logged in") }
	if err := Exec(context.Background(), f, p, []string{"greet", "world"}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.Contains(out.String(), "LFED_TOKEN=\n") {
		t.Errorf("output lacks empty LFED_TOKEN:\n%s", out.String())
	}
}

func TestCompleteNeverResolvesToken(t *testing.T) {
	setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	installHello(t, f)
	f.Token = func(context.Context) (string, error) {
		t.Fatal("Complete resolved the token")
		return "", nil
	}
	if err := Complete(context.Background(), f, "hello", []string{"gr", ""}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "LFED_TOKEN=\n") {
		t.Errorf("output lacks empty LFED_TOKEN:\n%s", out.String())
	}
}
