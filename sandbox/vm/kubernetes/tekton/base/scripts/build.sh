#!/usr/bin/env bashp
set -ex

p3forge::k3s

tekton_version="${TEKTON_VERSION:?must be set}"
tkn_version="${TKN_VERSION:?must be set}"

case "$tekton_version" in
    1.9.0)
        tekton_manifest_sha256="03686500346700250295fda68a77ed882f4ad8fa3b5b172f5f44c405ed9156eb"
        ;;
    *)
        echo "Unsupported Tekton Pipelines version: $tekton_version" >&2
        exit 1
        ;;
esac

case "$(uname -m)" in
    x86_64)
        tkn_arch="x86_64"
        tkn_sha256="ea282eeef10a886117c54ecd8f9c1cde516a4fe8c7ba05011d33515ced164ff2"
        ;;
    aarch64 | arm64)
        tkn_arch="aarch64"
        tkn_sha256="5c03526a45aad8218857b8e55dd7e1851f13927a347641c41e90df7efdfc7b47"
        ;;
    *)
        echo "Unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

case "$tkn_version" in
    0.44.0) ;;
    *)
        echo "Unsupported tkn version: $tkn_version" >&2
        exit 1
        ;;
esac

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

tekton_manifest="$tmpdir/tekton-pipelines-v${tekton_version}.yaml"
curl -fsSL --retry 5 --retry-all-errors \
    "https://infra.tekton.dev/tekton-releases/pipeline/previous/v${tekton_version}/release.yaml" \
    --output "$tekton_manifest"
printf '%s  %s\n' "$tekton_manifest_sha256" "$tekton_manifest" | sha256sum --check -
kubectl apply --filename "$tekton_manifest"

kubectl wait --for=condition=ready pod \
    --selector=app.kubernetes.io/part-of=tekton-pipelines \
    --namespace=tekton-pipelines \
    --timeout=180s
kubectl wait --for=condition=ready pod \
    --selector=app.kubernetes.io/component=resolvers \
    --namespace=tekton-pipelines-resolvers \
    --timeout=180s

tkn_archive="$tmpdir/tkn_${tkn_version}_Linux_${tkn_arch}.tar.gz"
curl -fsSL --retry 5 --retry-all-errors \
    "https://github.com/tektoncd/cli/releases/download/v${tkn_version}/$(basename "$tkn_archive")" \
    --output "$tkn_archive"
printf '%s  %s\n' "$tkn_sha256" "$tkn_archive" | sha256sum --check -
tar xzf "$tkn_archive" -C "$tmpdir" tkn
install -m 0755 "$tmpdir/tkn" /usr/local/bin/tkn
install -d /etc/bash_completion.d
tkn completion bash >/etc/bash_completion.d/tkn

tkn version
