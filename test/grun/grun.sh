#!/usr/bin/env bash



JAR_ANTLR="`pwd`/../../tools/antlr4-4.5.2-SNAPSHOT.jar"
JAR_ST="`pwd`/../../tools/ST-4.0.8.jar"

CP="$JAR_ANTLR:$JAR_ST"

ANTLR_CMD="java -Xmx500M  -cp $CP  org.antlr.v4.Tool"
GRUN_CMD="java -cp $CP:java/. org.antlr.v4.gui.TestRig SQ query -gui"

rm -rf java/*
$ANTLR_CMD -o "`pwd`/java" ../../grammar/SQ.g4
mv -f ../grammar/*.java ../grammar/*.tokens ./java/
rm -r ../grammar

javac java/*.java

$GRUN_CMD $1*.test.sq &
