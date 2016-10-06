#!/usr/bin/env bash


rm -rvf ../grammar/*.go
rm -rvf ../grammar/*.tokens
java -Xmx500M -cp "./ST-4.0.8.jar:./antlr4-4.5.2-SNAPSHOT.jar" org.antlr.v4.Tool -listener -visitor -package parser -Dlanguage=Go ../grammar/SQ.g4

# for some reason (luser error?), antlr is not respecting the -o (output) arg above,
# so need to manually move the generated files to the correct location.
mv -vf ../grammar/*.tokens ../grammar/*.go ../lib/ql/parser/