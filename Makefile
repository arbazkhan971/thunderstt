# ThunderSTT Makefile
# Build, test, lint, and deploy targets

BINARY      := thunderstt
MODULE      := github.com/arbaz/thunderstt
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE  ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS     := -s -w \
	-X main.Version=$(VERSION) \
	-X main.Commit=$(COMMIT) \
	-X main.BuildDate=$(BUILD_DATE)

GO          := go
GOFLAGS     ?=
CGO_ENABLED ?= 0

.PHONY: all build build-cgo test test-verbose test-race bench lint vet fmt \
	clean docker-build docker-build-gpu run help

all: lint test build ## Build after lint and test

## Build targets

build: ## Build the binary (no CGO)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/thunderstt/

build-cgo: ## Build with CGO enabled (sherpa-onnx support)
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/thunderstt/

install: ## Install binary to GOPATH/bin
	CGO_ENABLED=0 $(GO) install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/thunderstt/

## Test targets

test: ## Run all tests
	CGO_ENABLED=0 $(GO) test ./... -count=1

test-verbose: ## Run all tests with verbose output
	CGO_ENABLED=0 $(GO) test ./... -count=1 -v

test-race: ## Run tests with race detector (requires CGO)
	CGO_ENABLED=1 $(GO) test ./... -count=1 -race

test-cover: ## Run tests with coverage report
	CGO_ENABLED=0 $(GO) test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out
	@echo "HTML report: go tool cover -html=coverage.out"

bench: ## Run benchmarks
	CGO_ENABLED=0 $(GO) test ./... -bench=. -benchmem -run='^$$' -count=1

## Code quality

lint: vet fmt ## Run all linters
	@echo "Lint passed"

vet: ## Run go vet
	CGO_ENABLED=0 $(GO) vet ./...

fmt: ## Check formatting
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

tidy: ## Tidy go modules
	$(GO) mod tidy

## Docker targets

docker-build: ## Build CPU Docker image
	docker build -t $(BINARY):latest -f docker/Dockerfile .

docker-build-gpu: ## Build GPU Docker image
	docker build -t $(BINARY):latest-gpu -f docker/Dockerfile.gpu .

docker-compose-up: ## Start services via docker compose
	cd docker && docker compose up -d

docker-compose-down: ## Stop services
	cd docker && docker compose down

## Run targets

run: build ## Build and run the server
	./bin/$(BINARY) serve --model parakeet-tdt-0.6b-v3

run-dev: build ## Build and run with debug logging
	./bin/$(BINARY) serve --model parakeet-tdt-0.6b-v3 --log-level debug

## Utility

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out

version: ## Print version info
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
