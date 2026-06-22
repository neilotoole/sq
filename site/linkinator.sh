#!/usr/bin/env bash
# Use linkinator to verify links.
# For best results, we start a local web server (as a background process),
# and point linkinator at it.
#
# Note also: linkinator.config.json

set -euo pipefail

SERVER_PID=""
SERVER_LOG=""

# Fast mode: keep PR/local iteration snappy. Linkinator by default will crawl
# third-party links too, which is valuable but can look "stuck" in automation
# and is heavily network-dependent. Set to "internal" to only validate local
# pages and assets served from the temporary lint server.
# Values: "full" (default) | "internal"
LINKINATOR_SCOPE="${LINKINATOR_SCOPE:-full}"

# shellcheck disable=SC2329  # invoked indirectly via `trap cleanup EXIT INT TERM`.
cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" >/dev/null 2>&1; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
  if [[ -n "${SERVER_LOG}" ]] && [[ -f "${SERVER_LOG}" ]]; then
    rm -f "${SERVER_LOG}"
  fi
}

trap cleanup EXIT INT TERM

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'EOF'
linkinator.sh

Build a temporary Hugo site into site/.serve-lint, serve it, then run
linkinator against it.

Environment:
  LINKINATOR_SCOPE=full|internal
    - full: crawl local site and follow third-party external links
    - internal: only check site served from the temporary local server
  LINKINATOR_PORT=<port>
    - optional fixed port
  LINKINATOR_ATTEMPT_TIMEOUT=<seconds>
    - per-attempt wall-clock ceiling (default 480); a hung crawl is killed and
      retried instead of stalling
  LINKINATOR_MAX_ATTEMPTS=<n>
    - full scope only: how many times to retry the whole crawl on failure
      (default 3) so the run only fails on links broken on every attempt
EOF
  exit 0
fi

if [[ -n "${1:-}" && "${1:-}" != "internal" && "${1:-}" != "full" ]]; then
  echo "Unknown argument: $1" >&2
  echo "Use: $0 [full|internal]   or set LINKINATOR_SCOPE" >&2
  exit 2
fi

if [[ -n "${1:-}" ]]; then
  LINKINATOR_SCOPE="$1"
fi

case "${LINKINATOR_SCOPE}" in
  full|internal) ;;
  *)
    echo "LINKINATOR_SCOPE must be 'full' or 'internal' (got: ${LINKINATOR_SCOPE})" >&2
    exit 2
    ;;
esac

