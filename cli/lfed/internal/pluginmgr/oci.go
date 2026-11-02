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
	"sort"
	"strings"

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
// "Auth" of the distribution plan: Docker config, GITHUB_TOKEN, GH_TOKEN,
// then `gh auth token` for ghcr.io.
func ociRepository(src Source) (*remote.Repository, error) {
	repo, err := remote.NewRepository(src.Ref)
	if err != nil {
		return nil, fmt.Errorf("invalid OCI reference %q: %w", src.Ref, err)
	}
	repo.PlainHTTP = plainHTTP(repo.Reference.Registry)
	repo.Client = &auth.Client{
		Client:     retry.DefaultClient,
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

// resolveOCI picks the version to install. A pinned version is used as is.
// Otherwise tags are read, sorted as semver descending, and the first index
// whose requires_lfed annotation accepts running wins. It returns the
// version and the index.
func resolveOCI(ctx context.Context, repo *remote.Repository, src Source, running string) (string, ocispec.Index, error) {
	if src.Version != "" {
		idx, err := fetchIndex(ctx, repo, src.Version)
		return src.Version, idx, err
	}
	var versions []*semver.Version
	err := repo.Tags(ctx, "", func(tags []string) error {
		for _, t := range tags {
			if v, err := semver.StrictNewVersion(t); err == nil {
				versions = append(versions, v)
			}
		}
		return nil
	})
	if err != nil {
		return "", ocispec.Index{}, ociError(src.Ref, err)
	}
	sort.Sort(sort.Reverse(semver.Collection(versions)))
	notFound := &NoReleaseError{Ref: src.Ref, Running: running}
	for _, v := range versions {
		tag := v.Original()
		idx, err := fetchIndex(ctx, repo, tag)
		if err != nil {
			return "", ocispec.Index{}, err
		}
		requires := idx.Annotations[AnnotationRequiresLfed]
		if requires == "" || CheckLfedVersion(requires, running) == nil {
			return tag, idx, nil
		}
		if notFound.Newest == "" {
			notFound.Newest, notFound.Requires = tag, requires
		}
	}
	return "", ocispec.Index{}, notFound
}

// fetchIndex resolves tag and parses the index it names.
func fetchIndex(ctx context.Context, repo *remote.Repository, tag string) (ocispec.Index, error) {
	ref := repo.Reference.Registry + "/" + repo.Reference.Repository
	desc, data, err := fetchReference(ctx, repo, tag)
	if err != nil {
		return ocispec.Index{}, ociError(ref+":"+tag, err)
	}
	if desc.MediaType != ocispec.MediaTypeImageIndex {
		return ocispec.Index{}, fmt.Errorf("%s:%s is %s; expected an OCI index", ref, tag, desc.MediaType)
	}
	var idx ocispec.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return ocispec.Index{}, fmt.Errorf("parse index %s:%s: %w", ref, tag, err)
	}
	return idx, nil
}

func fetchReference(ctx context.Context, repo *remote.Repository, ref string) (ocispec.Descriptor, []byte, error) {
	desc, rc, err := repo.Manifests().FetchReference(ctx, ref)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	defer rc.Close()
	data, err := content.ReadAll(rc, desc)
	return desc, data, err
}

// pullOCI writes the lfed-<name> binary for the running platform from idx
// into dst with mode 0755.
func pullOCI(ctx context.Context, repo *remote.Repository, idx ocispec.Index, name, version, dst string) error {
	ref := repo.Reference.Registry + "/" + repo.Reference.Repository + ":" + version
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
	data, err := content.FetchAll(ctx, repo.Manifests(), *platform)
	if err != nil {
		return ociError(ref, err)
	}
	var m ocispec.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parse manifest %s: %w", ref, err)
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
func extractBinary(r io.Reader, want, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("layer is not gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	found := false
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read layer tar: %w", err)
		}
		if found || hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != want {
			continue
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, cerr := io.Copy(out, tr)
		if err := out.Close(); cerr == nil {
			cerr = err
		}
		if cerr != nil {
			return fmt.Errorf("write %s: %w", want, cerr)
		}
		found = true
	}
	if !found {
		return fmt.Errorf("layer holds no file named %s", want)
	}
	// Drain so digest verification sees the whole blob.
	_, err = io.Copy(io.Discard, r)
	return err
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
