package pluginmgr

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// OCI artifact contract. The release workflow in p3-platform writes these;
// the lfed CLI reads them. Both sides MUST change together.
const (
	// ArtifactType marks a per-platform plugin manifest.
	ArtifactType = "application/vnd.linuxfoundation.ed.cli.plugin.v1"
	// LayerMediaType is the single layer: a tar.gz holding lfed-<name>.
	LayerMediaType = "application/vnd.linuxfoundation.ed.cli.plugin.layer.v1.tar+gzip"
	// AnnotationName on the index is the plugin name.
	AnnotationName = "org.linuxfoundation.ed.cli.plugin.name"
	// AnnotationRequiresLfed on the index mirrors the manifest requires_lfed
	// range, so the lfed CLI can pick a compatible version before pulling.
	AnnotationRequiresLfed = "org.linuxfoundation.ed.cli.plugin.requires_lfed"
)

// Size limits for registry content. A registry that exceeds them is
// refused before the bytes are read or written.
const (
	// maxManifestBytes caps an index and a platform manifest.
	maxManifestBytes = 4 << 20
	// maxTarEntries caps the entries in the layer tar.
	maxTarEntries = 64
)

// maxLayerBytes caps the compressed layer and the expanded tar stream.
// Tests lower it.
var maxLayerBytes int64 = 512 << 20

// NoReleaseError reports an OCI repository with no version the running lfed
// accepts. The lfed CLI exits 2.
type NoReleaseError struct {
	Ref     string
	Running string
	// Newest is the highest version seen and its requires_lfed range.
	// Empty when the repository has no semver tags.
	Newest, Requires string
}

func (e *NoReleaseError) Error() string {
	if e.Newest == "" {
		return fmt.Sprintf("%s has no releases", e.Ref)
	}
	return fmt.Sprintf("no release of %s supports lfed %s; newest is %s (requires %s)", e.Ref, e.Running, e.Newest, e.Requires)
}

// ociRepository opens src.Ref with credentials resolved per section
// "Auth" of the distribution plan: Docker config, then GITHUB_TOKEN,
// GH_TOKEN and `gh auth token` for ghcr.io only.
//
// The HTTP client limits the dial, the TLS handshake and the wait for
// response headers. It sets no whole-request timeout, because a layer
// download can take long; the caller's ctx cancels it.
func ociRepository(src Source) (*remote.Repository, error) {
	repo, err := remote.NewRepository(src.Ref)
	if err != nil {
		return nil, fmt.Errorf("invalid OCI reference %q: %w", src.Ref, err)
	}
	repo.PlainHTTP = plainHTTP(repo.Reference.Registry)
	// Signature discovery reads referrers; see referrerBundles for the
	// descriptor cap that goes with this page cap.
	repo.ReferrerListMaxPages = maxReferrerPages
	repo.Client = &auth.Client{
		Client: &http.Client{Transport: retry.NewTransport(&http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ForceAttemptHTTP2:     true,
		})},
		Cache:      auth.NewCache(),
		Credential: credential,
	}
	return repo, nil
}

