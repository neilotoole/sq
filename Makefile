# Makefile for sq - simple queryer for structured data
# http://github.com/neilotoole/sq

BINARY := sq
BUILD_VERSION=0.41.6
BUILD_TIMESTAMP := $(shell date +'%FT%T%z')

# SOURCES is the .go files for the project, excluding test files
SOURCES := $(shell find .  -name '*.go' ! -name '*_test.go' ! -path "./test/*" ! -path "./tools/*"  ! -path "./build/*" ! -path "./vendor/*")
# SOURCES_NO_GENERATED is SOURCES excluding the generated parser files
SOURCES_NO_GENERATED := $(shell find .  -name '*.go' ! -name '*_test.go' ! -path "./libsq/slq/*" ! -path "./test/*" ! -path "./tools/*"  ! -path "./build/*" ! -path "./vendor/*")

# LDFLAGS are compiler flags, in this case to strip the binary of unneeded stuff
LDFLAGS="-s -w"


default: install


install: $(SOURCES) imports build-assets
	@go install


build-assets:
	@mkdir -p ./build/assets
	@echo $(BUILD_VERSION) > ./build/assets/build_version.txt
	@echo $(BUILD_TIMESTAMP) > ./build/assets/build_timestamp.txt
	@cd ./build/assets && go-bindata -pkg assets -o ../../cmd/assets/assets.go .


test: $(SOURCES) imports
	govendor test -timeout 10s ./libsq/...
	govendor test -timeout 10s ./cmd/...


testv: $(SOURCES) imports
	govendor test -v -timeout 10s ./libsq/...
	govendor test -v -timeout 10s ./cmd/...


clean:
	rm -f ./sq # Delete the sq binary in the project root
	rm -vf $(shell which sq)
	rm -rf ./bin/*
	rm -rf ./build/*
	rm -rf ./dist/*


imports:
	@goimports -w  -srcdir ./vendor/github.com/neilotoole/sq/libsq/ ./libsq/
	@goimports -w  -srcdir ./vendor/github.com/neilotoole/sq/cmd/ ./cmd/


lint:

	@golint ./cmd/...
	@golint ./libsq			# Because we want to exclude the generated parser files from linting, we invoke go lint repeatedly (could be doing this wrong)
	@golint ./libsq/ast/...
	@golint ./libsq/drvr/...
	@golint ./libsq/engine
	@golint ./libsq/shutdown/...
	@golint ./libsq/util/...


vet:
	@go vet ./libsq/...
	@go vet ./cmd/...


check: $(SOURCES) imports lint vet


install-go-tools:
	go get github.com/kardianos/govendor
	go get github.com/golang/lint/golint/...
	go get github.com/jteeuwen/go-bindata/...
	go get golang.org/x/tools/cmd/goimports/...
	go get github.com/karalabe/xgo/...


list-src:
	@echo $(SOURCES)


dist: clean test build-assets
	xgo -go=1.7.1 -dest=./dist -ldflags=$(LDFLAGS) -targets=darwin/amd64,linux/amd64,windows/amd64 .

	mkdir -p ./dist/darwin64
	mv ./dist/sq-darwin-10.6-amd64 ./dist/darwin64/sq
	tar -C ./dist/darwin64 -cvzf ./dist/darwin64/sq-$(BUILD_VERSION)-darwin64.tar.gz sq

	mkdir -p ./dist/linux64
	mv ./dist/sq-linux-amd64 ./dist/linux64/sq
	tar -C ./dist/linux64 -cvzf ./dist/linux64/sq-$(BUILD_VERSION)-linux64.tar.gz sq

	mkdir -p ./dist/win64
	mv ./dist/sq-windows-4.0-amd64.exe ./dist/win64/sq.exe
	zip -jr ./dist/win64/sq-$(BUILD_VERSION)-win64.zip ./dist/win64/sq.exe


smoke:
	@./test/smoke/smoke.sh

generate-parser:
	@cd ./tools && ./gen-antlr.sh

start-test-containers:
	cd ./test/mysql && ./start.sh
	cd ./test/postgres && ./start.sh

