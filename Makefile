# Makefile for sq: a local developer convenience entry point.
#
# CI does NOT use this Makefile: the GitHub workflows under .github/workflows/
# invoke go / bun / goreleaser directly. This file is for local development and
# is primarily maintained for macOS, but should work on Linux.
#
# Self-documenting: every target carries a "## help text" comment, and
# `make help` (the default target) prints them. When adding a target, append
# "## <one-line description>" to its rule so it shows up in the help listing.

# Print help instead of running a heavy pipeline when invoked with no target.
.DEFAULT_GOAL := help

# ---- Build metadata (stamped into the binary via -ldflags) ----------------

PKG             := github.com/neilotoole/sq
VERSION_PKG     := $(PKG)/cli/buildinfo
BUILD_VERSION   := $(shell git describe --tags --always --dirty)
BUILD_COMMIT    := $(shell git rev-parse HEAD)
BUILD_TIMESTAMP := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS         := -X $(VERSION_PKG).Version=$(BUILD_VERSION) -X $(VERSION_PKG).Commit=$(BUILD_COMMIT) -X $(VERSION_PKG).Timestamp=$(BUILD_TIMESTAMP)

# CGO build tags enabling the SQLite features sq depends on (vtable, FTS5, JSON,
# math functions, etc.). Note: the CI workflows set their own BUILD_TAGS and do
# not currently include sqlite_icu. Keep that in mind if behavior diverges.
BUILD_TAGS := sqlite_vtable sqlite_stat4 sqlite_fts5 sqlite_icu sqlite_introspect sqlite_json sqlite_math_functions

# On macOS (Xcode 15+), CGO/SQLite pass -lm to the linker more than once, which
# prints a harmless but noisy "ld: warning: ignoring duplicate libraries: '-lm'".
# Suppress it here; no-op on other platforms.
ifeq ($(shell uname -s),Darwin)
export CGO_LDFLAGS := -Wl,-no_warn_duplicate_libraries
endif

# ---- Help -----------------------------------------------------------------

.PHONY: help
help: ## Print this help listing (the default target).
	@awk -F':.*## ' '/^[a-zA-Z0-9_-]+:.*## /{printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---- Build, test, install -------------------------------------------------

.PHONY: all
all: gen fmt lint test build install ## Run the full local pipeline: gen, fmt, lint, test, build, install.

.PHONY: test
test: ## Run all tests (SQL-driver tests need the sakiladb/* Docker images).
	@go test -tags "$(BUILD_TAGS)" ./...

.PHONY: test-short
test-short: ## Run tests with -short (skips long-running / container-backed tests).
	@# See: https://pkg.go.dev/testing#Short
	@go test -short -tags "$(BUILD_TAGS)" ./...

.PHONY: build
build: ## Build the sq binary for the local platform to dist/sq.
	@mkdir -p dist
	@go build -ldflags "$(LDFLAGS)" -tags "$(BUILD_TAGS)" -o dist/sq

.PHONY: install
install: ## Build and install sq into the Go bin dir (GOBIN/GOPATH/bin).
	@go install -ldflags "$(LDFLAGS)" -tags "$(BUILD_TAGS)"

# ---- Dependencies & clone setup -------------------------------------------

.PHONY: deps
deps: ## Install dev dependencies: bun packages (dprint, biome) + Go modules.
	@# bun provides the formatting/lint tooling used by fmt, lint, and the site.
	@bun install --frozen-lockfile
	@# Pre-fetch Go module deps; go build/test would fetch them lazily otherwise.
	@go mod download

.PHONY: init
init: deps ## One-time clone setup: install deps and activate the git hooks.
	@# Point git at the repo's tracked hooks (.githooks) so the pre-commit
	@# formatting check runs. core.hooksPath is per-clone local config (git won't
	@# run cloned hooks until told to), so this is idempotent and run once per
	@# clone. Bypass a hook for a single commit with `git commit --no-verify`.
	@cur=$$(git config --get core.hooksPath 2>/dev/null || true); \
		if [ -n "$$cur" ] && [ "$$cur" != ".githooks" ]; then \
			echo "warning: replacing existing core.hooksPath ($$cur) with .githooks" >&2; \
		fi
	@git config core.hooksPath .githooks
	@echo "Activated git hooks (.githooks). Bypass a hook with 'git commit --no-verify'."

