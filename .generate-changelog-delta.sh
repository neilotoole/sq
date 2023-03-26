#!/usr/bin/env bash

# This script the per-release changelog file for goreleaser to use
# to publish to GitHub releases. It produces markdown output.

# First we get the current and previous git tags.
curTag=$(git tag --sort=-creatordate | head -n 1)
prevTag=$(git tag --sort=-creatordate | head -n 2 | tail -n 1)

# Then we run git diff on CHANGELOG.md;
# and grep for only the added lines;
# and stripping out the '+++ CHANGELOG.md' header line;
# and stripping the leading '+' from each line
# and getting rid of a superfluous starting newline.
git diff "$prevTag" "$curTag" --no-ext-diff --unified=0 --exit-code -a --no-prefix -- ./CHANGELOG.md \
| grep -E "^\+" \
| grep -vF '## [v' \
| grep -vF '+++ CHANGELOG.md' \
| cut -c 2- \
| tail -n +2



# Then we add a section for the commits.
printf "\n### Commits\n\n"

echo '```text'
git log --pretty=format:'%h   %s%n' "$prevTag".."$curTag"
echo '```'
