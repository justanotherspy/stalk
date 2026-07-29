# ==============================================================================
# go-template — developer Makefile
#
# Run `make help` to see all targets.
# Tool versions are pinned below and mirrored in the GitHub Actions workflows
# and .goreleaser.yaml. Bump them together.
# ==============================================================================

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# ---- Project ----------------------------------------------------------------
BINARY   := go-template
MAIN_PKG := ./cmd/$(BINARY)
BIN_DIR  := bin
DIST_DIR := dist
COVERAGE := coverage.out

# ---- Version / build metadata ----------------------------------------------
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.0")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# ---- Pinned tool versions ---------------------------------------------------
GOLANGCI_LINT_VERSION := v2.12.2
GORELEASER_VERSION    := v2.16.0
GOTESTSUM_VERSION     := v1.13.0
GOVULNCHECK_VERSION   := latest
GOPLS_VERSION         := latest
ACTIONLINT_VERSION    := v1.7.12
BENCHSTAT_VERSION     := latest

GO    ?= go
GOBIN := $(shell $(GO) env GOPATH)/bin
export GOTOOLCHAIN ?= auto
export PATH := $(GOBIN):$(PATH)

# ---- Test / benchmark / fuzz / profile knobs --------------------------------
# Override on the command line, e.g. `make bench BENCH=BenchmarkParse`.
# (Keep these free of trailing inline comments: Make bakes the trailing
# whitespace into the value, which corrupts paths and flags.)
FUZZ        ?=
FUZZPKG     ?= ./...
FUZZTIME    ?= 30s
FUZZTIME_CI ?= 1m
BENCH       ?= .
BENCHPKG    ?= ./...
BENCHTIME   ?= 1s
BENCHCOUNT  ?= 6
BENCHFILE   ?= bench-new.txt
PROFPKG     ?= ./internal/examples
PROFILE_DIR ?= profiles

# ==============================================================================
.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---- Dependencies & tooling -------------------------------------------------
.PHONY: deps
deps: ## Download and verify Go module dependencies
	$(GO) mod download
	$(GO) mod verify

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	$(GO) mod tidy

.PHONY: tools
tools: golangci-lint goreleaser gotestsum govulncheck lsp benchstat ## Install all pinned dev tools

.PHONY: check-tools
check-tools: ## Verify required tools are installed
	@missing=0; \
	for t in $(GO) git golangci-lint goreleaser govulncheck gopls; do \
		if command -v $$t >/dev/null 2>&1; then echo "  [ok] $$t"; \
		else echo "  [--] $$t  (run: make tools)"; missing=1; fi; \
	done; \
	exit $$missing

.PHONY: hooks
hooks: ## Install git pre-commit/pre-push hooks (requires pre-commit)
	@command -v pre-commit >/dev/null 2>&1 || { \
		echo ">> pre-commit not found; install from https://pre-commit.com"; exit 1; }
	pre-commit install --install-hooks
	pre-commit install --hook-type pre-push

.PHONY: golangci-lint
golangci-lint: ## Install golangci-lint (v2) if missing
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo ">> installing golangci-lint $(GOLANGCI_LINT_VERSION)"; \
		$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); }

.PHONY: goreleaser
goreleaser: ## Install GoReleaser if missing
	@command -v goreleaser >/dev/null 2>&1 || { \
		echo ">> installing goreleaser $(GORELEASER_VERSION)"; \
		$(GO) install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION); }

.PHONY: gotestsum
gotestsum: ## Install gotestsum if missing
	@command -v gotestsum >/dev/null 2>&1 || { \
		echo ">> installing gotestsum $(GOTESTSUM_VERSION)"; \
		$(GO) install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); }

.PHONY: govulncheck-install
govulncheck-install: ## (internal) install govulncheck if missing
	@command -v govulncheck >/dev/null 2>&1 || { \
		echo ">> installing govulncheck $(GOVULNCHECK_VERSION)"; \
		$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); }

.PHONY: lsp
lsp: ## Install gopls (Go language server for editors & Claude Code)
	@command -v gopls >/dev/null 2>&1 || { \
		echo ">> installing gopls $(GOPLS_VERSION)"; \
		$(GO) install golang.org/x/tools/gopls@$(GOPLS_VERSION); }

.PHONY: benchstat
benchstat: ## Install benchstat (benchmark comparison) if missing
	@command -v benchstat >/dev/null 2>&1 || { \
		echo ">> installing benchstat $(BENCHSTAT_VERSION)"; \
		$(GO) install golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION); }