# ---- Code generation, format & lint ---------------------------------------

.PHONY: gen
gen: ## Run code generation (go generate + betteralign on generated code).
	@go generate ./...
	@# betteralign re-orders struct fields in generated code for optimal memory
	@# layout. https://github.com/dkorunic/betteralign
	@go tool -modfile=tools/betteralign/go.mod betteralign -apply ./libsq/ast/internal/slq >/dev/null 2>&1 || true

.PHONY: fmt
fmt: ## Format the whole repo (Go imports + dprint for everything else).
	@# Normalize Go import grouping (whole module) and aliasing (only changed
	@# files, since -set-alias is expensive over ./...). Pass --all to alias the
	@# whole module. See the script header for the cost breakdown.
	@bash scripts/fmt-go-imports.sh
	@# dprint formats everything else: Go via the gofumpt plugin, plus markdown,
	@# JSON, YAML, TOML, SCSS/CSS, and site JS. See dprint.json.
	@bunx dprint fmt

.PHONY: fmt-check
fmt-check: ## Check dprint formatting repo-wide (read-only; does not modify files).
	@# This is the dprint half of the Format CI job (which also runs biome lint;
	@# use `make lint` for the full set). The .githooks/pre-commit hook runs this
	@# same dprint check, but only on staged files. See dprint.json for plugins.
	@bunx dprint check --list-different

.PHONY: lint
lint: deps ## Run linters: golangci-lint, shellcheck, dprint check, biome.
	go tool -modfile=tools/golangci-lint/go.mod golangci-lint version
	go tool -modfile=tools/golangci-lint/go.mod golangci-lint run --output.tab.path stdout
	@shellcheck ./install.sh .githooks/pre-commit scripts/build-db-matrix.sh scripts/dedup-db-matrix.sh scripts/build-db-matrix_test.sh
	@bash scripts/build-db-matrix_test.sh
	@# Repo-wide formatting gate (markdown, JSON, YAML, TOML, SCSS, Go, JS).
	@bunx dprint check
	@# Static analysis for site JS (replaces eslint).
	@bunx biome lint

# ---- Release tooling (local checks; CI runs the real release) -------------

.PHONY: goreleaser-verify-config
goreleaser-verify-config: ## Validate the goreleaser config files (no build/publish).
	@# Requires goreleaser: https://goreleaser.com/install/
	goreleaser check -f .goreleaser.yml
	goreleaser check -f .goreleaser-darwin.yml
	goreleaser check -f .goreleaser-linux-amd64.yml
	goreleaser check -f .goreleaser-linux-arm64.yml
	goreleaser check -f .goreleaser-windows.yml

.PHONY: goreleaser-build-local-arch
goreleaser-build-local-arch: ## Build via goreleaser for the local arch (snapshot; no publish).
	@# Uses --snapshot (no git tag required) and --single-target (current platform
	@# only), with the platform-specific config since .goreleaser.yml expects
	@# prebuilt binaries. Local smoke test only; proves little about how the CI
	@# release pipeline behaves.
ifeq ($(shell uname -s),Darwin)
	goreleaser build --snapshot --clean --single-target -f .goreleaser-darwin.yml
else ifeq ($(shell uname -s),Linux)
  ifeq ($(shell uname -m),aarch64)
	goreleaser build --snapshot --clean --single-target -f .goreleaser-linux-arm64.yml
  else
	goreleaser build --snapshot --clean --single-target -f .goreleaser-linux-amd64.yml
  endif
else
	goreleaser build --snapshot --clean --single-target -f .goreleaser-windows.yml
endif

# ---- Site (sq.io) ---------------------------------------------------------

.PHONY: site-local
site-local: ## Start the local sq.io dev server (delegates to site/Makefile).
	@# See site/Makefile and site/CLAUDE.md for the full set of site targets.
	$(MAKE) -C site site-local
