#!/usr/bin/env bash
#
# bench-format-lint.sh - benchmark sq's formatting/linting steps, re-runnably.
#
# Times each formatting/linting step we care about during the dprint overhaul and
# writes per-run results plus a cumulative history under the gitignored scratch/
# dir so successive runs can be compared to track improvements.
#
# Before timing, a tool preflight (modeled on the glide `make check` pattern)
# verifies the tools required by the in-scope steps: presence + version + an
# install hint, exiting non-zero on any miss. A normal run ABORTS if the preflight
# fails (use --allow-missing to run a partial benchmark anyway). The same gate is
# reusable for CI/automation via `--check`.
#
# "In-scope" is phase-aware: a step whose required config file is absent (e.g.
# dprint-check needs dprint.json) is not yet part of this phase, so its tool is
# not required yet. Steps with no config gate (the *-old steps) are always in
# scope. This lets the SAME script work at baseline and after each migration phase.
#
# Usage:
#   scripts/bench-format-lint.sh [options]
#
# Options:
#   --check          Run only the tool preflight, then exit (CI/automation gate).
#   --allow-missing  Don't abort when preflight finds missing tools; skip them.
#   --runs N         Timed runs per step (default 10).
#   --warmup N       Warmup runs per step, not timed (default 3).
#   --only a,b,c     Only run the named steps (comma-separated).
#   --list           List registered steps and whether each would run, then exit.
#   --no-color       Disable colored output.
#   -h, --help       Show this help.
#
# Engine: uses hyperfine when installed (stable stats); otherwise falls back to a
# portable timing loop (python3 high-res clock + awk stats). Install hyperfine
# for best results:  brew install hyperfine
#
# Output (under scratch/bench-format-lint/, all gitignored):
#   <UTC-timestamp>-<git-sha>/env.txt       environment + tool versions
#   <UTC-timestamp>-<git-sha>/results.md    human-readable table
#   <UTC-timestamp>-<git-sha>/results.json  machine-readable
#   history.ndjson                          one JSON line per (run, step), appended

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults / globals.
# ---------------------------------------------------------------------------
RUNS=10
WARMUP=3
ONLY=""
DO_LIST=""
DO_CHECK=""
ALLOW_MISSING=""
USE_COLOR=1

# ---------------------------------------------------------------------------
# Step registry.
#
# One entry per line: name|requires_bins|requires_files|mutating|command
#   requires_bins  : comma-separated binaries that must be on PATH (else SKIP)
#   requires_files : comma-separated repo-relative files that must exist (else SKIP)
#   mutating       : yes|no (informational; mutating steps run on a clean tree so
#                    the timed pass is an idempotent no-op write)
#   command        : shell command, run from repo root inside a subshell
#
# Add new steps here; they auto-activate once their tool/config exists.
# ---------------------------------------------------------------------------
STEPS="
go-fmt-old|go||yes|go tool -modfile=tools/goimports-reviser/go.mod goimports-reviser -company-prefixes github.com/neilotoole -set-alias -excludes 'libsq/core/termz/termz_windows.go' -rm-unused -output write -project-name github.com/neilotoole/sq ./... && go tool -modfile=tools/gofumpt/go.mod gofumpt -w .
go-lint|go||no|go tool -modfile=tools/golangci-lint/go.mod golangci-lint run
markdown-lint-old|bun||no|bun run lint:markdown
site-js-lint-old|bun||no|cd site && bun run lint:scripts
site-styles-lint-old|bun||no|cd site && bun run lint:styles
site-md-lint-old|bun||no|cd site && bun run lint:markdown
shellcheck|shellcheck||no|shellcheck ./install.sh
go-imports-new|go|scripts/fmt-go-imports.sh|yes|bash scripts/fmt-go-imports.sh
dprint-fmt|bun|dprint.json|yes|bunx dprint fmt
dprint-check|bun|dprint.json|no|bunx dprint check
biome-lint|bun|biome.json|no|bunx biome lint
"

# ---------------------------------------------------------------------------
# Helpers.
# ---------------------------------------------------------------------------
c_reset=""; c_dim=""; c_red=""; c_grn=""; c_yel=""; c_bld=""
setup_colors() {
  if [ "$USE_COLOR" = "1" ] && [ -t 1 ]; then
    c_reset=$'\033[0m'; c_dim=$'\033[2m'; c_red=$'\033[31m'
    c_grn=$'\033[32m'; c_yel=$'\033[33m'; c_bld=$'\033[1m'
  fi
}

