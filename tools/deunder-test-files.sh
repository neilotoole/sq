#!/usr/bin/env bash

find ../lib -type f -name '*_test_.go' | while read FILE ; do
    newfile="$(echo ${FILE} |sed -e 's/_test_.go/_test.go/')" ;
    mv -v "${FILE}" "${newfile}" ;
done