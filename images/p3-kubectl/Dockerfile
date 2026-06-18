FROM registry.k8s.io/kubectl:v1.35.2 AS kubectl-source

FROM docker.io/alpine:3.21

# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="p3-kubectl"
LABEL org.opencontainers.image.description="Minimal shell-capable kubectl image for use in K8s Jobs and hooks"

COPY --from=kubectl-source /bin/kubectl /usr/local/bin/kubectl

ENTRYPOINT ["/bin/sh"]
