#!/usr/bin/env bash


rm -vf ../grammar/*.go
rm -vf ../grammar/*.bak
rm -vf ../grammar/*.tokens
rm -vf ../libsq/slq/*
java -Xmx500M -cp "./ST-4.0.8.jar:./antlr4-4.5.4-SNAPSHOT.jar" org.antlr.v4.Tool -listener -visitor -package "slq" -Dlanguage=Go ../grammar/SLQ.g4

# for some reason (luser error?), antlr is not respecting the -o (output) arg above,
# so need to manually move the generated files to the correct location.
mv -vf ../grammar/*.tokens ../libsq/slq/
mv -vf ../grammar/*.go ../libsq/slq/
rm -vf ../grammar/*.bak