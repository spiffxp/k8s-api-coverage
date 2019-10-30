REPO ?= gcr.io/spiffxp-gke-dev
IMAGE ?= k8s-api-coverage
TAG ?= local

all: build

build: client server

client:
	go build ./cmd/k8s-api-coverage-client

server:
	go build ./cmd/k8s-api-coverage-server

image:
	docker build -t $(REPO)/$(IMAGE):$(TAG) .

.PHONY: all build client server image
