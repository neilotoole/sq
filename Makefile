PKG 				:= github.com/neilotoole/sq
VERSION_PKG 		:= $(PKG)/cli/buildinfo
BUILD_VERSION     	:= $(shell git describe --tags --always --dirty)
BUILD_COMMIT      	:= $(shell git rev-parse HEAD)
BUILD_TIMESTAMP		:= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS				:= -X $(VERSION_PKG).Version=$(BUILD_VERSION) -X $(VERSION_PKG).Commit=$(BUILD_COMMIT) -X $(VERSION_PKG).Timestamp=$(BUILD_TIMESTAMP)
BUILD_TAGS  		:= sqlite_vtable sqlite_stat4 sqlite_fts5 sqlite_icu sqlite_introspect sqlite_json sqlite_math_functions

.PHONY: test
test:
	@go test -tags "$(BUILD_TAGS)" ./...

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
