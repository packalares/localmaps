# LocalMaps gateway — dev targets. See docs/10-deploy.md for production builds.

GO         ?= go
BIN        ?= ./bin/localmaps
PKG        := ./...
LDFLAGS    := -X main.buildVersion=$(shell git describe --tags --always 2>/dev/null || echo dev) \
              -X main.buildCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none) \
              -X main.buildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build test lint run run-worker

build:
	mkdir -p $(dir $(BIN))
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/localmaps

test:
	$(GO) test $(PKG)

lint:
	$(GO) vet $(PKG)

run: build
	LOCALMAPS_DATA_DIR=$(CURDIR)/.data $(BIN)

run-worker: build
	LOCALMAPS_DATA_DIR=$(CURDIR)/.data LOCALMAPS_MODE=worker $(BIN)
