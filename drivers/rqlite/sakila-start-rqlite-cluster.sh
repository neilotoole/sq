#!/usr/bin/env bash
#
# sakila-start-rqlite-cluster.sh
#
# Start a 3-node rqlite cluster on 127.0.0.1 (HTTP ports 4001/4003/4005,
# Raft ports 4002/4004/4006) and load the Sakila sample database into
# the leader. Because every node binds and advertises 127.0.0.1,
# gorqlite's cluster discovery returns host-reachable addresses, so
# `sq` can talk to the cluster WITHOUT ?disableClusterDiscovery=true,
# exercising the real discovery + leader-redirect path.
#
# This is the local-machine analog of sakiladb/rqlite's
# cluster-compose.yml, sidestepping the resolver problem that
# Docker-based clusters hit when reached from the host (each container
# advertises an internal hostname like rqlite1 that the host can't
# resolve). For the equivalent single-node Docker setup with Sakila
# preloaded, see sakila-start-local.sh at the repo root.
#
# By default the nodes serve plain HTTP. Pass HTTPS=true (as an
# argument or an environment variable) to generate a self-signed
# certificate and serve the HTTP API over HTTPS instead:
#
#   ./sakila-start-rqlite-cluster.sh HTTPS=true
#
# Prerequisites: rqlited + curl on PATH, plus openssl for HTTPS mode.
# On macOS: `brew install rqlite`.
# See https://rqlite.io/docs/install-rqlite/ for other platforms.
#
# Run in the foreground. Ctrl-C tears down all three nodes and removes
# their data directory.

set -euo pipefail

# Sakila SQLite database loaded into the cluster leader. Overridable
# via the environment.
SAKILA_DB_URL="${SAKILA_DB_URL:-https://raw.githubusercontent.com/neilotoole/sq/master/drivers/sqlite3/testdata/sakila.db}"

HTTPS="${HTTPS:-false}"
for arg in "$@"; do
    case "$arg" in
        HTTPS=true) HTTPS=true ;;
        HTTPS=false) HTTPS=false ;;
        *)
            echo "Usage: $0 [HTTPS=true]" >&2
            exit 1
            ;;
    esac
done

command -v rqlited >/dev/null || {
    echo "rqlited not found; on macOS install via 'brew install rqlite'" >&2
    exit 1
}
command -v curl >/dev/null || {
    echo "curl not found on PATH" >&2
    exit 1
}
if [[ "$HTTPS" == "true" ]]; then
    command -v openssl >/dev/null || {
        echo "openssl not found on PATH (required for HTTPS mode)" >&2
        exit 1
    }
fi

DATA_DIR="$(mktemp -d -t sakila-rq-cluster.XXXX)"

scheme=http
tls_flags=()
curl_opts=()
if [[ "$HTTPS" == "true" ]]; then
    scheme=https
    # Self-signed cert with SANs for localhost/127.0.0.1: Go's TLS
    # stack ignores CN-only certs. The -config form (rather than
    # -addext) also works with the older LibreSSL shipped on macOS.
    cat > "$DATA_DIR/openssl.cnf" <<'CNF'
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = localhost
[ext]
subjectAltName = DNS:localhost, IP:127.0.0.1
CNF
    echo "Generating self-signed certificate..."
    openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days 1 \
        -keyout "$DATA_DIR/key.pem" -out "$DATA_DIR/cert.pem" \
        -config "$DATA_DIR/openssl.cnf" 2>/dev/null
    tls_flags=(-http-cert="$DATA_DIR/cert.pem" -http-key="$DATA_DIR/key.pem")
    curl_opts=(--cacert "$DATA_DIR/cert.pem")
fi

cleanup() {
    echo
    echo "Stopping rqlite nodes..."
    # Intentional word-splitting on the PID list.
    # shellcheck disable=SC2046
    kill $(jobs -p) 2>/dev/null || true
    wait 2>/dev/null || true
    rm -rf "$DATA_DIR"
    echo "Done."
}
trap cleanup EXIT INT TERM HUP

echo "Starting rqlite cluster ($scheme; data dir: $DATA_DIR)"

# ${arr[@]+...} expansions guard against empty arrays under set -u
# with macOS's bash 3.2.
rqlited \
    -node-id=1 \
    -http-addr=127.0.0.1:4001 \
    -raft-addr=127.0.0.1:4002 \
    ${tls_flags[@]+"${tls_flags[@]}"} \
    "$DATA_DIR/node1" &> "$DATA_DIR/node1.log" &

# Let node1 bind its Raft port before node2/3 try to join.
sleep 1

rqlited \
    -node-id=2 \
    -http-addr=127.0.0.1:4003 \
    -raft-addr=127.0.0.1:4004 \
    -join=127.0.0.1:4002 \
    ${tls_flags[@]+"${tls_flags[@]}"} \
    "$DATA_DIR/node2" &> "$DATA_DIR/node2.log" &

rqlited \
    -node-id=3 \
    -http-addr=127.0.0.1:4005 \
    -raft-addr=127.0.0.1:4006 \
    -join=127.0.0.1:4002 \
    ${tls_flags[@]+"${tls_flags[@]}"} \
    "$DATA_DIR/node3" &> "$DATA_DIR/node3.log" &

# Wait up to 30s for the leader's /readyz to return 200.
for _ in {1..30}; do
    if curl -fsS ${curl_opts[@]+"${curl_opts[@]}"} \
        "$scheme://127.0.0.1:4001/readyz" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

if ! curl -fsS ${curl_opts[@]+"${curl_opts[@]}"} \
    "$scheme://127.0.0.1:4001/readyz" >/dev/null 2>&1; then
    echo "Leader did not become ready within 30s." >&2
    echo "Inspect $DATA_DIR/node1.log for details." >&2
    exit 1
fi

echo "Loading Sakila into leader..."
sakila_db="$DATA_DIR/sakila.db"
curl -fsSL "$SAKILA_DB_URL" -o "$sakila_db"
curl -fsS ${curl_opts[@]+"${curl_opts[@]}"} \
    -X POST "$scheme://127.0.0.1:4001/db/load" \
    -H 'Content-Type: application/octet-stream' \
    --data-binary "@$sakila_db" >/dev/null

add_loc="rqlite://localhost:4001"
https_note=""
if [[ "$HTTPS" == "true" ]]; then
    add_loc="rqlite://localhost:4001?tls=true&insecure=true"
    https_note="
The certificate is self-signed, hence insecure=true. Adding the bare
location rqlite://localhost:4001 instead demonstrates sq's add-time
probe: it detects TLS, fails certificate verification, and errors with
instructions.
"
fi

cat <<MSG

Cluster ready: 3 nodes, leader on $scheme://localhost:4001.

Try it from another terminal (note: no disableClusterDiscovery):

    sq add '$add_loc' --handle @rq_local
    sq inspect @rq_local
$https_note
Logs: $DATA_DIR/node{1,2,3}.log

Press Ctrl-C here to stop the cluster.
MSG

wait
