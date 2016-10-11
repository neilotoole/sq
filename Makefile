# Makefile for sq - simple queryer for structured data
# http://github.com/neilotoole/sq

BINARY := sq
BUILD_VERSION=0.32.3
BUILD_TIMESTAMP := $(shell date +'%FT%T%z')

# SOURCES is the .go files for the project, excluding test files
SOURCES := $(shell find .  -name '*.go' ! -name '*_test.go' ! -path "./test/*" ! -path "./tools/*"  ! -path "./build/*" ! -path "./vendor/*")
# SOURCES_NO_GENERATED is SOURCES excluding the generated parser files
SOURCES_NO_GENERATED := $(shell find .  -name '*.go' ! -name '*_test.go' ! -path "./lib/ql/parser/*" ! -path "./test/*" ! -path "./tools/*"  ! -path "./build/*" ! -path "./vendor/*")

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
	@cd ./build/assets && go-bindata -pkg assets -o ../../lib/assets/assets.go .


test: $(SOURCES) fmt
	go test -timeout 10s ./lib/...


testv: $(SOURCES) fmt
	go test -v -timeout 10s ./lib/...


clean:
	rm -f ./sq # Delete the sq binary in the project root
	# rm -vf $(shell which sq)
	rm -rf ./bin/*
	rm -rf ./build/*
	rm -rf ./dist/*

fmt:
	@goimports -w ./lib 	# We use goimports rather than go fmt


lint:
	# Because we want to exclude the generated parser files from linting, we have
	# to invoke golint multiple times (unless I'm doing this wrong)
	@golint ./lib/bootstrap/...
	@golint ./lib/cmd/...
	@golint ./lib/common/...
	@golint ./lib/config/...
	@golint ./lib/driver/...
	@golint ./lib/out/...
	@golint ./lib/ql
	@golint ./lib/shutdown/...
	@golint ./lib/util/...


vet:
	@go vet ./lib/...


install-tools:
	go get -u github.com/golang/lint/golint/...
	go get -u github.com/jteeuwen/go-bindata/...
	go get -u golang.org/x/tools/cmd/goimports/...


list-src:
	@echo $(SOURCES)

build-for-dist: clean build-assets test
	go build $(LDFLAGS) -o bin/$(OS_PLATFORM)/$(BINARY) main.go
	cp -vf bin/$(OS_PLATFORM)/$(BINARY) $(GOPATH)/bin/

dist: clean test build-for-dist
	mkdir -p ./dist && cd ./dist && cp $(shell which sq) sq && tar -cvzf "sq-$(BUILD_VERSION)-darwin.tar.gz" sq && rm sq

smoke:
	@./test/smoke/smoke.sh