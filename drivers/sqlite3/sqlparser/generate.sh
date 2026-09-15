#!/bin/sh

set -e

dest_dir="./sqlite"
mkdir -p $dest_dir

echo "Generating parser code from grammar..."
alias antlr4='java -Xmx500M -cp "../../../tools/antlr-4.13.0-complete.jar:$CLASSPATH" org.antlr.v4.Tool'
antlr4 -Dlanguage=Go -listener -visitor -o $dest_dir -package sqlite SQLiteLexer.g4 SQLiteParser.g4

# ANTLR's Go target ends each rule function with a "goto errorExit" after the
# return, which go vet reports as unreachable code. Move it before the return,
# inside "if false", as proposed upstream in
# https://github.com/antlr/antlr4/pull/4445.
perl -0pi -e 's/^(\treturn localctx\n)(\tgoto errorExit [^\n]*\n)/\tif false {\n\t$2\t}\n$1/mg' \
  "$dest_dir/sqlite_parser.go"

echo "Verifying that generated files can build and pass go vet..."
go build -v $dest_dir
go vet $dest_dir
echo "Generated files are OK."
