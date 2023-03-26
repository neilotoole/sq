#!/usr/bin/env bash

# This script the per-release changelog file for goreleaser to use
# to publish to GitHub releases.
curTag=$(git tag --sort=-creatordate | head -n 1)
prevTag=$(git tag --sort=-creatordate | head -n 2 | tail -n 1)
git diff "$prevTag" "$curTag" --no-ext-diff --unified=0 --exit-code -a --no-prefix -- ./CHANGELOG.md \
| grep -E "^\+" | grep -v '+++ CHANGELOG.md' | cut  -c 2-

echo '### Commits'

echo '```text'
git log --pretty=format:'%h   %s%n' "$prevTag".."$curTag"
echo '```'
