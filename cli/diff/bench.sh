#!/usr/bin/env bash

go test -count=10 -bench BenchmarkExecTableDiff > tablediff.$(date "+%H%M%S").bench.txt
