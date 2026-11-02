package pluginmgr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	sdkplugin "github.com/LF-Certification/p3-forge/cli/lfed/sdk/plugin"
)

// nameRe is the accepted plugin name shape. The name is a path element and
// a command word, so it is restricted to lower-case ASCII.
var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Options tune Install.
type Options struct {
	// Reserved are core command words. A plugin with one of these names
	// could never be dispatched, so Install refuses it.
	Reserved []string
}

// Install installs the plugin named by arg (see ParseSource) and returns
// it. The binary is staged in the plugin directory, asked for __metadata,
// validated against f.Version, then moved into place together with
// lfed-plugin.json. An existing install of the same name is replaced.
func Install(ctx context.Context, f *cmdutil.Factory, arg string, opts Options) (Installed, error) {
	src, err := ParseSource(arg)
	if err != nil {
		return Installed{}, err
	}
	if src.Type == SourceOCI && slices.Contains(opts.Reserved, src.Name()) {
		return Installed{}, fmt.Errorf("plugin name %q is a core lfed command", src.Name())
	}
	return install(ctx, f, src, opts)
}

func install(ctx context.Context, f *cmdutil.Factory, src Source, opts Options) (Installed, error) {
	dir, err := Dir()
	if err != nil {
		return Installed{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Installed{}, fmt.Errorf("create plugin dir: %w", err)
	}
	// Staging inside dir keeps the final os.Rename on one filesystem.
	stage, err := os.MkdirTemp(dir, ".install-")
	if err != nil {
		return Installed{}, fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stage)

	bin := filepath.Join(stage, "plugin")
	switch src.Type {
	case SourcePath:
		err = fetchPath(ctx, src.Path, bin)
	case SourceOCI:
		src.Version, err = fetchOCI(ctx, src, f.Version, bin)
	default:
		err = fmt.Errorf("unknown source type %q", src.Type)
	}
	if err != nil {
		return Installed{}, err
	}

	m, err := metadata(ctx, bin)
	if err != nil {
		return Installed{}, err
	}
	if err := validate(m, f.Version, opts.Reserved); err != nil {
		return Installed{}, fmt.Errorf("plugin manifest invalid: %w", err)
	}
	if src.Type == SourceOCI {
		if m.Name != src.Name() {
			return Installed{}, fmt.Errorf("%s publishes plugin %q, not %q", src.Ref, m.Name, src.Name())
		}
		if m.Version != src.Version {
			return Installed{}, fmt.Errorf("%s:%s reports version %q; the tag and the manifest disagree", src.Ref, src.Version, m.Version)
		}
	}

	// Encoder, not MarshalIndent: requires_lfed holds `<` and `>`, which
	// Marshal would write as \u003c and \u003e.
	var manifest bytes.Buffer
	enc := json.NewEncoder(&manifest)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(record{Manifest: m, Source: src}); err != nil {
		return Installed{}, err
	}
	if err := os.Rename(bin, filepath.Join(stage, BinaryName(m.Name))); err != nil {
		return Installed{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, ManifestFile), manifest.Bytes(), 0o644); err != nil {
		return Installed{}, fmt.Errorf("write manifest: %w", err)
	}

	final := filepath.Join(dir, m.Name)
	if err := os.RemoveAll(final); err != nil {
		return Installed{}, fmt.Errorf("replace existing plugin: %w", err)
	}
	if err := os.Rename(stage, final); err != nil {
		return Installed{}, fmt.Errorf("install plugin: %w", err)
	}
	return Installed{Manifest: m, Source: src, Dir: final, Binary: filepath.Join(final, BinaryName(m.Name))}, nil
}

// fetchPath stages a local source: a directory is built with go build, a
// file is copied.
func fetchPath(ctx context.Context, path, bin string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return build(ctx, path, bin)
	}
	return copyFile(path, bin)
}

