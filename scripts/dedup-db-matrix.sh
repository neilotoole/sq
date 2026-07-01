#!/usr/bin/env bash
# Collapse matrix-include entries that resolve to the same container image, so
# that e.g. specifying both "latest" and "18" for postgres (which are the same
# published image) runs it only once instead of twice.
#
# Usage: build-db-matrix.sh <scope> <selection> | dedup-db-matrix.sh
#   stdin:  a JSON array of {engine,tag,image,port,env,packages} (build-db-matrix.sh).
#   stdout: the same array with duplicates removed.
#
# Dedup happens in two passes:
#   1. Exact (engine,tag) duplicates — pure text, always applied (e.g. "18,18").
#   2. Same image digest — e.g. postgres:latest == postgres:18. The digest is
#      resolved by inspecting the entry's `image` ref with `docker buildx
#      imagetools inspect` (manifest only, no layer pull). This is best-effort:
#      if resolution fails (offline, unknown tag), the entry is KEPT rather than
#      dropped, so a registry hiccup can never silently remove test coverage —
#      the worst case is the original redundant run.
#
# The `image` ref (registry included) is chosen once by build-db-matrix.sh; this
# script never reconstructs it. Because that registry is GHCR by default, digest
# resolution isn't subject to Docker Hub's anonymous pull rate limits. A tag
# missing on the registry (e.g. sqlserver:2017 on GHCR) simply fails open.
#
# When two tags collapse, the earlier-listed entry wins (input order preserved),
# so callers control which tag label survives by ordering their tags.
#
# Diagnostics go to stderr because stdout carries the JSON result.
set -euo pipefail

inc="$(cat)"

# Pass 1: drop exact (engine,tag) duplicates, preserving first occurrence.
inc="$(jq -c '
  reduce .[] as $x ([];
    if any(.[]; .engine == $x.engine and .tag == $x.tag) then . else . + [$x] end)
' <<<"$inc")"

# Pass 2: resolve digests and drop later entries sharing an (engine,digest).
seen=$'\n'   # newline-delimited "engine<TAB>digest" keys already emitted
out='[]'
while read -r entry; do
  [ -n "$entry" ] || continue
  engine="$(jq -r '.engine' <<<"$entry")"
  image="$(jq -r '.image' <<<"$entry")"
  digest="$(docker buildx imagetools inspect "$image" \
    --format '{{.Manifest.Digest}}' 2>/dev/null || true)"
  if [ -n "$digest" ]; then
    key="${engine}"$'\t'"${digest}"
    if [[ "$seen" == *$'\n'"$key"$'\n'* ]]; then
      echo "dedup: ${image} (${digest}) already scheduled; skipping" >&2
      continue
    fi
    seen="${seen}${key}"$'\n'
  fi
  out="$(jq -c --argjson e "$entry" '. + [$e]' <<<"$out")"
done < <(jq -c '.[]' <<<"$inc")

printf '%s\n' "$out"
