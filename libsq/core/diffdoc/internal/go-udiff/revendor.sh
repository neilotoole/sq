#!/usr/bin/env bash
#
# revendor.sh re-imports the go-udiff package from upstream
# github.com/aymanbagabas/go-udiff, rewriting import paths to sq's
# internal vendor path, then applying the sq-local patches under
# sq-patches/. It rewrites its own directory in place, preserving only
# this script, the UPSTREAM marker, and the sq-patches/ directory.
#
# Usage:
#   ./revendor.sh [git-ref]
# The ref defaults to the one recorded in ./UPSTREAM, else v0.4.1.
set -euo pipefail

UPSTREAM_REPO="https://github.com/aymanbagabas/go-udiff"
OLD_PATH="github.com/aymanbagabas/go-udiff"
NEW_PATH="github.com/neilotoole/sq/libsq/core/diffdoc/internal/go-udiff"

VENDOR_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

REF="${1:-}"
if [[ -z "$REF" && -f "$VENDOR_DIR/UPSTREAM" ]]; then
  REF="$(awk 'NR==1{print $2}' "$VENDOR_DIR/UPSTREAM")"
fi
REF="${REF:-v0.4.1}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

git clone --quiet --depth 1 --branch "$REF" "$UPSTREAM_REPO" "$tmp/src"
commit="$(git -C "$tmp/src" rev-parse HEAD)"

# Stage the curated tree, dropping upstream packaging sq does not vendor.
stage="$tmp/stage"
mkdir -p "$stage"
cp -R "$tmp/src/." "$stage/"
rm -rf "$stage/.git" "$stage/.github" "$stage/scripts" "$stage/Makefile" \
       "$stage/go.mod" "$stage/go.sum" "$stage/_examples"

# Rewrite import paths (perl -i is portable across macOS/Linux).
find "$stage" -type f -name '*.go' -print0 \
  | xargs -0 perl -i -pe "s{\\Q$OLD_PATH\\E}{$NEW_PATH}g"

# Apply sq-local patches on top of the import-rewritten tree. These live
# under sq-patches/ (preserved across syncs). set -e aborts if a patch no
# longer applies, signaling it must be regenerated against the new upstream.
shopt -s nullglob
for p in "$VENDOR_DIR"/sq-patches/*.patch; do
  echo "Applying sq-patch $(basename "$p")"
  ( cd "$stage" && git apply -p1 "$p" )
done
shopt -u nullglob

# Replace vendored content, preserving this script, the marker, and sq-patches/.
find "$VENDOR_DIR" -mindepth 1 -maxdepth 1 \
     ! -name 'revendor.sh' ! -name 'UPSTREAM' ! -name 'sq-patches' -exec rm -rf {} +
cp -R "$stage/." "$VENDOR_DIR/"

# Record provenance.
printf '%s %s\n%s\n' "$UPSTREAM_REPO" "$REF" "$commit" > "$VENDOR_DIR/UPSTREAM"

echo "Re-vendored $UPSTREAM_REPO@$REF ($commit)"
echo "Next: run 'make all' and inspect diff golden-test deltas before committing."
