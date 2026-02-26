#!/usr/bin/env bashp

# Software Installation
IFS=. read -ra versions <<<"$K8S_VERSION"
majorMinorVersion="${versions[0]}.${versions[1]}"
version="${majorMinorVersion}.${versions[2]:-0}"

export DEBIAN_FRONTEND=noninteractive
curl -fsSL "https://pkgs.k8s.io/core:/stable:/v${majorMinorVersion}/deb/Release.key" |
  gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${majorMinorVersion}/deb/ /" \
	> /etc/apt/sources.list.d/kubernetes.list
apt-get update
apt-get install -y containerd kubeadm="${version}-*" kubelet="${version}-*" kubectl="${version}-*"
apt-mark hold kubeadm kubelet kubectl

systemctl enable containerd
systemctl disable kubelet

# Configuration
k8s::configure_shell

mkdir -p /etc/modules-load.d
cat >/etc/modules-load.d/kubernetes.conf <<EOT
br_netfilter
nf_conntrack
EOT

cat >/etc/sysctl.d/kubernetes.conf <<EOT
net.bridge.bridge-nf-call-iptables = 1
net.ipv6.conf.all.forwarding = 1
net.ipv4.ip_forward = 1
net.netfilter.nf_conntrack_max = 131072
EOT

cat >/etc/crictl.yaml <<EOT
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
EOT

# Pre-caching
kubeadm config images pull