# ---- Quality ----------------------------------------------------------------
.PHONY: fmt
fmt: golangci-lint ## Format code (gofmt + goimports via golangci-lint)
	golangci-lint fmt

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint
	golangci-lint run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint with --fix
	golangci-lint run --fix

.PHONY: modernize
modernize: ## Apply go1.26 modernizers in place (go fix)
	$(GO) fix ./...

.PHONY: modernize-check
modernize-check: ## Report code that go fix would modernize (fails if any; CI)
	$(GO) fix -diff ./...

.PHONY: actionlint
actionlint: ## Lint GitHub Actions workflows (runs shellcheck on run: blocks if present)
	@command -v actionlint >/dev/null 2>&1 || { \
		echo ">> installing actionlint $(ACTIONLINT_VERSION)"; \
		$(GO) install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION); }
	actionlint

# ---- Tests ------------------------------------------------------------------
# Coverage excludes pure entrypoints that aren't unit-tested by design: the
# cmd/* main packages (the binary and the gen-docs doc generator). Override the
# pattern (an extended regexp matched against coverage.out paths) to change it,
# e.g. `make test COVER_EXCLUDE='/cmd/|/internal/examples/'`.
COVER_EXCLUDE ?= /cmd/

# Drop excluded files from the profile in place, preserving the leading
# `mode:` line. No-op when COVER_EXCLUDE is empty.
define filter_coverage
	@if [ -n "$(COVER_EXCLUDE)" ] && [ -f "$(COVERAGE)" ]; then \
		grep -v -E "$(COVER_EXCLUDE)" "$(COVERAGE)" > "$(COVERAGE).tmp" && mv "$(COVERAGE).tmp" "$(COVERAGE)"; \
	fi
endef

.PHONY: test
test: ## Run tests with the race detector and coverage
	$(GO) test -race -covermode=atomic -coverprofile=$(COVERAGE) ./...
	$(filter_coverage)

.PHONY: test-pretty
test-pretty: gotestsum ## Run tests with pretty output (gotestsum)
	gotestsum -- -race -covermode=atomic -coverprofile=$(COVERAGE) ./...
	$(filter_coverage)

.PHONY: cover
cover: test ## Print per-function coverage summary
	$(GO) tool cover -func=$(COVERAGE)

.PHONY: cover-html
cover-html: test ## Open the HTML coverage report
	$(GO) tool cover -html=$(COVERAGE)

.PHONY: cover-total
cover-total: ## Print the total coverage percentage (needs an existing coverage.out)
	@$(GO) tool cover -func=$(COVERAGE) | awk '/^total:/ {print $$3}'

.PHONY: cover-report
cover-report: ## Emit a Markdown coverage report to stdout (used by CI to comment on PRs)
	@total=$$($(GO) tool cover -func=$(COVERAGE) | awk '/^total:/ {print $$3}'); \
	printf '### 🧪 Code coverage: %s\n\n' "$$total"; \
	printf '<details><summary>Per-function coverage</summary>\n\n'; \
	printf '```\n'; \
	$(GO) tool cover -func=$(COVERAGE); \
	printf '```\n\n</details>\n'

# ---- Fuzzing ----------------------------------------------------------------
# Seed corpora run as ordinary unit tests under `make test`. These targets do
# active, mutation-based fuzzing; crashers are written to testdata/fuzz/<Fuzz>/
# next to the test — commit them as regression seeds.
.PHONY: fuzz
fuzz: ## Actively fuzz ONE target: make fuzz FUZZ=FuzzName [FUZZPKG=./pkg FUZZTIME=1m]
	@if [ -z "$(FUZZ)" ]; then echo ">> set FUZZ=FuzzName (a single fuzz target)"; exit 2; fi
	$(GO) test -run '^$$' -fuzz '^$(FUZZ)$$' -fuzztime $(FUZZTIME) $(FUZZPKG)

.PHONY: fuzz-all
fuzz-all: ## Briefly fuzz every target in the module (used by the nightly workflow)
	@set -euo pipefail; \
	for pkg in $$($(GO) list ./...); do \
		for fn in $$($(GO) test -list '^Fuzz' $$pkg 2>/dev/null | grep -E '^Fuzz' || true); do \
			echo ">> fuzzing $$fn ($$pkg) for $(FUZZTIME_CI)"; \
			$(GO) test -run '^$$' -fuzz "^$$fn$$" -fuzztime $(FUZZTIME_CI) $$pkg; \
		done; \
	done

# ---- Benchmarks & profiling -------------------------------------------------
.PHONY: bench
bench: ## Run benchmarks: make bench [BENCH=BenchmarkX BENCHPKG=./... BENCHTIME=1s]
	$(GO) test -run '^$$' -bench '$(BENCH)' -benchmem -benchtime $(BENCHTIME) $(BENCHPKG)