err() { printf '%s\n' "bench-format-lint: $*" >&2; }
die() { err "$*"; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

usage() {
  sed -n '2,46p' "$0" | sed 's/^# \{0,1\}//'
}

# now: high-resolution seconds as a float.
now() {
  python3 -c 'import time; print(time.perf_counter())'
}

# tool_ver <bin>: best-effort one-line version string, or "n/a".
tool_ver() {
  local bin="$1" v=""
  have "$bin" || { printf 'MISSING'; return; }
  case "$bin" in
    go)         v=$(go version 2>/dev/null) ;;
    dprint)     v=$(dprint --version 2>/dev/null) ;;
    biome)      v=$(biome --version 2>/dev/null) ;;
    bun)        v=$(bun --version 2>/dev/null) ;;
    shellcheck) v=$(shellcheck --version 2>/dev/null | awk '/version:/{print $2; exit}') ;;
    hyperfine)  v=$(hyperfine --version 2>/dev/null) ;;
    jq)         v=$(jq --version 2>/dev/null) ;;
    python3)    v=$(python3 --version 2>&1) ;;
    *)          v=$("$bin" --version 2>/dev/null | head -1) ;;
  esac
  [ -n "$v" ] && printf '%s' "$v" || printf 'present'
}

# ---------------------------------------------------------------------------
# Arg parsing.
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --runs)     RUNS="${2:-}"; shift 2 ;;
    --warmup)   WARMUP="${2:-}"; shift 2 ;;
    --only)     ONLY="${2:-}"; shift 2 ;;
    --list)     DO_LIST=1; shift ;;
    --check)    DO_CHECK=1; shift ;;
    --allow-missing) ALLOW_MISSING=1; shift ;;
    --no-color) USE_COLOR=0; shift ;;
    -h|--help)  usage; exit 0 ;;
    *)          die "unknown option: $1 (try --help)" ;;
  esac
done

case "$RUNS" in (*[!0-9]*|"") die "--runs must be a positive integer" ;; esac
case "$WARMUP" in (*[!0-9]*) die "--warmup must be a non-negative integer" ;; esac

setup_colors

# ---------------------------------------------------------------------------
# Locate repo root and required base tools.
# ---------------------------------------------------------------------------
have git || die "git is required"
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || die "not in a git repo"
cd "$REPO_ROOT"
have python3 || die "python3 is required for timing"
have jq || die "jq is required for JSON output"

GIT_SHA_SHORT=$(git rev-parse --short HEAD)
GIT_SHA_FULL=$(git rev-parse HEAD)
TS=$(date -u +%Y%m%dT%H%M%SZ)
TS_ISO=$(date -u +%Y-%m-%dT%H:%M:%SZ)

USE_HYPERFINE=""
have hyperfine && USE_HYPERFINE=1

# in_only <name>: true if --only unset or name is listed.
in_only() {
  [ -z "$ONLY" ] && return 0
  case ",$ONLY," in (*",$1,"*) return 0 ;; esac
  return 1
}

# ---------------------------------------------------------------------------
# Tool preflight (glide `make check` style: presence + version + install hint).
# ---------------------------------------------------------------------------
# install_hint <bin>: suggested install command for a missing tool.
install_hint() {
  case "$1" in
    go)         echo "brew install go  (or https://go.dev/dl)" ;;
    bun)        echo "brew install oven-sh/bun/bun  (or curl -fsSL https://bun.sh/install | bash)" ;;
    shellcheck) echo "brew install shellcheck" ;;
    biome)      echo "brew install biome  (or bun add -g @biomejs/biome)" ;;
    dprint)     echo "brew install dprint  (or curl -fsSL https://dprint.dev/install.sh | sh)" ;;
    hyperfine)  echo "brew install hyperfine" ;;
    jq)         echo "brew install jq" ;;
    python3)    echo "brew install python" ;;
    git)        echo "brew install git" ;;
    *)          echo "see the tool's documentation" ;;
  esac
}

# min_version <bin>: minimum version to enforce, or empty for presence-only.
min_version() {
  case "$1" in
    bun) echo "1.2" ;;  # site/package.json engines: bun >=1.2
    *)   echo "" ;;
  esac
}

# tool_semver <bin>: best-effort dotted version number for gating.
tool_semver() {
  case "$1" in
    bun)       bun --version 2>/dev/null ;;
    dprint)    dprint --version 2>/dev/null | awk '{print $2}' ;;
    hyperfine) hyperfine --version 2>/dev/null | awk '{print $2}' ;;
    *)         "$1" --version 2>/dev/null | head -1 | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1 ;;
  esac
}

