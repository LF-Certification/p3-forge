#!/usr/bin/env bashp

kubeVersion="$K8S_VERSION" kubeadm::prepare

# Install nerdctl to allow for self-contained multi-node
NERDCTL_VERSION=$(curl -s https://api.github.com/repos/containerd/nerdctl/releases/latest | yq -r '.tag_name')
curl -sL "https://github.com/containerd/nerdctl/releases/download/${NERDCTL_VERSION}/nerdctl-${NERDCTL_VERSION#v}-linux-$(system::get_arch).tar.gz" | tar -xzC /usr/local/bin/ nerdctl
