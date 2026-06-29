#!/usr/bin/env bash
# Build a GitHub Actions matrix-include array from a selection and scope.
#
# Usage: build-db-matrix.sh <full|narrow> <selection-json>
#   selection-json: {"postgres":["12","latest"],"mysql":["8"]}
# Emits a JSON array of {engine,tag,port,env,packages} on stdout.
#
# Note: the DSN is deliberately NOT included. It contains credentials that
# GitHub masks as a secret, and a job output containing a masked value is
# dropped (not passed to downstream jobs) — which would silently empty the
# matrix. The test job looks up the DSN from .github/sakila-db.json at runtime.
set -euo pipefail

scope="${1:?usage: build-db-matrix.sh <full|narrow> <selection-json>}"
selection="${2:?missing selection json}"
config="$(cd "$(dirname "$0")/.." && pwd)/.github/sakila-db.json"

jq -cn \
  --argjson sel "$selection" \
  --slurpfile cfg "$config" \
  --arg scope "$scope" '
  ($cfg[0]) as $c
  | [ $sel | to_entries[]
      | .key as $engine
      | ($c[$engine] // error("unknown engine: \($engine)")) as $e
      | .value[]
      | {
          engine:   $engine,
          tag:      .,
          port:     $e.port,
          env:      $e.env,
          packages: (if $scope == "narrow" then $e.packages else "./..." end)
        } ]
'
