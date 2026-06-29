#!/usr/bin/env bash

# Starts local Postgres, MySQL, SQL Server, ClickHouse, Oracle, and rqlite
# (via the sakiladb/* docker images) for repo-wide integration tests.
#
# Image tags, ports, DSNs, and env-var names come from .github/sakila-db.json
# (single source of truth, shared with CI). Each engine uses its first tag
# (tags[0], normally "latest"). `--pull always` avoids a silently-stale image.
#
# NOTE: tested on macOS / Apple Silicon. SQL Server is amd64-only.

set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
config="$here/.github/sakila-db.json"

# Stop anything already running.
"$here/sakila-stop-local.sh" &>/dev/null || true

declare -A cname=(
  [postgres]=sakiladb-pg [mysql]=sakiladb-my [sqlserver]=sakiladb-ms
  [clickhouse]=sakiladb-ch [oracle]=sakiladb-or [rqlite]=sakiladb-rq
)
declare -A platform=( [sqlserver]="--platform=linux/amd64" )

engines=(postgres mysql sqlserver clickhouse oracle rqlite)
exports=()

for engine in "${engines[@]}"; do
  tag=$(jq -r --arg e "$engine" '.[$e].tags[0]' "$config")
  port=$(jq -r --arg e "$engine" '.[$e].port' "$config")
  env=$(jq -r --arg e "$engine" '.[$e].env' "$config")
  dsn=$(jq -r --arg e "$engine" '.[$e].dsn' "$config")
  # shellcheck disable=SC2086
  docker run -d --pull always ${platform[$engine]:-} \
    -p "$port:$port" --name "${cname[$engine]}" "sakiladb/$engine:$tag" &>/dev/null
  exports+=("export $env=\"$dsn\"")
done

# Wait for every container to report healthy (images ship a HEALTHCHECK).
echo "Waiting for containers to become healthy..."
for engine in "${engines[@]}"; do
  c="${cname[$engine]}"
  for _ in $(seq 1 60); do
    status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$c" 2>/dev/null || echo missing)
    [ "$status" = healthy ] && break
    [ "$status" = none ] && break   # no healthcheck: don't block
    sleep 5
  done
  echo "  $c: $(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}running{{end}}' "$c" 2>/dev/null || echo '?')"
done

echo
echo "Export these envars (and source them) to run the tests with these sources enabled"
printf '%s\n' "${exports[@]}"
# Also export into the current shell when sourced.
for line in "${exports[@]}"; do eval "$line"; done
