package pluginmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"oras.land/oras-go/v2/registry"
)

const DefaultRegistry = "ghcr.io/lf-certification/lfed-cli-plugins"

const (
	SourcePath = "path"
	SourceOCI  = "oci"
)

type Source struct {
	Type string `json:"type"`

	Path string `json:"path,omitempty"`

	Ref string `json:"ref,omitempty"`

	Version string `json:"version,omitempty"`

	Digest string `json:"digest,omitempty"`

	Pinned bool `json:"pinned,omitempty"`
}

func (s Source) digestPin() bool {
	return s.Pinned && s.Version == "" && s.Digest != ""
}

func (s Source) Name() string {
	return s.Ref[strings.LastIndex(s.Ref, "/")+1:]
}

func (s Source) String() string {
	switch s.Type {
	case SourcePath:
		return "path:" + s.Path
	case SourceOCI:
		switch {
		case s.digestPin():
			return s.Ref + "@" + s.Digest
		case s.Version == "":
			return s.Ref
		}
		return s.Ref + "@" + s.Version
	}
	return "unknown"
}

var shortRe = regexp.MustCompile(`^([a-z][a-z0-9-]*)(?:@(sha256:[0-9a-f]{64}|[^@/:]+))?$`)

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func ParseSource(arg string) (Source, error) {
	switch {
	case arg == "." || arg == ".." || strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") ||
		strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "~/") || filepath.IsAbs(arg):
		p := arg
		if strings.HasPrefix(p, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return Source{}, err
			}
			p = filepath.Join(home, p[2:])
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return Source{}, err
		}
		return Source{Type: SourcePath, Path: abs}, nil
	}
	if m := shortRe.FindStringSubmatch(arg); m != nil {
		return ociSource(registryPrefix()+"/"+m[1], m[2]), nil
	}

	if host, _, ok := strings.Cut(arg, "/"); ok && (strings.ContainsAny(host, ".:") || host == "localhost") {
		ref, err := registry.ParseReference(arg)
		if err != nil {
			return Source{}, fmt.Errorf("invalid OCI reference %q: %w", arg, err)
		}
		if strings.Contains(ref.Reference, ":") && !digestRe.MatchString(ref.Reference) {
			return Source{}, fmt.Errorf("OCI reference %q pins a digest that is not sha256:<64 hex>", arg)
		}
		if !nameRe.MatchString(filepath.Base(ref.Repository)) {
			return Source{}, fmt.Errorf("OCI repository %q must end in a plugin name matching %s", ref.Repository, nameRe)
		}
		return ociSource(ref.Registry+"/"+ref.Repository, ref.Reference), nil
	}
	return Source{}, fmt.Errorf("unknown plugin source %q: use ./<dir> for a path, <name> for a plugin, or <registry>/<repository> for an OCI reference", arg)
}

func ociSource(ref, pin string) Source {
	s := Source{Type: SourceOCI, Ref: ref, Pinned: pin != ""}
	if digestRe.MatchString(pin) {
		s.Digest = pin
	} else {
		s.Version = pin
	}
	return s
}

func registryPrefix() string {
	if r := os.Getenv("LFED_PLUGIN_REGISTRY"); r != "" {
		return strings.TrimSuffix(r, "/")
	}
	return DefaultRegistry
}
