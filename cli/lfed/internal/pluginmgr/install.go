package pluginmgr

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	sdkplugin "github.com/LF-Certification/p3-forge/cli/lfed/sdk/plugin"
)

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type Options struct {
	Reserved []string

	AllowUnsigned bool
}

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
	// #nosec G301
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Installed{}, fmt.Errorf("create plugin dir: %w", err)
	}

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
		src, err = fetchOCI(ctx, f, src, opts, bin)
	default:
		err = fmt.Errorf("unknown source type %q", src.Type)
	}
	if err != nil {
		return Installed{}, err
	}

	m, err := metadata(ctx, bin, f.Version)
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

		if !src.digestPin() && m.Version != src.Version {
			return Installed{}, fmt.Errorf("%s:%s reports version %q; the tag and the manifest disagree", src.Ref, src.Version, m.Version)
		}
	}

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
	// #nosec G306
	if err := os.WriteFile(filepath.Join(stage, ManifestFile), manifest.Bytes(), 0o644); err != nil {
		return Installed{}, fmt.Errorf("write manifest: %w", err)
	}

	final := filepath.Join(dir, m.Name)
	if err := replaceDir(stage, final); err != nil {
		return Installed{}, err
	}
	if src.Type == SourceOCI && opts.AllowUnsigned {
		fmt.Fprintf(f.IOStreams.ErrOut, "warning: %s installed without signature verification\n", src)
	}
	return Installed{Manifest: m, Source: src, Dir: final, Binary: filepath.Join(final, BinaryName(m.Name))}, nil
}

var renameDir = os.Rename

func replaceDir(stage, final string) error {
	prev := ""
	if _, err := os.Lstat(final); err == nil {
		prev = filepath.Join(filepath.Dir(final), ".prev-"+filepath.Base(final)+"-"+rand.Text())
		if err := renameDir(final, prev); err != nil {
			return fmt.Errorf("move existing plugin aside: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect existing plugin: %w", err)
	}
	if err := renameDir(stage, final); err != nil {
		if prev != "" {
			if rerr := renameDir(prev, final); rerr != nil {
				return fmt.Errorf("install plugin: %w; the previous install is in %s: %v", err, prev, rerr)
			}
		}
		return fmt.Errorf("install plugin: %w", err)
	}
	if prev != "" {

		_ = os.RemoveAll(prev)
	}
	return nil
}

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

func fetchOCI(ctx context.Context, f *cmdutil.Factory, src Source, opts Options, bin string) (Source, error) {
	running := f.Version
	repo, err := ociRepository(src)
	if err != nil {
		return src, err
	}
	res, err := resolveOCI(ctx, repo, src, running)
	if err != nil {
		return src, err
	}
	label := src.Name() + "@" + res.Version
	if src.digestPin() {
		label = src.Name() + "@" + src.Digest
	}
	if requires := res.Index.Annotations[AnnotationRequiresLfed]; requires != "" {
		if err := CheckLfedVersion(requires, running); err != nil {
			var incompatible *IncompatibleError
			if errors.As(err, &incompatible) {
				incompatible.Name = label
			}
			return src, err
		}
	}
	if name := res.Index.Annotations[AnnotationName]; name != "" && name != src.Name() {
		return src, fmt.Errorf("%s is published as plugin %q", repoRef(repo, res.Desc.Digest.String()), name)
	}
	if !src.digestPin() {
		src.Version = res.Version
	}
	src.Digest = res.Desc.Digest.String()
	ref := repoRef(repo, src.Digest)
	if !opts.AllowUnsigned {
		s, err := signerFor(src, res.Version)
		if err != nil {
			return src, err
		}
		if err := verifySignature(ctx, repo, ref, res.Desc, s); err != nil {
			return src, err
		}
	}
	return src, pullOCI(ctx, repo, res.Index, src.Name(), ref, bin)
}

func Remove(name string) error {
	if !nameRe.MatchString(name) {
		return &NotFoundError{Name: name}
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	target := filepath.Join(dir, name)

	if rel, err := filepath.Rel(dir, target); err != nil || rel != name {
		return &NotFoundError{Name: name}
	}
	if _, err := os.Lstat(target); errors.Is(err, fs.ErrNotExist) {
		return &NotFoundError{Name: name}
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove plugin %q: %w", name, err)
	}
	return nil
}

const metadataTimeout = 30 * time.Second

func metadata(ctx context.Context, bin, lfedVersion string) (sdkplugin.Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "__metadata")
	cmd.Env = metadataEnv(lfedVersion)
	cmd.WaitDelay = 5 * time.Second
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

func metadataEnv(lfedVersion string) []string {
	env := []string{"LFED_CLI_VERSION=" + lfedVersion}
	for _, k := range []string{"PATH", "HOME", "TMPDIR", "TEMP", "TMP", "SystemRoot"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// #nosec G302
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

func trailer(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return ": " + s
}
