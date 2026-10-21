.PHONY: help all check test test-race coverage coverage-html coverage-check coverage-clean fmt fmt-check vet lint lint-ci vuln fuzz bench tidy build build-all install golden official nsd-fetch nsd-lock nsd-check nsd-generate nsd-generate-check nsd-publish nsd-publish-check nsd-clean clean

help: ## This help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

# Ignore a parent go.work (e.g. otfabric/go.work) so this module builds standalone.
export GOWORK := off

# CI formats and lints with the Go version from go.mod, not with the toolchain installed here.
# gofmt's output changes between Go releases, so `fmt` uses that version and `fmt-check` also
# requires the local gofmt to agree: a file two versions format differently fails on one side.
GO_MOD_VERSION := $(shell sed -nE 's/^go ([0-9]+\.[0-9]+).*/\1/p' go.mod | head -n1)
GOFMT_CI = $(shell GOTOOLCHAIN=go$(GO_MOD_VERSION).0 go env GOROOT)/bin/gofmt

PKGS         := ./...
COVERAGE_MIN := 90
FUZZTIME     ?= 15s

# Official IEC NSD packages: temporary inputs, never committed. NSD_CACHE must be
# outside the repository or the git-ignored .nsd-cache; generated output goes to
# GENERATED_DIR (git-ignored, and dot-named so that ./... never compiles it) and
# is local-only unless the lock permits publishing.
NSD_CACHE     ?= .nsd-cache
NSD_TIERS     ?= all
NSD_PROFILE   ?=
GENERATED_DIR ?= .generated
GO_PACKAGE    ?= iec61850names
# The committed output generated from the official packages, as the lock permits:
# NAMES_DIR, the Go names of the default profile; OFFICIAL_DIR, the embedded
# catalog of the extended profile; REFERENCE_DIR, the reference documentation of
# the extended profile. They are generated with the Go version of go.mod, as CI
# does, so that the output does not depend on the local toolchain.
NAMES_DIR     ?= names
OFFICIAL_DIR  ?= official
REFERENCE_DIR ?= docs/reference
PUBLISH        = --publish --lock sources/nsd.lock.json
NSDCTL_PINNED := GOTOOLCHAIN=go$(GO_MOD_VERSION).0 go run ./cmd/nsdctl
NSDCTL        := go run ./cmd/nsdctl
NSD_SOURCE     = -s $(NSD_CACHE) $(foreach p,$(NSD_PROFILE),-p "$(p)")

DIST      := dist
PLATFORMS := linux/amd64 linux/arm64 linux/arm/v7 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
PREFIX    ?= /usr/local

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
TAG        ?= $(shell git describe --tags --exact-match 2>/dev/null || echo "")
COMMIT     ?= $(shell git rev-parse --short=8 HEAD 2>/dev/null || echo "")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w -X main.version=$(VERSION) -X main.tag=$(TAG) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

all: check build ## Run all checks and build

check: fmt fmt-check tidy vet lint lint-ci vuln test-race coverage-check ## Run all pre-commit checks

test: ## Run unit tests
	@echo "Running unit tests"
	@go test -count=1 $(PKGS)

test-race: ## Run unit tests with the race detector
	@echo "Running tests (race detector)"
	@go test -count=1 -race $(PKGS)

coverage: ## Run tests with coverage profile and text summary
	@echo "Running tests with coverage"
	@go test -count=1 -coverprofile=coverage.out -covermode=atomic $(PKGS)
	@go tool cover -func=coverage.out | tee coverage.txt

coverage-html: coverage ## Generate HTML coverage report (coverage.html)
	@echo "Generating HTML coverage report"
	@go tool cover -html=coverage.out -o coverage.html

coverage-check: coverage ## Fail if total coverage is below COVERAGE_MIN
	@go tool cover -func=coverage.out | grep 'total:' | awk -v min=$(COVERAGE_MIN) '{gsub(/%/,""); p=$$NF+0; if (p < min) { printf "Coverage %.1f%% is below %d%%\n", p, min; exit 1 } else { printf "Coverage %.1f%% (>= %d%%)\n", p, min } }'

