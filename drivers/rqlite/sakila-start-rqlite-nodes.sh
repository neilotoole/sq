#!/usr/bin/env bash
#
# sakila-start-rqlite-nodes.sh
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
# Prerequisites: rqlited + curl on PATH. On macOS: `brew install rqlite`.
# See https://rqlite.io/docs/install-rqlite/ for other platforms.
#
# Run in the foreground. Ctrl-C tears down all three nodes and removes
# their data directory.

set -euo pipefail

command -v rqlited >/dev/null || {
    echo "rqlited not found; on macOS install via 'brew install rqlite'" >&2
    exit 1
}
command -v curl >/dev/null || {
    echo "curl not found on PATH" >&2
    exit 1
}

DATA_DIR="$(mktemp -d -t sakila-rq-nodes.XXXX)"

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

echo "Starting rqlite cluster (data dir: $DATA_DIR)"

rqlited \
    -node-id=1 \
    -http-addr=127.0.0.1:4001 \
    -raft-addr=127.0.0.1:4002 \
    "$DATA_DIR/node1" &> "$DATA_DIR/node1.log" &

# Let node1 bind its Raft port before node2/3 try to join.
sleep 1

rqlited \
    -node-id=2 \
    -http-addr=127.0.0.1:4003 \
    -raft-addr=127.0.0.1:4004 \
    -join=127.0.0.1:4002 \
    "$DATA_DIR/node2" &> "$DATA_DIR/node2.log" &

rqlited \
    -node-id=3 \
    -http-addr=127.0.0.1:4005 \
    -raft-addr=127.0.0.1:4006 \
    -join=127.0.0.1:4002 \
    "$DATA_DIR/node3" &> "$DATA_DIR/node3.log" &

# Wait up to 30s for the leader's /readyz to return 200.
for _ in {1..30}; do
    if curl -fsS http://127.0.0.1:4001/readyz >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

if ! curl -fsS http://127.0.0.1:4001/readyz >/dev/null 2>&1; then
    echo "Leader did not become ready within 30s." >&2
    echo "Inspect $DATA_DIR/node1.log for details." >&2
    exit 1
fi

echo "Loading Sakila into leader..."
sakila_db="$DATA_DIR/sakila.db"
curl -fsSL https://sq.io/testdata/sakila.db -o "$sakila_db"
curl -fsS -X POST 'http://127.0.0.1:4001/db/load' \
    -H 'Content-Type: application/octet-stream' \
    --data-binary "@$sakila_db" >/dev/null

cat <<MSG

Cluster ready: 3 nodes, leader on http://localhost:4001.

Try it from another terminal (note: no disableClusterDiscovery):

    sq add 'rqlite://localhost:4001' --handle @rq_local
    sq inspect @rq_local

Logs: $DATA_DIR/node{1,2,3}.log

Press Ctrl-C here to stop the cluster.
MSG

wait
