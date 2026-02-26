#!/usr/bin/env bashp

kubeVersion="$K8S_VERSION" kubeadm::prepare
kubeadm::create_cluster
