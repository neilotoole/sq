# Makefile for sq - simple queryer for structured data
# http://github.com/neilotoole/sq

BINARY := sq
BUILD_VERSION=0.34.0
BUILD_TIMESTAMP := $(shell date +'%FT%T%z')

# SOURCES is the .go files for the project, excluding test files
SOURCES := $(shell find .  -name '*.go' ! -name '*_test.go' ! -path "./test/*" ! -path "./tools/*"  ! -path "./build/*" ! -path "./vendor/*")
# SOURCES_NO_GENERATED is SOURCES excluding the generated parser files
SOURCES_NO_GENERATED := $(shell find .  -name '*.go' ! -name '*_test.go' ! -path "./libsq/slq/*" ! -path "./test/*" ! -path "./tools/*"  ! -path "./build/*" ! -path "./vendor/*")

ifeq ($(shell uname),Darwin) #Mac OS
	OS_PLATFORM := darwin
	OS_PLATFORM_NAME := Mac OS
else
	OS_PLATFORM := linux
	OS_PLATFORM_NAME := Linux
endif

LDFLAGS=-ldflags "-s -w"


default: install


install: $(SOURCES) fmt build-assets
	@go install


build-assets:
	@mkdir -p ./build/assets
	@echo $(BUILD_VERSION) > ./build/assets/build_version.txt
	@echo $(BUILD_TIMESTAMP) > ./build/assets/build_timestamp.txt
	@cd ./build/assets && go-bindata -pkg assets -o ../../cmd/assets/assets.go .


test: $(SOURCES) fmt
	go test -timeout 10s ./libsq/...
	go test -timeout 10s ./cmd/...


testv: $(SOURCES) fmt
	go test -v -timeout 10s ./libsq/...
	go test -v -timeout 10s ./cmd/...


clean:
	rm -f ./sq # Delete the sq binary in the project root
	# rm -vf $(shell which sq)
	rm -rf ./bin/*
	rm -rf ./build/*
	rm -rf ./dist/*

fmt:
	@goimports -w ./libsq/ 	# We use goimports rather than go fmt
	@goimports -w ./cmd/


lint:
	# Because we want to exclude the generated parser files from linting, we have
	# to invoke golint multiple times (unless I'm doing this wrong)
	@golint ./cmd/...
	@golint ./libsq
	@golint ./libsq/ast/...
	@golint ./libsq/drvr/...
	@golint ./libsq/engine
	@golint ./libsq/shutdown/...
	@golint ./libsq/util/...



vet:
	@go vet ./libsq/...
	@go vet ./cmd/...


install-go-tools:
	go get github.com/golang/lint/golint/...
	go get github.com/jteeuwen/go-bindata/...
	go get golang.org/x/tools/cmd/goimports/...


list-src:
	@echo $(SOURCES)

build-for-dist: clean build-assets test
	go build $(LDFLAGS) -o bin/$(OS_PLATFORM)/$(BINARY) main.go
	cp -vf bin/$(OS_PLATFORM)/$(BINARY) $(GOPATH)/bin/

dist: clean test build-for-dist
	mkdir -p ./dist && cd ./dist && cp $(shell which sq) sq && tar -cvzf "sq-$(BUILD_VERSION)-darwin.tar.gz" sq && rm sq

smoke:
	@./test/smoke/smoke.sh

generate-parser:
	@cd ./tools && ./gen-antlr.sh

