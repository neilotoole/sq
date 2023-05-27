PKG 				:= github.com/neilotoole/sq
VERSION_PKG 		:= $(PKG)/cli/buildinfo
BUILD_VERSION     	:= $(shell git describe --tags --always --dirty)
BUILD_COMMIT      	:= $(shell git rev-parse HEAD)
BUILD_TIMESTAMP		:= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS				?= -s -w -X $(VERSION_PKG).Version=$(BUILD_VERSION) -X $(VERSION_PKG).Commit=$(BUILD_COMMIT) -X $(VERSION_PKG).Timestamp=$(BUILD_TIMESTAMP)


.PHONY: test
test:
	@go test ./...

.PHONY: install
install:
	@go install -ldflags "$(LDFLAGS)"

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
