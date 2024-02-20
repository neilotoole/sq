#!/usr/bin/env bash

benches=$(ls -1 | grep ".bench.txt" | sort)

first=$(head -n 1 <<< "$benches")
previous=$(tail -n 2 <<< "$benches" | head -n 1)
last=$(tail -n 1 <<< "$benches")

benchstat "$first" "$previous" "$last"