# ver_lt <a> <b>: true (exit 0) if version a < version b (via sort -V).
ver_lt() {
  [ "$1" = "$2" ] && return 1
  [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$1" ]
}

# required_bins: unique binaries required by in-scope steps (respecting --only).
# A step is in-scope iff all its required config files exist (phase-aware gate).
required_bins() {
  printf '%s\n' "$STEPS" | sed '/^$/d' | while IFS='|' read -r name bins files mut cmd; do
    in_only "$name" || continue
    local inscope=1 b f old_ifs
    old_ifs="$IFS"; IFS=','
    for f in $files; do [ -z "$f" ] && continue; [ -e "$f" ] || { inscope=0; break; }; done
    if [ "$inscope" = "1" ]; then
      for b in $bins; do [ -z "$b" ] && continue; echo "$b"; done
    fi
    IFS="$old_ifs"
  done | sort -u
}

# preflight: verify base tools + in-scope step tools. Returns non-zero if any
# required tool is missing or too old. hyperfine is recommended (warn only).
preflight() {
  local missing=0 t v minv cur all="" req
  printf '%sTool preflight%s%s\n\n' "$c_bld" "$c_reset" \
    "$([ -n "$ONLY" ] && echo " (--only $ONLY)" || echo "")"
  printf '%-12s %-9s %s\n' "TOOL" "STATUS" "VERSION / INSTALL HINT"

  req=$(required_bins)
  for t in git jq python3 $req; do
    case " $all " in (*" $t "*) continue ;; esac
    all="$all $t"
  done

  for t in $all; do
    if have "$t"; then
      v=$(tool_ver "$t")
      minv=$(min_version "$t")
      if [ -n "$minv" ]; then
        cur=$(tool_semver "$t")
        if [ -n "$cur" ] && ver_lt "$cur" "$minv"; then
          printf '%-12s %s%-9s%s %s (need >= %s) -> %s\n' \
            "$t" "$c_red" "TOO OLD" "$c_reset" "$cur" "$minv" "$(install_hint "$t")"
          missing=$((missing+1)); continue
        fi
      fi
      printf '%-12s %s%-9s%s %s\n' "$t" "$c_grn" "OK" "$c_reset" "$v"
    else
      printf '%-12s %s%-9s%s %s\n' "$t" "$c_red" "MISSING" "$c_reset" "$(install_hint "$t")"
      missing=$((missing+1))
    fi
  done

  if have hyperfine; then
    printf '%-12s %s%-9s%s %s\n' "hyperfine" "$c_grn" "OK" "$c_reset" "$(tool_ver hyperfine)"
  else
    printf '%-12s %s%-9s%s not installed (recommended for stable stats) -> %s\n' \
      "hyperfine" "$c_yel" "WARN" "$c_reset" "$(install_hint hyperfine)"
  fi

  printf '\n'
  if [ "$missing" -gt 0 ]; then
    printf '%s%d required tool(s) missing or too old.%s\n' "$c_red" "$missing" "$c_reset" >&2
    return 1
  fi
  printf '%sAll required tools present.%s\n' "$c_grn" "$c_reset"
  return 0
}

# ---------------------------------------------------------------------------
# --list mode.
# ---------------------------------------------------------------------------
if [ -n "$DO_LIST" ]; then
  printf '%sRegistered steps%s (repo: %s)\n\n' "$c_bld" "$c_reset" "$REPO_ROOT"
  printf '%-22s %-10s %-9s %s\n' "STEP" "MUTATING" "ACTIVE" "REASON"
  printf '%s\n' "$STEPS" | while IFS='|' read -r name bins files mut cmd; do
    [ -z "$name" ] && continue
    active="yes"; reason="-"
    old_ifs="$IFS"; IFS=','
    for b in $bins; do [ -z "$b" ] && continue; have "$b" || { active="no"; reason="missing bin: $b"; break; }; done
    if [ "$active" = "yes" ]; then
      for f in $files; do [ -z "$f" ] && continue; [ -e "$f" ] || { active="no"; reason="missing file: $f"; break; }; done
    fi
    IFS="$old_ifs"
    printf '%-22s %-10s %-9s %s\n' "$name" "$mut" "$active" "$reason"
  done
  exit 0
fi

# ---------------------------------------------------------------------------
# Tool preflight gate.
# ---------------------------------------------------------------------------
if [ -n "$DO_CHECK" ]; then
  preflight
  exit $?
fi

if ! preflight; then
  if [ -n "$ALLOW_MISSING" ]; then
    err "continuing despite missing tools (--allow-missing); their steps will be skipped"
  else
    die "required tools missing; install them or re-run with --allow-missing for a partial benchmark"
  fi
fi
printf '\n'

# ---------------------------------------------------------------------------
# Output locations.
# ---------------------------------------------------------------------------
BASE_DIR="scratch/bench-format-lint"
OUT_DIR="$BASE_DIR/${TS}-${GIT_SHA_SHORT}"
HISTORY="$BASE_DIR/history.ndjson"
mkdir -p "$OUT_DIR"

# env.txt
NCPU=$( (command -v nproc >/dev/null 2>&1 && nproc) || sysctl -n hw.ncpu 2>/dev/null || echo "?")
CPU=$(sysctl -n machdep.cpu.brand_string 2>/dev/null || uname -p)
{
  printf 'timestamp   %s\n' "$TS_ISO"
  printf 'git_sha     %s\n' "$GIT_SHA_FULL"
  printf 'uname       %s\n' "$(uname -a)"
  printf 'cpu         %s\n' "$CPU"
  printf 'ncpu        %s\n' "$NCPU"
  printf 'engine      %s\n' "$([ -n "$USE_HYPERFINE" ] && echo hyperfine || echo 'bash+python3')"
  printf 'runs        %s\n' "$RUNS"
  printf 'warmup      %s\n' "$WARMUP"
  printf '\ntool versions:\n'
  for t in go dprint biome bun shellcheck hyperfine jq python3; do
    printf '  %-12s %s\n' "$t" "$(tool_ver "$t")"
  done
} > "$OUT_DIR/env.txt"

# ---------------------------------------------------------------------------
# Timing.
# ---------------------------------------------------------------------------
# measure_bash <cmd> -> echoes "mean sd min max ok" (seconds; ok=1 if all runs exit 0).
measure_bash() {
  local cmd="$1" i start end dur ok=1
  local times=""
  i=0
  while [ "$i" -lt "$WARMUP" ]; do ( eval "$cmd" ) >/dev/null 2>&1 || true; i=$((i+1)); done
  i=0
  while [ "$i" -lt "$RUNS" ]; do
    start=$(now)
    if ( eval "$cmd" ) >/dev/null 2>&1; then :; else ok=0; fi
    end=$(now)
    dur=$(python3 -c "print($end-$start)")
    times="$times$dur
"
    i=$((i+1))
  done
  printf '%s' "$times" | awk -v ok="$ok" '
    NF{a[++n]=$1; s+=$1}
    END{
      if(n==0){print "0 0 0 0 " ok; exit}
      mean=s/n; min=a[1]; max=a[1];
      for(i=1;i<=n;i++){d=a[i]-mean; ss+=d*d; if(a[i]<min)min=a[i]; if(a[i]>max)max=a[i]}
      sd=(n>1)?sqrt(ss/(n-1)):0;
      printf "%.4f %.4f %.4f %.4f %s", mean, sd, min, max, ok
    }'
}

# measure_hyperfine <name> <cmd> -> echoes "mean sd min max ok"
measure_hyperfine() {
  local name="$1" cmd="$2" tmp
  tmp=$(mktemp)
  if hyperfine --warmup "$WARMUP" --runs "$RUNS" --ignore-failure \
       --command-name "$name" --export-json "$tmp" "$cmd" >/dev/null 2>&1; then
    jq -r '.results[0] | "\(.mean) \(.stddev // 0) \(.min) \(.max) 1"' "$tmp"
  else
    printf '0 0 0 0 0'
  fi
  rm -f "$tmp"
}

# ---------------------------------------------------------------------------
# Run steps.
# ---------------------------------------------------------------------------
printf '%sbench-format-lint%s  repo=%s sha=%s engine=%s runs=%s warmup=%s\n\n' \
  "$c_bld" "$c_reset" "$REPO_ROOT" "$GIT_SHA_SHORT" \
  "$([ -n "$USE_HYPERFINE" ] && echo hyperfine || echo bash)" "$RUNS" "$WARMUP"

# Collect result rows as ndjson in a temp file (bash 3.2: no nested data structs).
ROWS_TMP=$(mktemp)
trap 'rm -f "$ROWS_TMP"' EXIT

# emit_row: append one compact JSON object to ROWS_TMP and history.
emit_row() {
  local name="$1" status="$2" mean="$3" sd="$4" min="$5" max="$6"
  local meanj sdj minj maxj
  if [ "$status" = "ok" ] || [ "$status" = "failed" ]; then
    meanj="$mean"; sdj="$sd"; minj="$min"; maxj="$max"
  else
    meanj="null"; sdj="null"; minj="null"; maxj="null"
  fi
  jq -nc \
    --arg ts "$TS_ISO" --arg sha "$GIT_SHA_SHORT" --arg name "$name" \
    --arg status "$status" --argjson runs "$RUNS" \
    --argjson mean "$meanj" --argjson sd "$sdj" \
    --argjson min "$minj" --argjson max "$maxj" \
    '{ts:$ts, sha:$sha, step:$name, status:$status, runs:$runs,
      mean_s:$mean, stddev_s:$sd, min_s:$min, max_s:$max}' \
    | tee -a "$ROWS_TMP" >> "$HISTORY"
}

