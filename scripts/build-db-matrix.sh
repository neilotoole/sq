#!/usr/bin/env bash
# Build a GitHub Actions matrix-include array from a selection.
#
# Usage: build-db-matrix.sh <selection-json>
#   selection-json: {"postgres":["12","latest"],"mysql":["bookends"],"*":["oldest"]}
# Emits a JSON array of {engine,tag,image,port,env} on stdout.
#
# Selection grammar. Each engine key maps to a list of selectors:
#   <tag>      an explicit numeric tag such as 18, 5.7 or 2019. Passes through
#              even if absent from the engine's tags list, so a new image can
#              be tried before it is added to .github/sakila-db.json.
#   latest     the floating :latest image
#   oldest     the last entry in the engine's tags (tags are newest-first)
#   bookends   latest + oldest
#   all        every entry in the engine's tags
# Any other word is an error. The engine key "*" expands to every engine in
# sakila-db.json; explicit engine keys alongside "*" append to that engine.
#
# Keywords resolve to tag strings here, and exact-string duplicates are dropped
# (first occurrence wins). Same-image duplicates such as latest and 18 are
# dedup-db-matrix.sh's job, which is what lets "bookends" on a single-tag engine
# collapse to one leg downstream.
#
# The `image` field is the fully-qualified pull ref, built once here so both the
# test job's service container and dedup-db-matrix.sh consume the same string;
# the registry is defined in exactly one place. Registry defaults to GHCR
# (ghcr.io/sakiladb), which isn't subject to Docker Hub's anonymous pull rate
# limits; override with SAKILADB_REGISTRY.
#
# Note: the DSN is deliberately NOT included. It contains credentials that
# GitHub masks as a secret, and a job output containing a masked value is
# dropped (not passed to downstream jobs), which would silently empty the
# matrix. The test job looks up the DSN from .github/sakila-db.json at runtime.
set -euo pipefail

selection="${1:?usage: build-db-matrix.sh <selection-json>}"
config="$(cd "$(dirname "$0")/.." && pwd)/.github/sakila-db.json"
registry="${SAKILADB_REGISTRY:-ghcr.io/sakiladb}"
registry="${registry%/}" # tolerate a trailing slash in the override

jq -cn \
  --argjson sel "$selection" \
  --slurpfile cfg "$config" \
  --arg registry "$registry" '
  ($cfg[0]) as $c
  # Expand "*" to every engine, then append explicit engine entries, so that
  # {"*":["latest"],"postgres":["9"]} yields postgres: ["latest","9"].
  | ( (if ($sel | has("*")) then [ $c | keys[] | {key: ., value: $sel["*"]} ] else [] end)
      + [ $sel | to_entries[] | select(.key != "*") ] )
  | group_by(.key)
  | map({key: .[0].key, value: (map(.value) | add)})
  | [ .[]
      | .key as $engine
      | ($c[$engine] // error("unknown engine: \($engine)")) as $e
      | [ .value[]
          | if . == "latest" then "latest"
            elif . == "oldest" then $e.tags[-1]
            elif . == "bookends" then ("latest", $e.tags[-1])
            elif . == "all" then $e.tags[]
            elif test("^[0-9]+(\\.[0-9]+)*$") then .
            else error("unknown selector \"\(.)\" for engine \($engine)")
            end ]
      # Drop exact-string duplicates; first occurrence wins.
      | reduce .[] as $t ([]; if any(.[]; . == $t) then . else . + [$t] end)
      | .[]
      | {
          engine: $engine,
          tag:    .,
          image:  "\($registry)/\($engine):\(.)",
          port:   $e.port,
          env:    $e.env
        } ]
'
