#!/usr/bin/env bash
#
# fmt-go-imports.sh - normalize Go import grouping and aliasing.
#
# goimports-reviser does two distinct jobs here, with very different costs:
#
#   1. Grouping/ordering imports into standard / general / company
#      (github.com/neilotoole) sections. This is purely syntactic and fast, so
#      it runs over the whole module (./...) on every invocation (~1s).
#
#   2. Setting aliases for versioned import paths via -set-alias, e.g.
#      `excelize "github.com/xuri/excelize/v2"`. To decide whether a /vN path
#      needs an explicit alias, goimports-reviser must resolve each imported
#      package's real name, which forces a full type-load of the ENTIRE module
#      dependency graph (including the CGO SQLite/DuckDB drivers). Over ./...
#      that costs ~145s; scoped to a handful of files it is ~1s. So the alias
#      pass is run only over the Go files that actually changed (branch vs base,
#      plus working-tree and untracked files). Use --all to alias the whole
#      module (rarely needed: the tree is already fully aliased, and a new alias
#      can only be introduced in a file you are editing).
#
# -rm-unused is deliberately NOT used: an unused import is a compile error,
# already caught by `go build`, `go test`, and the `unused` linter, and the flag
# triggers the same expensive whole-graph type-load as -set-alias for no gain.
#
# Flags:
#   --all          Run the -set-alias pass over ./... (full module). Slow.
#   --base <ref>   Diff against <ref> to find changed files, instead of the
#                  default merge-base with origin/master. Used by CI.
#
# termz_windows.go is excluded because goimports-reviser mangles code guarded by
# build tags that aren't currently in use. filepath.Match has no double-star, so
# the file is named explicitly.

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

mode="incremental"
base_ref=""
while [ $# -gt 0 ]; do
  case "$1" in
    --all)
      mode="all"
      ;;
    --base)
      shift
      base_ref="${1:-}"
      if [ -z "$base_ref" ]; then
        echo "fmt-go-imports: --base requires a ref argument" >&2
        exit 2
      fi
      ;;
    *)
      echo "fmt-go-imports: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

reviser=(go tool -modfile=tools/goimports-reviser/go.mod goimports-reviser
  -company-prefixes github.com/neilotoole
  -project-name github.com/neilotoole/sq
  -excludes 'libsq/core/termz/termz_windows.go'
  -output write)

# 1. Fast grouping/ordering across the whole module (no -set-alias, so no
#    whole-graph type-load).
"${reviser[@]}" ./...

# 2. Alias pass.
if [ "$mode" = "all" ]; then
  "${reviser[@]}" -set-alias ./...
  exit 0
fi

# Incremental: alias only the Go files that changed.
if [ -z "$base_ref" ]; then
  base_ref="$(git merge-base HEAD origin/master 2>/dev/null \
    || git merge-base HEAD master 2>/dev/null \
    || true)"
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

{
  if [ -n "$base_ref" ]; then
    git diff --name-only --diff-filter=ACMR "$base_ref...HEAD" || true
  fi
  # Working-tree changes (staged and unstaged) vs HEAD.
  git diff --name-only --diff-filter=ACMR HEAD || true
  # New files not yet tracked by git.
  git ls-files --others --exclude-standard || true
} 2>/dev/null | sort -u >"$tmp"

files=()
while IFS= read -r f; do
  [ -n "$f" ] || continue
  case "$f" in
    *.go) ;;
    *) continue ;;
  esac
  [ -f "$f" ] || continue
  if [ "$f" = "libsq/core/termz/termz_windows.go" ]; then
    continue
  fi
  files+=("$f")
done <"$tmp"

if [ "${#files[@]}" -eq 0 ]; then
  exit 0
fi

"${reviser[@]}" -set-alias "${files[@]}"
