package pluginmgr

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

// testRegistry serves an in-memory OCI registry and points
// LFED_PLUGIN_REGISTRY at <host>/lfed-cli-plugins.
func testRegistry(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	t.Setenv("LFED_PLUGIN_REGISTRY", host+"/lfed-cli-plugins")
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	return host
}

// helloBinary builds testdata/helloplugin once per test.
func helloBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "lfed-hello")
	if err := build(context.Background(), "testdata/helloplugin", bin); err != nil {
		t.Fatal(err)
	}
	return bin
}

// tarGz packs bin as name, plus a LICENSE file, like a GoReleaser archive.
func tarGz(t *testing.T, bin, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		body []byte
	}{{"LICENSE", []byte("MIT")}, {name, data}} {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type platform struct{ os, arch string }

var hostPlatform = platform{runtime.GOOS, runtime.GOARCH}

// publish pushes one plugin version the way the release workflow does:
// one artifact manifest per platform, then an index tagged with version.
func publish(t *testing.T, host, name, version, requires string, platforms []platform, archive []byte) {
	t.Helper()
	ctx := context.Background()
	repo, err := remote.NewRepository(host + "/lfed-cli-plugins/" + name)
	if err != nil {
		t.Fatal(err)
	}
	repo.PlainHTTP = true
	layer, err := oras.PushBytes(ctx, repo, LayerMediaType, archive)
	if err != nil {
		t.Fatal(err)
	}
	idx := ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Annotations: map[string]string{
			ocispec.AnnotationVersion: version,
			AnnotationName:            name,
		},
	}
	if requires != "" {
		idx.Annotations[AnnotationRequiresLfed] = requires
	}
	for _, p := range platforms {
		desc, err := oras.PackManifest(ctx, repo, oras.PackManifestVersion1_1, ArtifactType, oras.PackManifestOptions{
			Layers:              []ocispec.Descriptor{layer},
			ManifestAnnotations: map[string]string{ocispec.AnnotationCreated: "2026-01-01T00:00:00Z"},
		})
		if err != nil {
			t.Fatal(err)
		}
		desc.Platform = &ocispec.Platform{OS: p.os, Architecture: p.arch}
		idx.Manifests = append(idx.Manifests, desc)
	}
	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	desc := ocispec.Descriptor{MediaType: ocispec.MediaTypeImageIndex, Digest: digest.FromBytes(data), Size: int64(len(data))}
	if err := repo.Manifests().PushReference(ctx, desc, bytes.NewReader(data), version); err != nil {
		t.Fatal(err)
	}
}

func TestParseSource(t *testing.T) {
	t.Setenv("LFED_PLUGIN_REGISTRY", "")
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cases := []struct {
		arg  string
		want Source
		err  string
	}{
		{arg: "./cli", want: Source{Type: SourcePath, Path: filepath.Join(cwd, "cli")}},
		{arg: "../x", want: Source{Type: SourcePath, Path: filepath.Join(filepath.Dir(cwd), "x")}},
		{arg: "/opt/p", want: Source{Type: SourcePath, Path: "/opt/p"}},
		{arg: "~/p", want: Source{Type: SourcePath, Path: filepath.Join(home, "p")}},
		{arg: ".", want: Source{Type: SourcePath, Path: cwd}},
		{arg: "cluster", want: Source{Type: SourceOCI, Ref: DefaultRegistry + "/cluster"}},
		{arg: "cluster@1.2.0", want: Source{Type: SourceOCI, Ref: DefaultRegistry + "/cluster", Version: "1.2.0"}},
		{arg: "ghcr.io/acme/tools/foo", want: Source{Type: SourceOCI, Ref: "ghcr.io/acme/tools/foo"}},
		{arg: "ghcr.io/acme/tools/foo:2.0.0", want: Source{Type: SourceOCI, Ref: "ghcr.io/acme/tools/foo", Version: "2.0.0"}},
		{arg: "localhost:5000/p/foo:1.0.0", want: Source{Type: SourceOCI, Ref: "localhost:5000/p/foo", Version: "1.0.0"}},
		{arg: "services/delivery/cli", err: "unknown plugin source"},
		{arg: "Cluster", err: "unknown plugin source"},
		{arg: "cluster@1@2", err: "unknown plugin source"},
		{arg: "ghcr.io/acme/foo@sha256:0000000000000000000000000000000000000000000000000000000000000000", err: "pins a digest"},
		{arg: "ghcr.io/acme/Foo", err: "invalid OCI reference"},
	}
	for _, c := range cases {
		got, err := ParseSource(c.arg)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%q: err = %v, want containing %q", c.arg, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.arg, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q = %+v, want %+v", c.arg, got, c.want)
		}
	}
	if got, _ := ParseSource("services/delivery/cli"); got.Type != "" {
		t.Error("relative dir without ./ accepted")
	}
}

