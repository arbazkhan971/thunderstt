APP_NAME   := thunderstt
MODULE     := github.com/arbaz/thunderstt
BIN_DIR    := bin
BUILD_OUT  := $(BIN_DIR)/$(APP_NAME)

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
	-X main.Version=$(VERSION) \
	-X main.Commit=$(COMMIT) \
	-X main.BuildDate=$(BUILD_DATE)

GO       := go
GOFLAGS  :=
DOCKER   := docker
IMAGE    := $(APP_NAME):$(VERSION)

DEFAULT_MODEL ?= base

.PHONY: all build test lint clean docker-build docker-run download-model help

all: build

## build: compile the thunderstt binary
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_OUT) ./cmd/thunderstt

## test: run all tests with race detector
test:
	$(GO) test -race -count=1 -coverprofile=coverage.out ./...
	@echo "coverage report: coverage.out"

## lint: run golangci-lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found, install: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

## docker-build: build Docker image
docker-build:
	$(DOCKER) build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE) .

## docker-run: run the Docker image
docker-run:
	$(DOCKER) run --rm -p 8080:8080 $(IMAGE)

## download-model: download a whisper model (usage: make download-model MODEL=base)
download-model:
	$(GO) run ./cmd/thunderstt download $(or $(MODEL),$(DEFAULT_MODEL))

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) coverage.out

## help: display this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | column -t -s ':'
