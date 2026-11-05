package pluginmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"oras.land/oras-go/v2/registry"
)

// DefaultRegistry is the OCI repository prefix a bare plugin name resolves
// against. LFED_PLUGIN_REGISTRY overrides it.
const DefaultRegistry = "ghcr.io/lf-certification/lfed-cli-plugins"

// Source types stored in lfed-plugin.json.
const (
	SourcePath = "path"
	SourceOCI  = "oci"
)

// Source records where a plugin was installed from. `lfed plugin upgrade`
// re-resolves an OCI source; a path source can only be reinstalled.
type Source struct {
	Type string `json:"type"`
	// Path is the absolute path given to `lfed plugin install`. Type path only.
	Path string `json:"path,omitempty"`
	// Ref is the OCI repository without a tag, e.g.
	// ghcr.io/lf-certification/lfed-cli-plugins/cluster. Type oci only.
	Ref string `json:"ref,omitempty"`
	// Version is the OCI tag installed, or the tag requested before
	// resolution. Empty means "highest compatible" before resolution, and
	// a digest pin after it. Type oci only.
	Version string `json:"version,omitempty"`
	// Digest is the OCI index digest (sha256:...). Install records it for
	// every OCI install. Before resolution it holds a digest pin. Type oci
	// only.
	Digest string `json:"digest,omitempty"`
	// Pinned is true when the operator named a version or a digest.
	// `lfed plugin upgrade` skips pinned installs. Type oci only.
	Pinned bool `json:"pinned,omitempty"`
}

// digestPin reports whether s names an index digest and no version.
func (s Source) digestPin() bool {
	return s.Pinned && s.Version == "" && s.Digest != ""
}

// Name returns the plugin name an OCI ref implies: its last path element.
func (s Source) Name() string {
	return s.Ref[strings.LastIndex(s.Ref, "/")+1:]
}

// String renders the source for `lfed plugin list`.
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

// shortRe is the bare form: <name>, <name>@<version> or
// <name>@sha256:<64 hex>.
var shortRe = regexp.MustCompile(`^([a-z][a-z0-9-]*)(?:@(sha256:[0-9a-f]{64}|[^@/:]+))?$`)

// digestRe is a sha256 index digest.
var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ParseSource classifies an install argument. The rule follows Go's import
// path convention so a name never shadows a directory:
//
//	./x ../x /x ~/x          -> local path (file or Go main package directory)
//	name[@version|@sha256:…]  -> OCI artifact under the default registry
//	host/path[:tag|@sha256:…] -> OCI artifact at a full reference
//
// A relative directory without ./ is rejected. The error names the fix.
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
	// Docker's rule: the first segment is a registry host only when it has
	// a dot or a port, or is localhost. `services/delivery/cli` is a
	// relative directory typed without ./ and falls through to the error.
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

// ociSource builds an OCI source for ref. pin is empty, a version tag or a
// sha256 digest.
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
