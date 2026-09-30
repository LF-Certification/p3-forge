#!/usr/bin/env bashp
set -ex

install -d /etc/rancher/k3s
cat >/etc/rancher/k3s/config.yaml <<'EOF'
write-kubeconfig-mode: "0644"
node-name: node
disable-network-policy: true
disable:
  - traefik
  - metrics-server
  - servicelb
EOF

p3forge::k3s

tekton_version="${TEKTON_VERSION:?must be set}"
tkn_version="${TKN_VERSION:?must be set}"

case "$(uname -m)" in
    x86_64)
        tkn_arch="x86_64"
        ;;
    aarch64 | arm64)
        tkn_arch="aarch64"
        ;;
    *)
        echo "Unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

tekton_manifest="$tmpdir/tekton-pipelines-v${tekton_version}.yaml"
curl -fsSL --retry 5 --retry-all-errors \
    "https://infra.tekton.dev/tekton-releases/pipeline/previous/v${tekton_version}/release.yaml" \
    --output "$tekton_manifest"
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
tkn_checksums="$tmpdir/checksums.txt"
curl -fsSL --retry 5 --retry-all-errors \
    "https://github.com/tektoncd/cli/releases/download/v${tkn_version}/checksums.txt" \
    --output "$tkn_checksums"
(
    cd "$tmpdir"
    sha256sum --check --ignore-missing "$(basename "$tkn_checksums")"
)
tar xzf "$tkn_archive" -C "$tmpdir" tkn
install -m 0755 "$tmpdir/tkn" /usr/local/bin/tkn
install -d /etc/bash_completion.d
tkn completion bash >/etc/bash_completion.d/tkn

tkn version

p3forge::wipe_machine_id