// plainHTTP is true for loopback registries, which tests run.
func plainHTTP(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

// resolved is the outcome of resolveOCI.
type resolved struct {
	// Version is the tag, or for a digest pin the index annotation
	// org.opencontainers.image.version (empty when the index has none).
	Version string
	// Desc is the index descriptor. Desc.Digest is what a signature covers.
	Desc  ocispec.Descriptor
	Index ocispec.Index
}

// resolveOCI picks the version to install. A pinned digest or version is
// used as is. Otherwise tags are read, sorted as semver descending, and
// the first index whose requires_lfed annotation accepts running wins.
func resolveOCI(ctx context.Context, repo *remote.Repository, src Source, running string) (resolved, error) {
	switch {
	case src.Digest != "" && src.Version == "":
		desc, idx, err := fetchIndex(ctx, repo, src.Digest)
		return resolved{Version: idx.Annotations[ocispec.AnnotationVersion], Desc: desc, Index: idx}, err
	case src.Version != "":
		desc, idx, err := fetchIndex(ctx, repo, src.Version)
		return resolved{Version: src.Version, Desc: desc, Index: idx}, err
	}
	var versions []*semver.Version
	// Only plain X.Y.Z tags are versions. The publish script also tags each
	// platform manifest as <version>_<os>_<arch>; pre-release or metadata
	// suffixes would be a publishing mistake. Both are ignored here.
	err := repo.Tags(ctx, "", func(tags []string) error {
		for _, t := range tags {
			if v, err := semver.StrictNewVersion(t); err == nil && v.Prerelease() == "" && v.Metadata() == "" {
				versions = append(versions, v)
			}
		}
		return nil
	})
	if err != nil {
		return resolved{}, ociError(src.Ref, err)
	}
	sort.Sort(sort.Reverse(semver.Collection(versions)))
	notFound := &NoReleaseError{Ref: src.Ref, Running: running}
	for _, v := range versions {
		tag := v.Original()
		desc, idx, err := fetchIndex(ctx, repo, tag)
		if err != nil {
			return resolved{}, err
		}
		requires := idx.Annotations[AnnotationRequiresLfed]
		if requires == "" || CheckLfedVersion(requires, running) == nil {
			return resolved{Version: tag, Desc: desc, Index: idx}, nil
		}
		if notFound.Newest == "" {
			notFound.Newest, notFound.Requires = tag, requires
		}
	}
	return resolved{}, notFound
}

// fetchIndex resolves reference (a tag or a digest) and parses the index
// it names.
func fetchIndex(ctx context.Context, repo *remote.Repository, reference string) (ocispec.Descriptor, ocispec.Index, error) {
	ref := repoRef(repo, reference)
	desc, data, err := fetchReference(ctx, repo, reference)
	if err != nil {
		return ocispec.Descriptor{}, ocispec.Index{}, ociError(ref, err)
	}
	if desc.MediaType != ocispec.MediaTypeImageIndex {
		return ocispec.Descriptor{}, ocispec.Index{}, fmt.Errorf("%s is %s; expected an OCI index", ref, desc.MediaType)
	}
	var idx ocispec.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return ocispec.Descriptor{}, ocispec.Index{}, fmt.Errorf("parse index %s: %w", ref, err)
	}
	return desc, idx, nil
}

// repoRef renders the repository of repo with reference: name:tag or
// name@digest.
func repoRef(repo *remote.Repository, reference string) string {
	sep := ":"
	if strings.Contains(reference, ":") {
		sep = "@"
	}
	return repo.Reference.Registry + "/" + repo.Reference.Repository + sep + reference
}

// fetchReference fetches the manifest ref names. A descriptor larger than
// maxManifestBytes is refused before the body is read.
func fetchReference(ctx context.Context, repo *remote.Repository, ref string) (ocispec.Descriptor, []byte, error) {
	desc, rc, err := repo.Manifests().FetchReference(ctx, ref)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	defer rc.Close()
	if desc.Size > maxManifestBytes {
		return ocispec.Descriptor{}, nil, fmt.Errorf("manifest is %d bytes; the limit is %d", desc.Size, maxManifestBytes)
	}
	data, err := content.ReadAll(rc, desc)
	return desc, data, err
}

// pullOCI writes the lfed-<name> binary for the running platform from idx
// into dst with mode 0755.
func pullOCI(ctx context.Context, repo *remote.Repository, idx ocispec.Index, name, ref, dst string) error {
	var platform *ocispec.Descriptor
	for i := range idx.Manifests {
		p := idx.Manifests[i].Platform
		if p != nil && p.OS == runtime.GOOS && p.Architecture == runtime.GOARCH {
			platform = &idx.Manifests[i]
			break
		}
	}
	if platform == nil {
		return fmt.Errorf("%s has no build for %s/%s", ref, runtime.GOOS, runtime.GOARCH)
	}
	if platform.Size > maxManifestBytes {
		return fmt.Errorf("%s: platform manifest is %d bytes; the limit is %d", ref, platform.Size, maxManifestBytes)
	}
	data, err := content.FetchAll(ctx, repo.Manifests(), *platform)
	if err != nil {
		return ociError(ref, err)
	}
	var m ocispec.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parse manifest %s: %w", ref, err)
	}
	if m.ArtifactType != ArtifactType {
		return fmt.Errorf("%s: platform manifest has artifactType %q; expected %q", ref, m.ArtifactType, ArtifactType)
	}
	var layer *ocispec.Descriptor
	for i := range m.Layers {
		if m.Layers[i].MediaType == LayerMediaType {
			layer = &m.Layers[i]
			break
		}
	}
	if layer == nil {
		return fmt.Errorf("%s has no %s layer", ref, LayerMediaType)
	}
	if layer.Size > maxLayerBytes {
		return fmt.Errorf("%s: layer is %d bytes; the limit is %d", ref, layer.Size, maxLayerBytes)
	}
	rc, err := repo.Blobs().Fetch(ctx, *layer)
	if err != nil {
		return ociError(ref, err)
	}
	defer rc.Close()
	vr := content.NewVerifyReader(rc, *layer)
	if err := extractBinary(vr, BinaryName(name), dst); err != nil {
		return fmt.Errorf("%s: %w", ref, err)
	}
	if err := vr.Verify(); err != nil {
		return fmt.Errorf("%s: layer %w", ref, err)
	}
	return nil
}

