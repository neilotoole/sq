#!/usr/bin/env bash

find ../lib -type f -name '*_test.go' | while read FILE ; do
    newfile="$(echo ${FILE} |sed -e 's/_test.go/_test_.go/')" ;
    mv -v "${FILE}" "${newfile}" ;
done