pick_free_port() {
  python3 - <<'PY'
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

port="${LINKINATOR_PORT:-$(pick_free_port)}"
base_url="http://localhost:${port}"

# Build a nice fresh site into $lint_dir
lint_dir="./.serve-lint"
rm -rf "${lint_dir}"
./node_modules/.bin/hugo/hugo --gc --minify -b "${base_url}" -d "${lint_dir}"

# Start a local webserver (background process)
echo "Starting server for linting at: ${base_url}"
SERVER_LOG="$(mktemp -t linkinator-serve.XXXXXX.log)"
bun scripts/lighthouse-server.ts --port "${port}" --dir "${lint_dir}" >"${SERVER_LOG}" 2>&1 &
SERVER_PID=$!

# Wait for server readiness with a bounded loop.
ready=false
for _ in {1..40}; do
  if ! kill -0 "${SERVER_PID}" >/dev/null 2>&1; then
    echo "Local server exited before startup. Output:" >&2
    sed 's/^/  /' "${SERVER_LOG}" >&2 || true
    exit 1
  fi

  if curl -fsS -o /dev/null "${base_url}/" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.25
done

if [[ "${ready}" != "true" ]]; then
  echo "Local server did not become ready at ${base_url}." >&2
  echo "Server output:" >&2
  sed 's/^/  /' "${SERVER_LOG}" >&2 || true
  exit 1
fi

echo "Server started"
LINKINATOR_ARGS=(--config ./linkinator.config.json -r "${base_url}")
if [[ "${LINKINATOR_SCOPE}" == "internal" ]]; then
  # Do not follow arbitrary third-party http(s) pages from docs, but *do* keep
  # following links to the local lint server (http://localhost:<port>/...).
  #
  # `linkinator` uses regex skip patterns; split http/https to avoid a single
  # overly-broad `https?://` pattern that can accidentally match everything and
  # scan 0 links.
  #
  # In bash single-quoted strings, `\\.` is two literal backslashes + a dot, not
  # a regex-escaped dot. Linkinator does `new RegExp(pattern)`; use `\.` here
  # so argv contains one `\` before each `.` (literal IPv4 dots in the URL).
  LINKINATOR_ARGS+=(-s '^http://(?!127\.0\.0\.1|localhost)')
  LINKINATOR_ARGS+=(-s '^https://(?!127\.0\.0\.1|localhost)')
else
  # Full scope crawls third-party hosts, which fail for reasons that are not
  # broken links: rate limiting, bot-blocking, and transient 5xx blips. Keep
  # genuine dead links (404/410, and 0/network errors after retries) fatal, but
  # downgrade infrastructure noise to warnings so a red run means a real broken
  # link, not a flaky external host. `retry: true` (in linkinator.config.json)
  # already retries 429s honoring `Retry-After`; these mappings catch what is
  # still failing afterwards.
  #   403: forbidden / bot-blocking (many sites reject CI user agents)
  #   429: rate limiting (e.g. wikipedia.org throttles the runner IP)
  #   5xx: transient third-party server errors
  LINKINATOR_ARGS+=(--status-code '403:warn')
  LINKINATOR_ARGS+=(--status-code '429:warn')
  LINKINATOR_ARGS+=(--status-code '5xx:warn')
  # Hammering a host with many parallel requests is what provokes 429s in the
  # first place; a gentler external crawl trips fewer rate limits. Overrides the
  # config's `concurrency`, which stays high for the localhost-only internal
  # crawl.
  LINKINATOR_ARGS+=(--concurrency 4)
fi

# Per-attempt wall-clock ceiling. linkinator's per-request `timeout` does not
# reliably abort a connection that never responds; under Bun a wedged request
# leaves an unsettled top-level await and the process dies with a bare exit 13
# after a long stall. Wrap each attempt in coreutils `timeout` so a hang becomes
# a bounded, retryable failure instead of a mystery crash.
#
# coreutils ships it as `timeout` (Linux/CI) or `gtimeout` (macOS via Homebrew).
# If neither is present (a stock macOS dev box), run without the ceiling rather
# than failing outright; CI always has GNU `timeout`.
attempt_timeout="${LINKINATOR_ATTEMPT_TIMEOUT:-480}"

if command -v timeout >/dev/null 2>&1; then
  run_linkinator() {
    timeout --signal=KILL "${attempt_timeout}" bunx linkinator "${LINKINATOR_ARGS[@]}"
  }
elif command -v gtimeout >/dev/null 2>&1; then
  run_linkinator() {
    gtimeout --signal=KILL "${attempt_timeout}" bunx linkinator "${LINKINATOR_ARGS[@]}"
  }
else
  echo "warning: coreutils 'timeout'/'gtimeout' not found; running without a per-attempt ceiling" >&2
  run_linkinator() {
    bunx linkinator "${LINKINATOR_ARGS[@]}"
  }
fi

rc=0
if [[ "${LINKINATOR_SCOPE}" == "full" ]]; then
  # The external crawl is network-dependent: hosts hang, rate-limit, or blip
  # with transient errors that per-request retries don't fully absorb. Retry the
  # whole crawl so the nightly only goes red on a link that is broken on *every*
  # attempt: real signal, not infrastructure noise. The internal crawl
  # (PR-gating) deliberately does not retry; a broken internal link is
  # deterministic and should fail fast.
  max_attempts="${LINKINATOR_MAX_ATTEMPTS:-3}"
  attempt=1
  while true; do
    echo "Linkinator attempt ${attempt}/${max_attempts} (scope=full)"
    rc=0
    run_linkinator || rc=$?
    if [[ "${rc}" -eq 0 ]]; then
      break
    fi
    echo "Linkinator attempt ${attempt} failed (exit ${rc})." >&2
    if [[ "${attempt}" -ge "${max_attempts}" ]]; then
      break
    fi
    attempt=$((attempt + 1))
    sleep 5
  done
else
  run_linkinator || rc=$?
fi

if [[ "${rc}" -eq 0 ]]; then
  echo "Linkinator finished"
else
  echo "Linkinator failed (exit ${rc})" >&2
fi
exit "${rc}"
