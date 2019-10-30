#!/usr/bin/env bash

# kind create cluster

# assumes a kind cluster already exists
KUBECONFIG=$(kind get kubeconfig-path)
export KUBECONFIG

make client

make image
kind load docker-image gcr.io/spiffxp-gke-dev/k8s-api-coverage:local

kubectl create -f ./manifests/service-account.yaml
kubectl delete -f ./manifests/apicoverage-webhook.yaml
kubectl create -f ./manifests/apicoverage-webhook.yaml
sleep 2
kubectl logs -l name=apicoverage-webhook -n k8s-api-coverage -f
