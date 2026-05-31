#!/usr/bin/env bash
# Print a per-release x per-package-type download breakdown for a GitHub repo.
# Usage: release-stats.sh [owner/repo]   (default: neilotoole/sq)
#
# Uses `gh api`, which requires a logged-in `gh` session (`gh auth login`).
# Token-free fallback for public repos (60 req/hr per IP, first page only):
#
#   curl -sS "https://api.github.com/repos/${repo}/releases?per_page=100" \
#     | jq -rs '...as below, but the input is already a single page array...'
#
# For full pagination without `gh`, walk the response `Link:` header.

set -euo pipefail

repo="${1:-neilotoole/sq}"

gh api --paginate "repos/${repo}/releases?per_page=100" \
  | jq -rs '
    [ .[] | .[] | {tag: .tag_name, assets} | . as $r | $r.assets[] |
      { tag: $r.tag, dl: .download_count,
        pkg: ( .name |
          if   test("\\.deb$")              then "deb"
          elif test("\\.rpm$")              then "rpm"
          elif test("\\.apk$")              then "apk"
          elif test("\\.pkg\\.tar\\.zst$")  then "pacman"
          elif test("windows-.*\\.zip$")    then "win"
          elif test("macos-.*\\.tar\\.gz$") then "mac"
          elif test("linux-.*\\.tar\\.gz$") then "linux"
          elif . == "checksums.txt"         then "chksum"
          else "other" end ) } ]
    | group_by(.tag)
    | map( (group_by(.pkg) | map({(.[0].pkg): (map(.dl)|add)}) | add) + {tag: .[0].tag} )
    | sort_by(.tag) | reverse
    | (["TAG","mac","linux","win","deb","rpm","apk","pacman","other","chksum","TOTAL"]),
      (.[] | [.tag,
        (.mac//0), (.linux//0), (.win//0), (.deb//0), (.rpm//0), (.apk//0), (.pacman//0), (.other//0), (.chksum//0),
        ([.mac,.linux,.win,.deb,.rpm,.apk,.pacman,.other,.chksum] | map(.//0) | add) ])
    | @tsv' \
  | column -t -s $'\t'
