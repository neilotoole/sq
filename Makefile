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
	@golangci-lint run --out-format tab --sort-results

.PHONY: gen
gen:
	@go generate ./...

.PHONY: fmt
fmt:
	@# Use gofumpt instead of "go fmt"
	@gofumpt -w .
