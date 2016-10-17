#!/usr/bin/env bash


rm -vf ../grammar/*.go
rm -vf ../grammar/*.bak
rm -vf ../grammar/*.tokens
rm -vf ../libsq/slq/*
java -Xmx500M -cp "./ST-4.0.8.jar:./antlr4-4.5.2-SNAPSHOT.jar" org.antlr.v4.Tool -listener -visitor -package "slq" -Dlanguage=Go ../grammar/SLQ.g4

# Due to a bug in the antlr generator, the generated lexer file is not respecting the -package flag, so we need to do some magic
sed -i '.bak' 's/package parser/package slq/' ../grammar/slq_lexer.go

# for some reason (luser error?), antlr is not respecting the -o (output) arg above,
# so need to manually move the generated files to the correct location.
mv -vf ../grammar/*.tokens ../libsq/slq/
mv -vf ../grammar/*.go ../libsq/slq/
rm -vf ../grammar/*.bak