# Makefile for sq.
#
# This Makefile is provided as a developer convenience. The GitHub CI builds
# do not make use of it. The Makefile has traditionally been actively maintained
# for macOS dev work, but should work on Linux.


# Suppress macOS linker warning about duplicate -lm from CGO/SQLite
# dependencies. Previously this annoying warning was printed:
#
#   ld: warning: ignoring duplicate libraries: '-lm'
#
# This is a macOS linker warning (common on newer Xcode versions) caused by CGO
# dependencies (the SQLite library) passing -lm multiple times. The warning is
# harmless but annoying. Only apply this linker flag on macOS.
ifeq ($(shell uname),Darwin)
export CGO_LDFLAGS  := -Wl,-no_warn_duplicate_libraries
endif

PKG 				:= github.com/neilotoole/sq
VERSION_PKG 		:= $(PKG)/cli/buildinfo
BUILD_VERSION     	:= $(shell git describe --tags --always --dirty)
BUILD_COMMIT      	:= $(shell git rev-parse HEAD)
BUILD_TIMESTAMP		:= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS				:= -X $(VERSION_PKG).Version=$(BUILD_VERSION) -X $(VERSION_PKG).Commit=$(BUILD_COMMIT) -X $(VERSION_PKG).Timestamp=$(BUILD_TIMESTAMP)
BUILD_TAGS  		:= sqlite_vtable sqlite_stat4 sqlite_fts5 sqlite_icu sqlite_introspect sqlite_json sqlite_math_functions

# Suppress "ld: warning: ignoring duplicate libraries" on macOS (Xcode 15+)
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	CGO_LDFLAGS := -Wl,-no_warn_duplicate_libraries
	export CGO_LDFLAGS
endif

.PHONY: all
all: gen fmt lint test build install

.PHONY: test
test:
	@# Run all tests.
	@go test -tags "$(BUILD_TAGS)" ./...

.PHONY: test-short
test-short:
	@# Run tests with -short flag, skipping long-running tests.
	@# See: https://pkg.go.dev/testing#Short
	@go test -short -tags "$(BUILD_TAGS)" ./...

.PHONY: test-duckdb
test-duckdb:
	@# Run only the DuckDB driver tests.
	@go test -tags "$(BUILD_TAGS)" ./drivers/duckdb/...

.PHONY: build
build:
	@# Build binary for the current (local) platform only; output to dist/sq.
	@mkdir -p dist
	@go build -ldflags "$(LDFLAGS)" -tags "$(BUILD_TAGS)" -o dist/sq

.PHONY: install
install:
	@go install -ldflags "$(LDFLAGS)" -tags "$(BUILD_TAGS)"

.PHONY: lint
lint:
	go tool -modfile=tools/golangci-lint/go.mod golangci-lint version
	go tool -modfile=tools/golangci-lint/go.mod golangci-lint run --output.tab.path stdout
	@shellcheck ./install.sh
	@# Repo-wide formatting gate (markdown, JSON, YAML, TOML, SCSS, Go, JS).
	@bun install --frozen-lockfile
	@bunx dprint check
	@# Static analysis for site JS (replaces eslint).
	@bunx biome lint

.PHONY: gen
gen:
	@go generate ./...
	@# Run betteralign on generated code
	@# https://github.com/dkorunic/betteralign
	@go tool -modfile=tools/betteralign/go.mod betteralign -apply ./libsq/ast/internal/slq &> /dev/null | true

.PHONY: fmt
fmt:
	@# Normalize Go import grouping (whole module) and aliasing (only changed
	@# files, since -set-alias is expensive over ./...). Pass --all to alias the
	@# whole module. See the script header for the cost breakdown.
	@bash scripts/fmt-go-imports.sh
	@# Format everything else with dprint: Go via the gofumpt plugin, plus
	@# markdown, JSON, YAML, TOML, SCSS/CSS, and site JS. See dprint.json.
	@bunx dprint fmt

.PHONY: fmt-check
fmt-check:
	@# Check formatting repo-wide with dprint (read-only; does not modify files).
	@# See dprint.json for the plugin set and includes/excludes.
	@bunx dprint check --list-different

.PHONY: goreleaser-verify-config
goreleaser-verify-config:
	@# Validate goreleaser config files (does not build or publish).
	@# Requires goreleaser: https://goreleaser.com/install/
	goreleaser check -f .goreleaser.yml
	goreleaser check -f .goreleaser-darwin.yml
	goreleaser check -f .goreleaser-linux-amd64.yml
	goreleaser check -f .goreleaser-linux-arm64.yml
	goreleaser check -f .goreleaser-windows.yml

.PHONY: goreleaser-build-local-arch
goreleaser-build-local-arch:
	@# Build binary for current platform using goreleaser (does not publish).
	@# Uses --snapshot (no git tag required) and --single-target (current platform only).
	@# Note: Uses platform-specific config since .goreleaser.yml expects prebuilt binaries.
	@# This is just for local testing. It does not prove much about how the
	@# CI pipeline will behave.
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

.PHONY: site-local
site-local:
	@# Start the local sq.io dev server by delegating to site/Makefile.
	@# See site/Makefile and site/CLAUDE.md for the full set of site targets.
	$(MAKE) -C site site-local

