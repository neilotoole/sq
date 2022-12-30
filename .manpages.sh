#!/bin/sh
# This script generates man pages.
set -e
rm -rf manpages
mkdir manpages
go run . man | gzip -c -9 >manpages/sq.1.gz