fmt_secs() { [ "$1" = "null" ] && printf '       -' || printf '%8.3f' "$1"; }

printf '%-22s %-9s %10s %10s %10s\n' "STEP" "STATUS" "MEAN(s)" "MIN(s)" "MAX(s)"
printf '%s\n' "$STEPS" | sed '/^$/d' | while IFS='|' read -r name bins files mut cmd; do
  in_only "$name" || continue

  # Gating: required bins + files.
  skip_reason=""
  old_ifs="$IFS"; IFS=','
  for b in $bins; do [ -z "$b" ] && continue; have "$b" || { skip_reason="missing bin: $b"; break; }; done
  if [ -z "$skip_reason" ]; then
    for f in $files; do [ -z "$f" ] && continue; [ -e "$f" ] || { skip_reason="missing file: $f"; break; }; done
  fi
  IFS="$old_ifs"

  if [ -n "$skip_reason" ]; then
    printf '%-22s %s%-9s%s %s%s%s\n' "$name" "$c_yel" "SKIPPED" "$c_reset" "$c_dim" "$skip_reason" "$c_reset"
    emit_row "$name" "skipped" null null null null
    continue
  fi

  if [ -n "$USE_HYPERFINE" ]; then
    read -r mean sd min max ok <<<"$(measure_hyperfine "$name" "$cmd")"
  else
    read -r mean sd min max ok <<<"$(measure_bash "$cmd")"
  fi

  if [ "$ok" = "1" ]; then
    status="ok"; color="$c_grn"
  else
    status="failed"; color="$c_red"
  fi
  printf '%-22s %s%-9s%s %10.3f %10.3f %10.3f\n' \
    "$name" "$color" "$(printf '%s' "$status" | tr '[:lower:]' '[:upper:]')" "$c_reset" "$mean" "$min" "$max"
  emit_row "$name" "$status" "$mean" "$sd" "$min" "$max"
