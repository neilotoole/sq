#!/usr/bin/env bash
# Build a GitHub Actions matrix-include array from a selection and scope.
#
# Usage: build-db-matrix.sh <full|narrow> <selection-json>
#   selection-json: {"postgres":["12","latest"],"mysql":["8"]}
# Emits a JSON array of {engine,tag,image,port,env,packages} on stdout.
#
# The `image` field is the fully-qualified pull ref, built once here so both the
# test job's service container and dedup-db-matrix.sh consume the same string —
# the registry is defined in exactly one place. Registry defaults to GHCR
# (ghcr.io/sakiladb), which isn't subject to Docker Hub's anonymous pull rate
# limits; override with SAKILADB_REGISTRY.
#
# Note: the DSN is deliberately NOT included. It contains credentials that
# GitHub masks as a secret, and a job output containing a masked value is
# dropped (not passed to downstream jobs) — which would silently empty the
# matrix. The test job looks up the DSN from .github/sakila-db.json at runtime.
set -euo pipefail

scope="${1:?usage: build-db-matrix.sh <full|narrow> <selection-json>}"
selection="${2:?missing selection json}"
config="$(cd "$(dirname "$0")/.." && pwd)/.github/sakila-db.json"
registry="${SAKILADB_REGISTRY:-ghcr.io/sakiladb}"

# workflow_dispatch constrains scope via type:choice, but the workflow_call
# input is a free string — reject typos rather than silently treating any
# non-"narrow" value as "full".
case "$scope" in
  full | narrow) ;;
  *)
    echo "build-db-matrix.sh: scope must be 'full' or 'narrow', got '$scope'" >&2
    exit 2
    ;;
esac

jq -cn \
  --argjson sel "$selection" \
  --slurpfile cfg "$config" \
  --arg scope "$scope" \
  --arg registry "$registry" '
  ($cfg[0]) as $c
  | [ $sel | to_entries[]
      | .key as $engine
      | ($c[$engine] // error("unknown engine: \($engine)")) as $e
      | .value[]
      | {
          engine:   $engine,
          tag:      .,
          image:    "\($registry)/\($engine):\(.)",
          port:     $e.port,
          env:      $e.env,
          packages: (if $scope == "narrow" then $e.packages else "./..." end)
        } ]
'
