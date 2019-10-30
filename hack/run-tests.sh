#!/usr/bin/env bash

# hack together a script that assumes a kind cluster is running
# run parallel conformance tests against it

KUBECONFIG="$(kind get kubeconfig-path)"
export KUBECONFIG

# hardcodes I hacked in, NUM_NODES=1 is going to cause some conformance
# tests to fail, oh well, I just want some coverage for the moment
PARALLEL="true"
NUM_NODES=1

# ginkgo regexes
SKIP="${SKIP:-}"
FOCUS="${FOCUS:-"\\[Conformance\\]"}"
# FOCUS="${FOCUS:-"ConfigMap.*optional.*\\[Conformance\\]"}"

# if we set PARALLEL=true, skip serial tests set --ginkgo-parallel
if [ "${PARALLEL:-false}" = "true" ]; then
  export GINKGO_PARALLEL=y
  if [ -z "${SKIP}" ]; then
    SKIP="\\[Serial\\]"
  else
    SKIP="\\[Serial\\]|${SKIP}"
  fi
fi

# setting this env prevents ginkgo e2e from trying to run provider setup
export KUBERNETES_CONFORMANCE_TEST='y'
# setting these is required to make RuntimeClass tests work ...
export KUBE_CONTAINER_RUNTIME=remote
export KUBE_CONTAINER_RUNTIME_ENDPOINT=unix:///run/containerd/containerd.sock
export KUBE_CONTAINER_RUNTIME_NAME=containerd

./hack/ginkgo-e2e.sh \
  '--provider=skeleton' "--num-nodes=${NUM_NODES}" \
  "--ginkgo.focus=${FOCUS}" "--ginkgo.skip=${SKIP}" \
  "--report-dir=${ARTIFACTS}" '--disable-log-dump=true'
