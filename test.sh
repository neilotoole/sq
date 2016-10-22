#!/usr/bin/env bash

docker build -t sq-build .
docker run --rm -v "${PWD}":/go/src/github.com/neilotoole/sq  sq-build bash -c "make test"
