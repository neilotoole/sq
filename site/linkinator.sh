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
  LINKINATOR_ARGS+=(-s '^http://(?!127\\.0\\.0\\.1|localhost)')
  LINKINATOR_ARGS+=(-s '^https://(?!127\\.0\\.0\\.1|localhost)')
fi

bunx linkinator "${LINKINATOR_ARGS[@]}"
echo "Linkinator finished"
