#!/usr/bin/env bash

go test -count=10 -bench BenchmarkExecTableDiff -run "" > tablediff.$(date "+%Y%m%d-%H%M%S").bench.txt
