MODULE := github.com/gvault/gvault
GO ?= go
BIN_DIR := bin

.PHONY: fmt lint test test-integration test-gpu build run-api run-controller docker-up docker-down docker-build tidy vet gvaultctl

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint: vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; ran go vet only"; \
	fi

test:
	$(GO) test ./... -count=1

test-integration:
	$(GO) test ./tests/integration/... -count=1 -tags=integration

test-gpu:
	$(GO) test ./tests/e2e/... -count=1 -tags=gpu

VERSION ?= $(shell cat VERSION 2>/dev/null || echo 0.1.0-beta.1)
LDFLAGS := -X github.com/gvault/gvault/internal/version.Version=$(VERSION)

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/gvault-api ./cmd/gvault-api
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/gvault-controller ./cmd/gvault-controller
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/gvaultctl ./cmd/gvaultctl

gvaultctl: build
	./$(BIN_DIR)/gvaultctl help

.PHONY: gvaultctl-demo
gvaultctl-demo: build
	./$(BIN_DIR)/gvaultctl health
	./$(BIN_DIR)/gvaultctl overview
	./$(BIN_DIR)/gvaultctl providers


run-api: build
	./$(BIN_DIR)/gvault-api

run-controller: build
	./$(BIN_DIR)/gvault-controller

docker-up:
	docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

docker-build:
	docker build -t gvault/gvault-api:$(VERSION) --build-arg VERSION=$(VERSION) .

tidy:
	$(GO) mod tidy

generate manifests:
	@echo "CRD YAML lives in config/crd/bases (hand-maintained for gvault v1alpha1)"

install-crds:
	kubectl apply -f config/crd/bases/

uninstall-crds:
	kubectl delete -f config/crd/bases/ --ignore-not-found

kustomize-build:
	kubectl kustomize config/kustomize/overlays/nvidia

kustomize-build-nvidia:
	kubectl kustomize config/kustomize/overlays/nvidia

kustomize-build-testharness:
	kubectl kustomize config/kustomize/overlays/fake
