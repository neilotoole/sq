#!/bin/sh
# ".completions.sh regenerates completions to "./completions".

set +e
brew uninstall neilotoole/sq/sq

if [[ $(which sq) ]]; then
    echo "sq is still present"
    rm $(which sq)
fi

set -e
brew install neilotoole/sq/sq

# TODO: Test the version
