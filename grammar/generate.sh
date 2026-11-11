#!/bin/sh

set -e

dest_dir="../libsq/ast/internal/slq"
mkdir -p $dest_dir

echo "Generating SLQ parser code from grammar..."
alias antlr4='java -Xmx500M -cp "../tools/antlr-4.13.0-complete.jar:$CLASSPATH" org.antlr.v4.Tool'
antlr4 -Dlanguage=Go -listener -visitor -o $dest_dir -package slq SLQ.g4

# ANTLR's Go target ends each rule function with a "goto errorExit" after the
# return, which go vet reports as unreachable code. fixgoto moves it before the
# return.
go run ../libsq/ast/antlrz/internal/fixgoto "$dest_dir/slq_parser.go"

echo "Verifying that generated files can build and pass go vet..."
go build -v $dest_dir
go vet $dest_dir
echo "Generated files are OK."
