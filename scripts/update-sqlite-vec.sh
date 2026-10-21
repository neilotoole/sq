#!/usr/bin/env bash
#
# update-sqlite-vec.sh - refresh the vendored sqlite-vec sources.
#
# Usage: scripts/update-sqlite-vec.sh <commit>
#
# Downloads sqlite-vec.c, sqlite-vec.h and the license files from the
# vlasky/sqlite-vec fork at <commit> into drivers/sqlite3/internal/sqlitevec,
# copies mattn/go-sqlite3's sqlite3-binding.h (for the version in go.mod) to
# sqlite3.h there, and rewrites README.md with provenance and sha256 sums.

set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: $0 <commit>" >&2
  exit 2
fi

COMMIT="$1"
REPO="vlasky/sqlite-vec"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="$ROOT/drivers/sqlite3/internal/sqlitevec"

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

# Resolve to a full SHA so raw URLs work and the README records it.
FULL_SHA="$(gh api "repos/$REPO/commits/$COMMIT" --jq .sha)"

mkdir -p "$DEST"
for f in sqlite-vec.c sqlite-vec.h LICENSE-MIT LICENSE-APACHE; do
  curl -fsSL "https://raw.githubusercontent.com/$REPO/$FULL_SHA/$f" -o "$DEST/$f"
done

# Local patch: gcc (unlike clang) rejects the implicit signed/unsigned NEON
# vector conversions in the int8 L1 distance code, which breaks linux/arm64
# builds. Make them explicit with vreinterpretq; the generated code is identical.
sed -E 's/vaddq_s32\((acc[0-9]), vpaddlq_u16\(vpaddlq_u8\(diff([0-9]?)\)\)\)/vaddq_s32(\1, vreinterpretq_s32_u32(vpaddlq_u16(vpaddlq_u8(vreinterpretq_u8_s8(diff\2)))))/' \
  "$DEST/sqlite-vec.c" >"$DEST/sqlite-vec.c.patched"
mv "$DEST/sqlite-vec.c.patched" "$DEST/sqlite-vec.c"

MATTN_VER="$(cd "$ROOT" && go list -m -f '{{.Version}}' github.com/mattn/go-sqlite3)"
MATTN_DIR="$(go env GOMODCACHE)/github.com/mattn/go-sqlite3@$MATTN_VER"
cp "$MATTN_DIR/sqlite3-binding.h" "$DEST/sqlite3.h"
chmod 644 "$DEST/sqlite3.h"

VEC_VER="$(sed -n 's/^#define SQLITE_VEC_VERSION "\(.*\)"/\1/p' "$DEST/sqlite-vec.h")"
SQLITE_VER="$(sed -n 's/^#define SQLITE_VERSION  *"\(.*\)"/\1/p' "$DEST/sqlite3.h")"

{
  echo "# sqlitevec"
  echo
  echo "Vendored [sqlite-vec](https://github.com/$REPO) C source, compiled into sq via cgo and"
  echo "registered with SQLite using \`sqlite3_auto_extension\`. Do not edit these files by hand;"
  echo "run \`scripts/update-sqlite-vec.sh <commit>\` instead."
  echo
  echo "- Repo: https://github.com/$REPO"
  echo "- Version: $VEC_VER"
  echo "- Commit: $FULL_SHA"
  echo "- mattn/go-sqlite3: $MATTN_VER (SQLite $SQLITE_VER)"
  echo
  echo "\`sqlite-vec.c\` carries one local patch (applied by the script) in the NEON int8 L1 code:"
  echo "explicit \`vreinterpretq\` casts replace implicit signed/unsigned vector conversions that gcc"
  echo "rejects (clang accepts them). The sha256 below is for the patched file."
  echo
  echo "\`sqlite3.h\` is mattn's \`sqlite3-binding.h\`. When mattn is bumped, rerun the script so the"
  echo "header matches the SQLite that is linked."
  echo
  echo "| File | sha256 |"
  echo "| ---- | ------ |"
  for f in sqlite-vec.c sqlite-vec.h sqlite3.h LICENSE-MIT LICENSE-APACHE; do
    echo "| $f | $(sha256_of "$DEST/$f") |"
  done
} >"$DEST/README.md"

echo "Vendored sqlite-vec $VEC_VER ($FULL_SHA) with SQLite $SQLITE_VER header."
