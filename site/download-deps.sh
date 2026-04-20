#!/usr/bin/env bash
# Miscellaneous pre-build tasks.
set -e

BASE_URL=https://github.com/asciinema/asciinema-player/releases/download
curl -fsSL $BASE_URL/v3.0.1/asciinema-player.css -o ./static/css/asciinema-player.css

# Note that we save the minified version to ".js", not ".min.js".
curl -fsSL  $BASE_URL/v3.0.1/asciinema-player.min.js -o ./static/js/asciinema-player.js
