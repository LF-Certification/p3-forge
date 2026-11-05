FROM registry.k8s.io/kubectl:v1.35.2 AS kubectl-source

FROM docker.io/alpine:3.22

# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/p3-forge"
LABEL org.opencontainers.image.title="p3-artifact-collector"
LABEL org.opencontainers.image.description="kubectl, aws-cli v2, jq and an ssh client for K8s Jobs that copy files from VMs to S3"

# aws-cli MUST be 2.15.0 or newer. EKS Pod Identity needs it:
# https://docs.aws.amazon.com/eks/latest/userguide/pod-id-minimum-sdk.html
# Alpine 3.22 ships 2.27.x. Check with `aws --version` after a base bump.
RUN apk add --no-cache \
        aws-cli \
        bash \
        coreutils \
        jq \
        openssh-client

COPY --from=kubectl-source /bin/kubectl /usr/local/bin/kubectl

ENTRYPOINT ["/bin/bash"]