// extractBinary copies the regular file called want out of the tar.gz
// stream r into dst. The stream is drained so the caller can verify it.
//
// Only the first regular entry with base name want is written. Other
// entries, and every non-regular entry, are skipped. The function
// refuses an entry name that is absolute or holds a ".." element, more
// than maxTarEntries entries, and an expanded stream above maxLayerBytes.
func extractBinary(r io.Reader, want, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("layer is not gzip: %w", err)
	}
	defer gz.Close()
	// One byte above the limit shows an overflow.
	expanded := &io.LimitedReader{R: gz, N: maxLayerBytes + 1}
	overflow := func() error {
		if expanded.N <= 0 {
			return fmt.Errorf("layer expands to more than %d bytes", maxLayerBytes)
		}
		return nil
	}
	tr := tar.NewReader(expanded)
	found := false
	for entries := 1; ; entries++ {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if oerr := overflow(); oerr != nil {
				return oerr
			}
			return fmt.Errorf("read layer tar: %w", err)
		}
		if entries > maxTarEntries {
			return fmt.Errorf("layer holds more than %d entries", maxTarEntries)
		}
		if unsafeEntryName(hdr.Name) {
			return fmt.Errorf("layer entry %q is absolute or holds \"..\"", hdr.Name)
		}
		if found || hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != want {
			continue
		}
		// #nosec G302 -- the plugin binary must be executable.
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, cerr := io.CopyN(out, tr, maxLayerBytes+1)
		if errors.Is(cerr, io.EOF) {
			cerr = nil
		}
		if err := out.Close(); cerr == nil {
			cerr = err
		}
		if oerr := overflow(); oerr != nil {
			return oerr
		}
		if cerr != nil {
			return fmt.Errorf("write %s: %w", want, cerr)
		}
		found = true
	}
	if err := overflow(); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("layer holds no file named %s", want)
	}
	// Drain so digest verification sees the whole blob.
	_, err = io.Copy(io.Discard, r)
	return err
}

// unsafeEntryName reports a tar entry name that is absolute or holds a
// ".." element, with either path separator.
func unsafeEntryName(name string) bool {
	if path.IsAbs(name) || strings.HasPrefix(name, `\`) || (len(name) > 1 && name[1] == ':') {
		return true
	}
	return slices.Contains(strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }), "..")
}

// ociError turns registry failures into messages that name the next step.
func ociError(ref string, err error) error {
	var resp *errcode.ErrorResponse
	if errors.As(err, &resp) {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			host := resp.URL.Host
			hint := fmt.Sprintf(`run "docker login %s"`, host)
			if host == "ghcr.io" {
				hint = `run "gh auth refresh -s read:packages" or "docker login ghcr.io"`
			}
			return fmt.Errorf("%s denied access to %s; %s", host, ref, hint)
		case http.StatusNotFound:
			return fmt.Errorf("%s not found", ref)
		}
	}
	return fmt.Errorf("%s: %w", ref, err)
}

// defaultRegistryHost is the host part of DefaultRegistry.
var defaultRegistryHost = strings.SplitN(DefaultRegistry, "/", 2)[0]