// fetchOCI stages the binary from an OCI artifact and returns the version
// it resolved to.
func fetchOCI(ctx context.Context, src Source, running, bin string) (string, error) {
	repo, err := ociRepository(src)
	if err != nil {
		return "", err
	}
	version, idx, err := resolveOCI(ctx, repo, src, running)
	if err != nil {
		return "", err
	}
	if requires := idx.Annotations[AnnotationRequiresLfed]; requires != "" {
		if err := CheckLfedVersion(requires, running); err != nil {
			var incompatible *IncompatibleError
			if errors.As(err, &incompatible) {
				incompatible.Name = src.Name() + "@" + version
			}
			return "", err
		}
	}
	if name := idx.Annotations[AnnotationName]; name != "" && name != src.Name() {
		return "", fmt.Errorf("%s:%s is published as plugin %q", src.Ref, version, name)
	}
	return version, pullOCI(ctx, repo, idx, src.Name(), version, bin)
}

// Remove deletes the plugin directory. A missing plugin yields
// *NotFoundError.
func Remove(name string) error {
	p, err := Lookup(name)
	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		return err
	}
	dir, derr := Dir()
	if derr != nil {
		return derr
	}
	target := filepath.Join(dir, name)
	if err == nil {
		target = p.Dir
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove plugin %q: %w", name, err)
	}
	return nil
}

// metadata runs `<bin> __metadata` and parses the manifest.
func metadata(ctx context.Context, bin string) (sdkplugin.Manifest, error) {
	cmd := exec.CommandContext(ctx, bin, "__metadata")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return sdkplugin.Manifest{}, fmt.Errorf("run __metadata: %w%s", err, trailer(stderr.String()))
	}
	var m sdkplugin.Manifest
	if err := json.Unmarshal(out, &m); err != nil {
		return sdkplugin.Manifest{}, fmt.Errorf("parse __metadata output: %w", err)
	}
	return m, nil
}

// validate checks the manifest fields the lfed CLI depends on.
func validate(m sdkplugin.Manifest, running string, reserved []string) error {
	if m.SchemaVersion != sdkplugin.ManifestSchemaVersion {
		return fmt.Errorf("schema_version %q; expected %q", m.SchemaVersion, sdkplugin.ManifestSchemaVersion)
	}
	if !nameRe.MatchString(m.Name) {
		return fmt.Errorf("name %q must match %s", m.Name, nameRe)
	}
	if slices.Contains(reserved, m.Name) {
		return fmt.Errorf("name %q is a core lfed command", m.Name)
	}
	if m.Version == "" {
		return errors.New("version is empty")
	}
	if m.Summary == "" {
		return errors.New("summary is empty")
	}
	if _, err := semver.NewConstraint(m.RequiresLfed); err != nil {
		return fmt.Errorf("requires_lfed %q: %w", m.RequiresLfed, err)
	}
	err := CheckLfedVersion(m.RequiresLfed, running)
	var incompatible *IncompatibleError
	if errors.As(err, &incompatible) {
		incompatible.Name = m.Name
	}
	return err
}

// build compiles the main package under dir into out. dir itself is used
// when it holds package main; otherwise exactly one cmd/lfed-* directory
// must hold it.
func build(ctx context.Context, dir, out string) error {
	pkg, err := mainPackage(dir)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", abs, ".")
	cmd.Dir = pkg
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build %s: %w%s", pkg, err, trailer(stderr.String()))
	}
	return nil
}

// mainPackage finds the directory holding package main for dir.
func mainPackage(dir string) (string, error) {
	ok, err := isMain(dir)
	if err != nil {
		return "", err
	}
	if ok {
		return dir, nil
	}
	candidates, err := filepath.Glob(filepath.Join(dir, "cmd", "lfed-*"))
	if err != nil {
		return "", err
	}
	var mains []string
	for _, c := range candidates {
		ok, err := isMain(c)
		if err != nil {
			return "", err
		}
		if ok {
			mains = append(mains, c)
		}
	}
	switch len(mains) {
	case 1:
		return mains[0], nil
	case 0:
		return "", fmt.Errorf("%s holds no Go main package, directly or under cmd/lfed-*", dir)
	}
	return "", fmt.Errorf("%s holds several main packages: %s", dir, strings.Join(mains, ", "))
}

// isMain reports whether dir holds non-test Go files declaring package main.
func isMain(dir string) (bool, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return false, err
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, parser.PackageClauseOnly)
		if err != nil {
			return false, fmt.Errorf("parse %s: %w", file, err)
		}
		return f.Name.Name == "main", nil
	}
	return false, nil
}

// copyFile copies src to dst with mode 0755.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return out.Close()
}

// trailer formats captured stderr for appending to an error message.
func trailer(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return ": " + s
}
