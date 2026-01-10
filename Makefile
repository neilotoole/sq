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
# harmless but annoying.
export CGO_LDFLAGS  := -Wl,-no_warn_duplicate_libraries

PKG 				:= github.com/neilotoole/sq
VERSION_PKG 		:= $(PKG)/cli/buildinfo
BUILD_VERSION     	:= $(shell git describe --tags --always --dirty)
BUILD_COMMIT      	:= $(shell git rev-parse HEAD)
BUILD_TIMESTAMP		:= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS				:= -X $(VERSION_PKG).Version=$(BUILD_VERSION) -X $(VERSION_PKG).Commit=$(BUILD_COMMIT) -X $(VERSION_PKG).Timestamp=$(BUILD_TIMESTAMP)
BUILD_TAGS  		:= sqlite_vtable sqlite_stat4 sqlite_fts5 sqlite_icu sqlite_introspect sqlite_json sqlite_math_functions



.PHONY: all
all: gen fmt lint test build install

.PHONY: test
test:
	@go test -tags "$(BUILD_TAGS)" ./...

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

.PHONY: gen
gen:
	@go generate ./...
	@# Run betteralign on generated code
	@# https://github.com/dkorunic/betteralign
	@go tool -modfile=tools/betteralign/go.mod betteralign -apply ./libsq/ast/internal/slq &> /dev/null | true

.PHONY: fmt
fmt:
	@# https://github.com/incu6us/goimports-reviser
	@# Note that termz_windows.go is excluded because the tool seems
	@# to mangle Go code that is guarded by build tags that
	@# are not in use. Alas, we can't provide a double star glob,
	@# e.g. **/*_windows.go, because filepath.Match doesn't support
	@# double star, so we explicitly name the file.
	@go tool -modfile=tools/goimports-reviser/go.mod goimports-reviser \
		-company-prefixes github.com/neilotoole -set-alias \
		-excludes 'libsq/core/termz/termz_windows.go' \
		-rm-unused -output write \
		-project-name github.com/neilotoole/sq ./...

	@# Use gofumpt instead of "go fmt"
	@# https://github.com/mvdan/gofumpt
	@go tool -modfile=tools/gofumpt/go.mod gofumpt -w .

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