coverage-clean: ## Remove coverage artifacts
	@echo "Removing coverage artifacts"
	@rm -f coverage.out coverage.txt coverage.html

fmt: ## Format Go code with the gofmt of the Go version in go.mod (what CI uses)
	@echo "Running gofmt (go$(GO_MOD_VERSION))"
	@$(GOFMT_CI) -w .

fmt-check: ## Fail if the go.mod-version gofmt or the local gofmt would change any file
	@echo "Checking formatting (gofmt go$(GO_MOD_VERSION) and local $$(go env GOVERSION))"
	@ci="$$($(GOFMT_CI) -l .)"; loc="$$(gofmt -l .)"; \
	if [ -n "$$ci$$loc" ]; then \
		[ -z "$$ci" ] || { echo "Not formatted for gofmt go$(GO_MOD_VERSION) (CI):"; echo "$$ci"; }; \
		[ -z "$$loc" ] || { echo "Not formatted for the local gofmt:"; echo "$$loc"; }; \
		echo "Files must be stable under both. Run 'make fmt'; if the two versions still disagree,"; \
		echo "restructure the code they format differently (e.g. split long lines yourself)."; \
		exit 1; \
	fi

vet: ## Run go vet
	@echo "Running go vet"
	@go vet $(PKGS)

lint: ## Run staticcheck
	@echo "Running staticcheck"
	@staticcheck $(PKGS)

lint-ci: ## Run golangci-lint (uses .golangci.yml)
	@echo "Running golangci-lint"
	@golangci-lint run $(PKGS)

vuln: ## Run govulncheck
	@echo "Running govulncheck"
	@govulncheck $(PKGS)

fuzz: ## Run all fuzz targets for FUZZTIME each (default 15s)
	@echo "Fuzzing nsd.Parse"
	@go test -run='^$$' -fuzz=FuzzParse -fuzztime=$(FUZZTIME) ./nsd
	@echo "Fuzzing reference.Parse"
	@go test -run='^$$' -fuzz=FuzzParse -fuzztime=$(FUZZTIME) ./reference

bench: ## Run benchmarks
	@echo "Running benchmarks"
	@go test -run='^$$' -bench=. -benchmem $(PKGS)

tidy: ## Tidy and verify module files
	@echo "Running go mod tidy"
	@go mod tidy
	@echo "Running go mod verify"
	@go mod verify

build: ## Build all packages and bin/nsdctl
	@echo "Building all packages"
	@go build $(PKGS)
	@mkdir -p bin
	@echo "  BUILD nsdctl $(VERSION) -> bin/nsdctl"
	@go build -ldflags '$(LDFLAGS)' -o bin/nsdctl ./cmd/nsdctl

build-all: ## Cross-compile nsdctl for every release target into dist/ (names as in the releases)
	@echo "Cross-compiling nsdctl $(VERSION)"
	@mkdir -p $(DIST)
	@for target in $(PLATFORMS); do \
		os=$$(echo $$target | cut -d/ -f1); arch=$$(echo $$target | cut -d/ -f2); arm=$$(echo $$target | cut -s -d/ -f3 | tr -d v); \
		label=$$os-$$arch; [ -n "$$arm" ] && label=$$os-armv$$arm; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out=$(DIST)/nsdctl-$(VERSION)-$$label$$ext; \
		echo "  BUILD $$target -> $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch GOARM=$$arm go build -trimpath -ldflags '$(LDFLAGS)' -o $$out ./cmd/nsdctl || exit 1; \
	done

install: build ## Install nsdctl to PREFIX/bin (default /usr/local/bin)
	@echo "Installing nsdctl to $(PREFIX)/bin"
	@sudo install -m 755 bin/nsdctl $(PREFIX)/bin/nsdctl

nsd-fetch: ## Download the tracked IEC NSD packages into NSD_CACHE, verified against the lock
	@echo "Fetching IEC NSD packages ($(NSD_TIERS)) into $(NSD_CACHE)"
	@$(NSDCTL) sources fetch --dest "$(NSD_CACHE)" --tiers "$(NSD_TIERS)"