func TestParseSourceRegistryOverride(t *testing.T) {
	t.Setenv("LFED_PLUGIN_REGISTRY", "localhost:5000/mine/")
	got, err := ParseSource("foo@0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ref != "localhost:5000/mine/foo" {
		t.Errorf("Ref = %q", got.Ref)
	}
}

func TestInstallFromOCIPicksHighestCompatible(t *testing.T) {
	dir := setPluginDir(t)
	host := testRegistry(t)
	bin := helloBinary(t)
	archive := tarGz(t, bin, "lfed-hello")
	// 0.0.1 installs on lfed 0.x; 0.0.2 needs lfed 9; 0.0.10 has no build
	// for this platform but is skipped before that matters. A non-semver
	// tag is ignored.
	publish(t, host, "hello", "0.0.1", ">=0.1.0 <1", []platform{hostPlatform, {"plan9", "mips"}}, archive)
	publish(t, host, "hello", "0.0.2", ">=9", []platform{hostPlatform}, archive)
	publish(t, host, "hello", "latest", "", []platform{hostPlatform}, archive)

	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	p, err := Install(context.Background(), f, "hello", Options{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if p.Manifest.Version != "0.0.1" || p.Source.Version != "0.0.1" || p.Source.Type != SourceOCI {
		t.Errorf("installed %+v", p)
	}
	if p.Source.Ref != host+"/lfed-cli-plugins/hello" {
		t.Errorf("Ref = %q", p.Source.Ref)
	}
	info, err := os.Stat(filepath.Join(dir, "hello", "lfed-hello"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("binary mode %v is not executable", info.Mode())
	}
	// The record round-trips through List.
	got, err := Lookup("hello")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != p.Source {
		t.Errorf("Lookup source = %+v, want %+v", got.Source, p.Source)
	}
	// The installed binary runs.
	if err := Run(context.Background(), f, "hello", []string{"greet"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "args: greet") {
		t.Errorf("output = %q", out.String())
	}
}

func TestInstallFromOCIPinned(t *testing.T) {
	setPluginDir(t)
	host := testRegistry(t)
	archive := tarGz(t, helloBinary(t), "lfed-hello")
	publish(t, host, "hello", "0.0.1", ">=0.1.0 <1", []platform{hostPlatform}, archive)
	publish(t, host, "hello", "0.0.2", ">=9", []platform{hostPlatform}, archive)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)

	p, err := Install(context.Background(), f, "hello@0.0.1", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Source.Version != "0.0.1" {
		t.Errorf("Version = %q", p.Source.Version)
	}

	_, err = Install(context.Background(), f, "hello@0.0.2", Options{})
	var incompatible *IncompatibleError
	if !errors.As(err, &incompatible) || ExitCode(err) != 2 {
		t.Fatalf("pinned incompatible: err = %v, want *IncompatibleError", err)
	}

	_, err = Install(context.Background(), f, "hello@0.0.9", Options{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing tag: err = %v", err)
	}
}

func TestInstallFromOCIErrors(t *testing.T) {
	setPluginDir(t)
	host := testRegistry(t)
	bin := helloBinary(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	ctx := context.Background()

	// Nothing compatible.
	publish(t, host, "hello", "0.0.2", ">=9", []platform{hostPlatform}, tarGz(t, bin, "lfed-hello"))
	_, err := Install(ctx, f, "hello", Options{})
	var noRelease *NoReleaseError
	if !errors.As(err, &noRelease) || ExitCode(err) != 2 {
		t.Fatalf("err = %v, want *NoReleaseError", err)
	}
	if noRelease.Newest != "0.0.2" || noRelease.Requires != ">=9" {
		t.Errorf("NoReleaseError = %+v", noRelease)
	}

	// Unknown repository.
	_, err = Install(ctx, f, "nothing", Options{})
	if err == nil || !strings.Contains(err.Error(), "lfed-cli-plugins/nothing not found") {
		t.Fatalf("unknown repo: err = %v", err)
	}

	// No build for this platform.
	publish(t, host, "other", "0.0.1", "", []platform{{"plan9", "mips"}}, tarGz(t, bin, "lfed-other"))
	_, err = Install(ctx, f, "other", Options{})
	if err == nil || !strings.Contains(err.Error(), "no build for "+runtime.GOOS) {
		t.Fatalf("platform: err = %v", err)
	}

	// Layer without the binary.
	publish(t, host, "empty", "0.0.1", "", []platform{hostPlatform}, tarGz(t, bin, "README"))
	_, err = Install(ctx, f, "empty", Options{})
	if err == nil || !strings.Contains(err.Error(), "no file named lfed-empty") {
		t.Fatalf("missing binary: err = %v", err)
	}

	// Repository name and plugin name disagree.
	publish(t, host, "alias", "0.0.1", "", []platform{hostPlatform}, tarGz(t, bin, "lfed-alias"))
	_, err = Install(ctx, f, "alias", Options{})
	if err == nil || !strings.Contains(err.Error(), `plugin "hello", not "alias"`) {
		t.Fatalf("name mismatch: err = %v", err)
	}

	// Tag and manifest version disagree.
	publish(t, host, "hello", "0.0.3", "", []platform{hostPlatform}, tarGz(t, bin, "lfed-hello"))
	_, err = Install(ctx, f, "hello@0.0.3", Options{})
	if err == nil || !strings.Contains(err.Error(), "disagree") {
		t.Fatalf("version mismatch: err = %v", err)
	}
}

func TestInstallRejectsReservedName(t *testing.T) {
	setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	_, err := Install(context.Background(), f, "./testdata/helloplugin", Options{Reserved: []string{"auth", "hello"}})
	if err == nil || !strings.Contains(err.Error(), `"hello" is a core lfed command`) {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallFromPathRecordsSource(t *testing.T) {
	setPluginDir(t)
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	p := installHello(t, f)
	abs, _ := filepath.Abs("testdata/helloplugin")
	if p.Source.Type != SourcePath || p.Source.Path != abs {
		t.Errorf("Source = %+v", p.Source)
	}
}

func TestUpgrade(t *testing.T) {
	dir := setPluginDir(t)
	host := testRegistry(t)
	archive := tarGz(t, helloBinary(t), "lfed-hello")
	var out, errOut bytes.Buffer
	f := testFactory(t, &out, &errOut)
	ctx := context.Background()

	// stale: installed 0.0.1 from OCI; 0.0.2 is incompatible, so it stays
	// current until a compatible newer version appears.
	publish(t, host, "hello", "0.0.1", ">=0.1.0 <1", []platform{hostPlatform}, archive)
	publish(t, host, "hello", "0.0.2", ">=9", []platform{hostPlatform}, archive)
	if _, err := Install(ctx, f, "hello@0.0.1", Options{}); err != nil {
		t.Fatal(err)
	}
	// path: installed from a directory.
	writeFake(t, dir, "local", "Local")
	rewriteSource(t, dir, "local", Source{Type: SourcePath, Path: "/src/local"})
	// legacy: manifest without a source.
	writeFake(t, dir, "legacy", "Legacy")

	results, err := Upgrade(ctx, f, nil, false, Options{})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]UpgradeResult{}
	for _, r := range results {
		byName[r.Name] = r
	}
	if r := byName["hello"]; r.Upgraded || r.To != "0.0.1" || r.Note != "current" {
		t.Errorf("hello = %+v", r)
	}
	if r := byName["local"]; r.Upgraded || !strings.Contains(r.Note, `lfed plugin install /src/local`) {
		t.Errorf("local = %+v", r)
	}
	if r := byName["legacy"]; r.Upgraded || !strings.Contains(r.Note, "source unknown") {
		t.Errorf("legacy = %+v", r)
	}

	// Simulate an older install: the binary stays 0.0.1 (the only version
	// the test plugin reports), the recorded source drops to 0.0.0.
	rewriteSource(t, dir, "hello", Source{Type: SourceOCI, Ref: host + "/lfed-cli-plugins/hello", Version: "0.0.0"})
	results, err = Upgrade(ctx, f, []string{"hello"}, true, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r := results[0]; r.Upgraded || r.To != "0.0.1" || r.Note != "dry run" {
		t.Errorf("dry run = %+v", r)
	}
	if p, _ := Lookup("hello"); p.Source.Version != "0.0.0" {
		t.Errorf("dry run changed the install: %+v", p.Source)
	}
	results, err = Upgrade(ctx, f, []string{"hello"}, false, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r := results[0]; !r.Upgraded || r.To != "0.0.1" {
		t.Errorf("upgrade = %+v", r)
	}
	if p, _ := Lookup("hello"); p.Source.Version != "0.0.1" {
		t.Errorf("after upgrade: %+v", p.Source)
	}

	_, err = Upgrade(ctx, f, []string{"nosuch"}, false, Options{})
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("unknown name: err = %v", err)
	}
}

// rewriteSource replaces the source in an installed manifest.
func rewriteSource(t *testing.T, dir, name string, src Source) {
	t.Helper()
	p, err := load(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record{Manifest: p.Manifest, Source: src})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name, ManifestFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