.PHONY: bench-save
bench-save: ## Run benchmarks BENCHCOUNT times into BENCHFILE (for benchstat)
	$(GO) test -run '^$$' -bench '$(BENCH)' -benchmem -count=$(BENCHCOUNT) $(BENCHPKG) | tee $(BENCHFILE)

.PHONY: benchstat-cmp
benchstat-cmp: benchstat ## Compare bench-old.txt vs bench-new.txt with benchstat
	benchstat bench-old.txt bench-new.txt

.PHONY: profile
profile: ## CPU+mem profile a benchmark into PROFILE_DIR: make profile BENCH=BenchmarkX
	@mkdir -p $(PROFILE_DIR)
	$(GO) test -run '^$$' -bench '$(BENCH)' -benchmem \
		-cpuprofile $(PROFILE_DIR)/cpu.prof \
		-memprofile $(PROFILE_DIR)/mem.prof \
		-o $(PROFILE_DIR)/bench.test $(PROFPKG)
	@echo ">> open: make pprof-cpu   (or)   go tool pprof -http=: $(PROFILE_DIR)/cpu.prof"

.PHONY: pprof-cpu
pprof-cpu: ## Open the CPU profile in the pprof web UI (defaults to the flame graph)
	$(GO) tool pprof -http=: $(PROFILE_DIR)/cpu.prof

.PHONY: pprof-mem
pprof-mem: ## Open the memory profile in the pprof web UI
	$(GO) tool pprof -http=: $(PROFILE_DIR)/mem.prof

# ---- Build / run ------------------------------------------------------------
.PHONY: build
build: ## Build the binary into ./bin
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(MAIN_PKG)

.PHONY: install
install: ## go install the binary
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' $(MAIN_PKG)

.PHONY: run
run: ## Run the CLI (pass args via ARGS="...")
	$(GO) run $(MAIN_PKG) $(ARGS)

# ---- Docs (completions & man pages) -----------------------------------------
# gen-docs writes both ./completions and ./man; the targets below are aliases so
# `make completions` / `make man` read naturally. These dirs are bundled into
# the release archives (see .goreleaser.yaml) and are git-ignored.
.PHONY: completions
completions: ## Generate shell completions into ./completions
	$(GO) run ./cmd/gen-docs

.PHONY: man
man: ## Generate man pages into ./man
	$(GO) run ./cmd/gen-docs

.PHONY: dist-extras
dist-extras: ## Generate completions + man pages
	$(GO) run ./cmd/gen-docs

# ---- Container --------------------------------------------------------------
IMAGE     ?= go-template
IMAGE_TAG ?= dev

.PHONY: docker-build
docker-build: ## Build a local container image (IMAGE=go-template IMAGE_TAG=dev)
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		-t $(IMAGE):$(IMAGE_TAG) .

.PHONY: docker-run
docker-run: ## Run the local container image (pass args via ARGS="...")
	docker run --rm $(IMAGE):$(IMAGE_TAG) $(ARGS)

# ---- Security ---------------------------------------------------------------
.PHONY: vuln
vuln: govulncheck-install ## Scan dependencies for known vulnerabilities
	govulncheck ./...

.PHONY: secrets
secrets: ## Scan the working tree for committed secrets (requires trufflehog)
	@command -v trufflehog >/dev/null 2>&1 || { \
		echo ">> trufflehog not found; install from https://github.com/trufflesecurity/trufflehog"; exit 1; }
	trufflehog --no-update filesystem . --results=verified,unknown --fail

.PHONY: zizmor
zizmor: ## Audit GitHub Actions workflows for security issues (requires zizmor; `pipx install zizmor`)
	@command -v zizmor >/dev/null 2>&1 || { \
		echo ">> zizmor not found; install with: pipx install zizmor"; exit 1; }
	zizmor --min-severity=high .github/workflows

.PHONY: security
security: vuln ## Run all local security checks
	@echo ">> security checks complete"

# ---- Release ----------------------------------------------------------------
.PHONY: release-check
release-check: goreleaser ## Validate the GoReleaser configuration
	goreleaser check

.PHONY: snapshot
snapshot: goreleaser ## Build a local snapshot release (no publish)
	goreleaser release --snapshot --clean --skip=sign,sbom

# ---- Aggregates -------------------------------------------------------------
.PHONY: ci
ci: deps lint modernize-check test build ## Run the pipeline that CI runs

.PHONY: all
all: tidy fmt modernize lint test build ## Tidy, format, modernize, lint, test, and build

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR) $(COVERAGE) $(PROFILE_DIR) bench-*.txt *.prof *.test
