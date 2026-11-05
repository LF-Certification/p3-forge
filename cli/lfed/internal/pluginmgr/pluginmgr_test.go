package pluginmgr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

var badNames = []string{"..", "../..", "a/../..", "/", "foo/..", "-x", "Foo", ".", "", "a/b", `a\b`, ".install-x", ".prev-hello-x"}

func TestLookupRejectsBadNames(t *testing.T) {
	dir := setPluginDir(t)
	writeFake(t, dir, "alpha", "Alpha")
	for _, name := range badNames {
		_, err := Lookup(name)
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("Lookup(%q): err = %v, want *NotFoundError", name, err)
		}
	}
}

func TestLookupRejectsOtherSchemaVersion(t *testing.T) {
	dir := setPluginDir(t)
	writeFake(t, dir, "alpha", "Alpha")
	m := sdkplugin.Manifest{SchemaVersion: "0", Name: "alpha", Version: "1.2.3", Summary: "Alpha", RequiresLfed: ">=0.1.0 <1"}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "alpha", ManifestFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Lookup("alpha")
	if err == nil || !strings.Contains(err.Error(), "install\" again") {
		t.Fatalf("err = %v, want a reinstall hint", err)
	}
}

func TestRemove(t *testing.T) {

	root := t.TempDir()
	dir := filepath.Join(root, "a", "plugins")
	t.Setenv("LFED_PLUGIN_DIR", dir)
	writeFake(t, dir, "alpha", "Alpha")
	canary := filepath.Join(root, "canary")
	if err := os.WriteFile(canary, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, name := range badNames {
		err := Remove(name)
		var notFound *NotFoundError
		if !errors.As(err, &notFound) || ExitCode(err) != 2 {
			t.Errorf("Remove(%q): err = %v, want *NotFoundError", name, err)
		}
	}
	for _, p := range []string{canary, filepath.Join(dir, "alpha", ManifestFile)} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s removed by a bad name: %v", p, err)
		}
	}

	if err := os.MkdirAll(filepath.Join(dir, "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "broken"} {
		if err := Remove(name); err != nil {
			t.Fatalf("Remove(%q): %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still present: %v", name, err)
		}
	}
	var notFound *NotFoundError
	if err := Remove("alpha"); !errors.As(err, &notFound) {
		t.Errorf("second Remove: err = %v, want *NotFoundError", err)
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
	base := []string{
		"LFED_CLI_VERSION=0.1.0",
		"LFED_CONTEXT=dev",
		"LFED_API_BASE_URL=" + strings.TrimSuffix(c.Audience, "/"),
		"LFED_OUTPUT=json",
		"LFED_REEXEC=/usr/bin/lfed",
	}
	cases := []struct {
		yes, insecure bool
		want          []string
	}{
		{false, false, []string{"LFED_YES=", "LFED_INSECURE_STORAGE="}},
		{true, true, []string{"LFED_YES=1", "LFED_INSECURE_STORAGE=1"}},
	}
	for _, tc := range cases {
		got := Env("0.1.0", c, output.JSON, "/usr/bin/lfed", tc.yes, tc.insecure)
		want := append(slices.Clone(base), tc.want...)
		if !slices.Equal(got, want) {
			t.Errorf("Env(yes=%v, insecure=%v) = %v, want %v", tc.yes, tc.insecure, got, want)
		}
	}
}

func TestMetadataEnv(t *testing.T) {
	t.Setenv("PATH", "/bin")
	t.Setenv("GITHUB_TOKEN", "secret")
	t.Setenv("LFED_TOKEN", "secret")
	got := metadataEnv("0.1.0")
	if !slices.Contains(got, "PATH=/bin") || !slices.Contains(got, "LFED_CLI_VERSION=0.1.0") {
		t.Errorf("metadataEnv = %v, want PATH and LFED_CLI_VERSION", got)
	}
	for _, kv := range got {
		if strings.Contains(kv, "secret") {
			t.Errorf("metadataEnv leaks %q", kv)
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

	installHello(t, f)
	if _, err := Lookup("hello"); err != nil {
		t.Fatal(err)
	}
	entries, _ = os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "hello" {
		t.Errorf("plugin dir after reinstall = %v, want only hello", entries)
	}

	if err := Remove("hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := Lookup("hello"); err == nil {
		t.Error("plugin still present after Remove")
	}
}

func TestInstallKeepsOldPluginWhenReplaceFails(t *testing.T) {
	dir := setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	installHello(t, f)
	old, err := os.ReadFile(filepath.Join(dir, "hello", ManifestFile))
	if err != nil {
		t.Fatal(err)
	}

	renameDir = func(from, to string) error {
		if strings.Contains(filepath.Base(from), ".install-") {
			return errors.New("injected rename failure")
		}
		return os.Rename(from, to)
	}
	t.Cleanup(func() { renameDir = os.Rename })

	_, err = Install(context.Background(), f, "./testdata/helloplugin", Options{})
	if err == nil || !strings.Contains(err.Error(), "injected rename failure") {
		t.Fatalf("err = %v, want the injected failure", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "hello", ManifestFile))
	if err != nil || !bytes.Equal(got, old) {
		t.Fatalf("old install not restored: %v", err)
	}
	if _, err := Lookup("hello"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "hello" {
			t.Errorf("leftover %s in plugin dir", e.Name())
		}
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

	t.Setenv("LFED_YES", "stale")
	t.Setenv("LFED_INSECURE_STORAGE", "stale")

	t.Setenv("LFED_TOKEN", "")
	os.Unsetenv("LFED_TOKEN")
	f.Token = func(context.Context) (string, error) {
		t.Fatal("Exec resolved the token")
		return "", nil
	}

	f.Yes = true
	if err := Exec(context.Background(), f, p, []string{"greet", "world"}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	for _, want := range []string{
		"LFED_CLI_VERSION=0.1.0\n",
		"LFED_CONTEXT=staging\n",
		"LFED_API_BASE_URL=https://api.staging.ed.linuxfoundation.org\n",
		"LFED_OUTPUT=json\n",
		"LFED_REEXEC=/opt/lfed/bin/lfed\n",
		"LFED_YES=1\n",
		"LFED_INSECURE_STORAGE=\n",
		"args: greet world\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "LFED_TOKEN") {
		t.Errorf("output has LFED_TOKEN:\n%s", out.String())
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

	f.Version = "9.0.0"
	err = Run(context.Background(), f, "hello", nil)
	if ExitCode(err) != 2 {
		t.Errorf("Run with lfed CLI 9.0.0: err = %v, want exit 2", err)
	}
}

func TestExitCodeUsageError(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &cmdutil.UsageError{Err: errors.New("unknown flag")})
	if ExitCode(err) != 2 {
		t.Errorf("ExitCode = %d, want 2", ExitCode(err))
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

func TestCompleteGetsContractEnv(t *testing.T) {
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
	for _, want := range []string{"LFED_CONTEXT=staging\n", "LFED_YES=\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}