nsd-lock: ## Pin the digests and statuses of the packages in NSD_CACHE into sources/nsd.lock.json
	@echo "Pinning packages in $(NSD_CACHE)"
	@$(NSDCTL) sources lock --packages "$(NSD_CACHE)" --write

nsd-check: ## Check package coverage and validate the namespaces in NSD_CACHE
	@echo "Checking packages in $(NSD_CACHE)"
	@$(NSDCTL) sources check --packages "$(NSD_CACHE)" --tiers "$(NSD_TIERS)"
	@$(NSDCTL) validate "$(NSD_CACHE)"

nsd-generate: ## Generate Go, docs and JSON from NSD_CACHE into GENERATED_DIR (local-only)
	@echo "Generating from $(NSD_CACHE) into $(GENERATED_DIR)"
	@$(NSDCTL) $(NSD_SOURCE) generate go -o "$(GENERATED_DIR)/go" --package "$(GO_PACKAGE)"
	@$(NSDCTL) $(NSD_SOURCE) generate docs -o "$(GENERATED_DIR)/docs"
	@$(NSDCTL) $(NSD_SOURCE) generate json -o "$(GENERATED_DIR)/json"

nsd-generate-check: ## Fail if GENERATED_DIR differs from what NSD_CACHE generates
	@$(NSDCTL) $(NSD_SOURCE) generate go -o "$(GENERATED_DIR)/go" --package "$(GO_PACKAGE)" --check
	@$(NSDCTL) $(NSD_SOURCE) generate docs -o "$(GENERATED_DIR)/docs" --check
	@$(NSDCTL) $(NSD_SOURCE) generate json -o "$(GENERATED_DIR)/json" --check

nsd-publish: ## Regenerate the committed output (names, official, docs/reference) from NSD_CACHE (the lock must permit every namespace)
	@echo "Generating the committed output from $(NSD_CACHE)"
	@$(NSDCTL_PINNED) -s "$(NSD_CACHE)" -p default generate go -o "$(NAMES_DIR)" --package names $(PUBLISH) --force
	@$(NSDCTL_PINNED) -s "$(NSD_CACHE)" -p extended generate catalog -o "$(OFFICIAL_DIR)" --package official $(PUBLISH) --force
	@$(NSDCTL_PINNED) -s "$(NSD_CACHE)" -p extended generate docs -o "$(REFERENCE_DIR)" $(PUBLISH) --force

nsd-publish-check: ## Fail if the committed output differs from what NSD_CACHE generates
	@$(NSDCTL_PINNED) -s "$(NSD_CACHE)" -p default generate go -o "$(NAMES_DIR)" --package names $(PUBLISH) --check
	@$(NSDCTL_PINNED) -s "$(NSD_CACHE)" -p extended generate catalog -o "$(OFFICIAL_DIR)" --package official $(PUBLISH) --check
	@$(NSDCTL_PINNED) -s "$(NSD_CACHE)" -p extended generate docs -o "$(REFERENCE_DIR)" $(PUBLISH) --check

nsd-clean: ## Remove downloaded packages and generated output
	@echo "Removing $(NSD_CACHE) and $(GENERATED_DIR)"
	@rm -rf "$(NSD_CACHE)" "$(GENERATED_DIR)"

official: ## Test against the official packages in NSD_DIR (default NSD_CACHE), read in place
	@test -d "$(or $(NSD_DIR),$(NSD_CACHE))" || { echo "No packages in $(or $(NSD_DIR),$(NSD_CACHE)); run make nsd-fetch"; exit 1; }
	@echo "Testing against official packages in $(or $(NSD_DIR),$(NSD_CACHE))"
	@NSD_DIR="$(abspath $(or $(NSD_DIR),$(NSD_CACHE)))" go test -count=1 -v -run TestOfficialPackages ./cmd/nsdctl

golden: ## Regenerate testdata/golden after an intentional generator output change
	@echo "Regenerating golden generator output"
	@UPDATE_GOLDEN=1 go test -count=1 -run TestGolden ./internal/generator

clean: coverage-clean ## Clean test cache, coverage and build artifacts
	@echo "Cleaning test cache and build artifacts"
	@go clean -testcache
	@rm -rf bin $(DIST)
