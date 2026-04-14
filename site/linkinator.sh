#!/usr/bin/env bash
# Use linkinator to verify links.
# For best results, we start a local web server (as a background process),
# and point linkinator at it.
#
# Note also: linkinator.config.json

set -e
# Kill the web server background process when the script exits
# https://stackoverflow.com/a/2173421

trap "exit" INT TERM
trap "kill -- -$$ &> /dev/null && exit 0" EXIT

port=31317
base_url="http://localhost:$port"

# Build a nice fresh site into $lint_dir
lint_dir="./.serve-lint"
rm -rf $lint_dir
./node_modules/.bin/hugo/hugo --gc --minify -b $base_url -d $lint_dir

# Start a local webserver (background process)
echo "Starting server for linting at: $base_url"
bunx serve -l $port $lint_dir &
#bunx serve -l $port $lint_dir > /dev/null &
sleep 2 # Give server time to start
echo "Server started"

bunx linkinator --config ./linkinator.config.json -r $base_url

echo "Linkinator finished"
