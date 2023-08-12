#!/bin/sh

set -e

dest_dir="../libsq/ast/internal/slq"
mkdir -p $dest_dir

echo "Generating SLQ parser code from grammar..."
alias antlr4='java -Xmx500M -cp "./antlr-4.13.0-complete.jar:$CLASSPATH" org.antlr.v4.Tool'
antlr4 -Dlanguage=Go -listener -visitor -o $dest_dir -package slq SLQ.g4

echo "Verifying that generated files can build..."
go build -v $dest_dir
echo "Generated files are OK."
