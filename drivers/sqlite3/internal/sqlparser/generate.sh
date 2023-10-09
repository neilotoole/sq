#!/bin/sh

set -e

dest_dir="./sqlite"
mkdir -p $dest_dir

echo "Generating parser code from grammar..."
alias antlr4='java -Xmx500M -cp "../../../../tools/antlr-4.13.0-complete.jar:$CLASSPATH" org.antlr.v4.Tool'
antlr4 -Dlanguage=Go -listener -visitor -o $dest_dir -package sqlite SQLiteLexer.g4 SQLiteParser.g4

echo "Verifying that generated files can build..."
go build -v $dest_dir
echo "Generated files are OK."