done

# ---------------------------------------------------------------------------
# Write results.json + results.md.
# ---------------------------------------------------------------------------
jq -s '.' "$ROWS_TMP" > "$OUT_DIR/results.json"

# Literal backticks below are intentional Markdown, not command substitution.
# shellcheck disable=SC2016
{
  printf '# bench-format-lint results\n\n'
  printf -- '- timestamp: `%s`\n' "$TS_ISO"
  printf -- '- git sha: `%s`\n' "$GIT_SHA_SHORT"
  printf -- '- engine: `%s`\n' "$([ -n "$USE_HYPERFINE" ] && echo hyperfine || echo 'bash+python3')"
  printf -- '- runs/warmup: `%s` / `%s`\n' "$RUNS" "$WARMUP"
  printf -- '- cpu: `%s` (%s cores)\n\n' "$CPU" "$NCPU"
  printf '| Step | Status | Mean (s) | Stddev | Min (s) | Max (s) |\n'
  printf '| ---- | ------ | -------- | ------ | ------- | ------- |\n'
  jq -r '.[] |
    "| \(.step) | \(.status) | \(.mean_s // "-") | \(.stddev_s // "-") | \(.min_s // "-") | \(.max_s // "-") |"' \
    "$OUT_DIR/results.json"
} > "$OUT_DIR/results.md"

printf '\n%sWrote%s %s\n' "$c_bld" "$c_reset" "$OUT_DIR/results.md"
printf '%sHistory%s %s (%s rows total)\n' "$c_bld" "$c_reset" "$HISTORY" "$(wc -l < "$HISTORY" | tr -d ' ')"
