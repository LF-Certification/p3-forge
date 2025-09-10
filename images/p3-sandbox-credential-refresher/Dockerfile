FROM public.ecr.aws/aws-cli/aws-cli:2.17.47

# Install kubectl with multi-arch support
RUN yum install -y curl && \
    ARCH=$(uname -m | sed 's/x86_64/amd64/g' | sed 's/aarch64/arm64/g') && \
    curl -LO "https://dl.k8s.io/release/v1.31.0/bin/linux/${ARCH}/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/ && \
    kubectl version --client && \
    yum clean all && \
    rm -rf /var/cache/yum

# Verify installations
RUN aws --version && \
    kubectl version --client

# Set entrypoint to shell for script execution
ENTRYPOINT ["/bin/sh"]
