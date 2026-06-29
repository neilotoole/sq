#!/usr/bin/env bash
# Build a GitHub Actions matrix-include array from a selection and scope.
#
# Usage: build-db-matrix.sh <full|narrow> <selection-json>
#   selection-json: {"postgres":["12","latest"],"mysql":["8"]}
# Emits a JSON array of {engine,tag,port,env,dsn,packages} on stdout.
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
          dsn:      $e.dsn,
          packages: (if $scope == "narrow" then $e.packages else "./..." end)
        } ]
'
