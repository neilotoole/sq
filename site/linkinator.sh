#!/usr/bin/env bash
# Use linkinator to verify links.
# For best results, we start a local web server (as a background process),
# and point linkinator at it.
#
# Note also: linkinator.config.json

set -euo pipefail

SERVER_PID=""
SERVER_LOG=""

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
bunx linkinator --config ./linkinator.config.json -r "${base_url}"
echo "Linkinator finished"
