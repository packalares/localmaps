GO ?= go

# Coverage thresholds track docs/09-testing.md. Keep them in sync.
GO_COVERAGE_THRESHOLD ?= 60
UI_COVERAGE_THRESHOLD ?= 50

COVERAGE_DIR := coverage-report
GO_COVERAGE_DIR := $(COVERAGE_DIR)/go
UI_COVERAGE_DIR := $(COVERAGE_DIR)/ui

.PHONY: build build-gateway build-worker build-worker-docker test test-server test-worker lint tidy \
	coverage coverage-go coverage-ui

build: build-gateway build-worker

build-gateway:
	$(GO) build -o bin/localmaps ./server/cmd/localmaps

build-worker:
	$(GO) build -o bin/localmaps-worker ./worker/cmd/worker

test: test-server test-worker

test-server:
	cd server && $(GO) test ./...

test-worker:
	cd worker && $(GO) test ./...

lint:
	cd server && $(GO) vet ./...
	cd worker && $(GO) vet ./...

tidy:
	$(GO) mod tidy

# ---------------------------------------------------------------------------
# Coverage (Phase 7 — Agent X).
#
# `make coverage` is the umbrella that produces:
#   - coverage-report/go/coverage.out  (atomic go cover profile)
#   - coverage-report/go/coverage.html (HTML report)
#   - coverage-report/ui/              (vitest v8 HTML report)
# Both targets enforce the thresholds from docs/09-testing.md and exit
# non-zero when the total is below the gate so CI bounces regressions.
# ---------------------------------------------------------------------------

coverage: coverage-go coverage-ui

coverage-go:
	@mkdir -p $(GO_COVERAGE_DIR)
	@echo "==> go test (server + worker + shared internal/) with coverage"
	$(GO) test -covermode=atomic \
		-coverprofile=$(GO_COVERAGE_DIR)/coverage.out \
		./server/... ./worker/... ./internal/...
	@$(GO) tool cover -html=$(GO_COVERAGE_DIR)/coverage.out \
		-o $(GO_COVERAGE_DIR)/coverage.html
	@echo "==> go coverage summary"
	@$(GO) tool cover -func=$(GO_COVERAGE_DIR)/coverage.out \
		| tee $(GO_COVERAGE_DIR)/summary.txt
	@total=$$(tail -n1 $(GO_COVERAGE_DIR)/summary.txt \
		| awk '{print $$NF}' | tr -d '%'); \
	 echo "total: $$total% (gate: $(GO_COVERAGE_THRESHOLD)%)"; \
	 awk -v t="$$total" -v g=$(GO_COVERAGE_THRESHOLD) \
	   'BEGIN{ if (t+0 < g+0){ print "FAIL: go coverage below gate"; exit 1 } \
	           else { print "PASS: go coverage >= gate"; exit 0 } }'

coverage-ui:
	@mkdir -p $(UI_COVERAGE_DIR)
	cd ui && npm run test:coverage

# ---------------------------------------------------------------------------
# Worker docker image (local build for compose).
# ---------------------------------------------------------------------------

build-worker-docker:
	docker build -f deploy/Dockerfile.worker -t localmaps/worker:dev .
