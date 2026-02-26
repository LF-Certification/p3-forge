#!/usr/bin/env bashp
set -ex

curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL="v${K8S_VERSION:?must be set}" sh -

mkdir -p ~/.kube
sudo -u tux mkdir -p ~tux/.kube
for homedir in ~ ~tux; do file::copy /etc/rancher/k3s/k3s.yaml "$homedir"/.kube/config; done
k8s::configure_shell

k8s::wait_for_all
