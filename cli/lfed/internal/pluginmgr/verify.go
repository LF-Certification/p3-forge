package pluginmgr

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
)

const (
	bundleArtifactType = "application/vnd.dev.sigstore.bundle.v0.3+json"

	bundleMediaTypePrefix = "application/vnd.dev.sigstore.bundle"

	maxBundleBytes = 1 << 20

	maxReferrerPages = 4

	maxReferrers = 16

	githubIssuer = "https://token.actions.githubusercontent.com"

	releaseWorkflow = "https://github.com/LF-Certification/p3-platform/.github/workflows/lfed-cli-plugins-release.yml"

	EnvSignerIdentity = "LFED_PLUGIN_SIGNER_IDENTITY"

	EnvSignerIssuer = "LFED_PLUGIN_SIGNER_ISSUER"
)

type signer struct {
	Issuer   string
	SANRegex string
}

func signerFor(src Source, version string) (signer, error) {
	if strings.HasPrefix(src.Ref, DefaultRegistry+"/") {
		if version == "" {
			return signer{}, fmt.Errorf("%s has no %s annotation; the signer identity needs the version", src, ocispec.AnnotationVersion)
		}
		san := releaseWorkflow + "@refs/tags/lfed-cli-" + src.Name() + "/v" + version
		return signer{Issuer: githubIssuer, SANRegex: "^" + regexp.QuoteMeta(san) + "$"}, nil
	}
	id, issuer := os.Getenv(EnvSignerIdentity), os.Getenv(EnvSignerIssuer)
	if id == "" || issuer == "" {
		return signer{}, fmt.Errorf("%s is not under %s; set %s (SAN regular expression) and %s (OIDC issuer) to verify its signature, or pass --allow-unsigned for a registry you control",
			src.Ref, DefaultRegistry, EnvSignerIdentity, EnvSignerIssuer)
	}
	if _, err := regexp.Compile(id); err != nil {
		return signer{}, fmt.Errorf("%s: %w", EnvSignerIdentity, err)
	}
	return signer{Issuer: issuer, SANRegex: id}, nil
}

func verifySignature(ctx context.Context, repo *remote.Repository, ref string, desc ocispec.Descriptor, s signer) error {
	if desc.Digest.Algorithm() != digest.SHA256 {
		return fmt.Errorf("%s: index digest %s is not sha256", ref, desc.Digest)
	}
	want, err := hex.DecodeString(desc.Digest.Encoded())
	if err != nil {
		return fmt.Errorf("%s: %w", ref, err)
	}
	bundles, err := referrerBundles(ctx, repo, desc)
	if err != nil {
		return fmt.Errorf("%s: read signatures: %w", ref, err)
	}
	if len(bundles) == 0 {
		return fmt.Errorf("%s has no Sigstore signature; pass --allow-unsigned only for a registry you control", ref)
	}
	trusted, err := trustedRoot(ctx)
	if err != nil {
		return fmt.Errorf("fetch Sigstore trusted root: %w", err)
	}
	v, err := verify.NewVerifier(trusted,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1))
	if err != nil {
		return err
	}
	id, err := verify.NewShortCertificateIdentity(s.Issuer, "", "", s.SANRegex)
	if err != nil {
		return err
	}
	policy := verify.NewPolicy(verify.WithArtifactDigest("sha256", want), verify.WithCertificateIdentity(id))
	var errs []error
	for _, b := range bundles {
		_, err := v.Verify(b, policy)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	return fmt.Errorf("%s: no signature from %s (issuer %s) is valid: %w", ref, s.SANRegex, s.Issuer, errors.Join(errs...))
}

func referrerBundles(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor) ([]*bundle.Bundle, error) {
	var referrers []ocispec.Descriptor
	err := repo.Referrers(ctx, desc, bundleArtifactType, func(page []ocispec.Descriptor) error {
		if len(referrers)+len(page) > maxReferrers {
			return fmt.Errorf("more than %d signature referrers", maxReferrers)
		}
		referrers = append(referrers, page...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	var out []*bundle.Bundle
	for _, r := range referrers {
		if r.Size > maxManifestBytes {
			return nil, fmt.Errorf("referrer %s is %d bytes; the limit is %d", r.Digest, r.Size, maxManifestBytes)
		}
		data, err := content.FetchAll(ctx, repo.Manifests(), r)
		if err != nil {
			return nil, fmt.Errorf("fetch referrer %s: %w", r.Digest, err)
		}
		layer, err := parseBundleReferrer(data)
		if err != nil {
			return nil, fmt.Errorf("referrer %s: %w", r.Digest, err)
		}
		raw, err := content.FetchAll(ctx, repo.Blobs(), layer)
		if err != nil {
			return nil, fmt.Errorf("fetch bundle %s: %w", layer.Digest, err)
		}
		var b bundle.Bundle
		if err := b.UnmarshalJSON(raw); err != nil {
			return nil, fmt.Errorf("parse bundle %s: %w", layer.Digest, err)
		}
		out = append(out, &b)
	}
	return out, nil
}

func parseBundleReferrer(data []byte) (ocispec.Descriptor, error) {
	var m ocispec.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("parse manifest: %w", err)
	}
	var layers []ocispec.Descriptor
	for _, l := range m.Layers {
		if strings.HasPrefix(l.MediaType, bundleMediaTypePrefix) {
			layers = append(layers, l)
		}
	}
	if len(layers) != 1 {
		return ocispec.Descriptor{}, fmt.Errorf("manifest has %d Sigstore bundle layers; expected 1", len(layers))
	}
	if layers[0].Size > maxBundleBytes {
		return ocispec.Descriptor{}, fmt.Errorf("bundle is %d bytes; the limit is %d", layers[0].Size, maxBundleBytes)
	}
	return layers[0], nil
}

func trustedRoot(ctx context.Context) (*root.TrustedRoot, error) {
	opts := tuf.DefaultOptions().WithContext(ctx)
	if dir, err := os.UserCacheDir(); err == nil {
		opts = opts.WithCachePath(filepath.Join(dir, "lfed", "sigstore"))
	}
	return root.FetchTrustedRootWithOptions(opts)
}
