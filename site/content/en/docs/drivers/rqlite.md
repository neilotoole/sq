---
title: "rqlite"
description: "rqlite driver"
draft: false
images: []
weight: 4045
toc: true
url: /docs/drivers/rqlite
---

The `sq` rqlite driver implements connectivity for
[rqlite](https://rqlite.io), the lightweight distributed SQLite database. It uses the
[`rqlite/gorqlite`](https://github.com/rqlite/gorqlite) library and talks to rqlite over HTTP(S).
The SQL dialect underneath is still SQLite, so queries written for a source `@sqlite` translate
verbatim to `@rqlite`.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source.

```shell
# Single-node HTTP setup (the common local case): disable cluster discovery
# so the client talks directly to localhost rather than chasing a
# container-internal Raft hostname. See "Cluster discovery" below.
$ sq add 'rqlite://localhost:4001?disableClusterDiscovery=true'

# With credentials.
$ sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?disableClusterDiscovery=true'

# Multi-node HTTP cluster: leave discovery on. The driver follows leader
# redirects automatically.
$ sq add 'rqlite://node1.example.com:4001'

# HTTPS
$ sq add 'rqlite://node.example.com:4001?tls=true'

# HTTPS with a self-signed certificate
$ sq add 'rqlite://node.example.com:4001?tls=true&insecure=true'
```

If the port is omitted, `sq` auto-applies the default port `4001`.

## HTTP vs HTTPS

rqlite serves plain HTTP by default, and so does this driver: a bare
`rqlite://host:4001` location connects over HTTP. To connect over HTTPS
instead, add `tls=true`:

```shell
# No tls param; defaults to HTTP
$ sq add 'rqlite://node.example.com:4001'

# Explicitly HTTP
$ sq add 'rqlite://node.example.com:4001?tls=false'

# HTTPS
$ sq add 'rqlite://node.example.com:4001?tls=true'
```

{{< alert icon="👉" >}}
If an explicit `tls` param is not provided, at [`sq add`](/docs/cmd/add) time, `sq` probes the
endpoint, and if it detects that the server requires TLS, it stores `tls=true` on the source
automatically:

```shell
# node.example.com is TLS-only: sq detects this, and persists the
# location as rqlite://node.example.com:4001?tls=true
$ sq add 'rqlite://node.example.com:4001'
```

The probe is skipped if you pass `--skip-verify`, if the location already includes a `tls` or
`insecure` param, or if the location contains [secret placeholders](/docs/secrets/#placeholders).
A source is probed only when it's added: if the server's transport changes later, connections fail
with an error that suggests the fix; the saved location is never silently rewritten.
{{< /alert >}}

### Self-signed certificates

If the server presents a self-signed certificate or one issued by a
private CA, certificate verification fails, and `sq add` errors with
instructions. To accept the certificate, add `insecure=true` (valid
only in combination with `tls=true`):

```shell
$ sq add 'rqlite://node.example.com:4001?tls=true&insecure=true'
```

`insecure=true` skips TLS certificate verification for the source.
Prefer installing the CA in your trust store for production use.

## Cluster discovery

By default, the driver asks the node it connects to for its cluster
peers, then uses the peer list for leader redirects and failover. In a
multi-node cluster whose node hostnames are resolvable from the client
(typically via internal DNS), leave discovery enabled: it's what makes
connecting via any node work.

### Single-node localhost

When you run a single rqlite node in Docker and connect to it from your
host (the most common newcomer setup), discovery backfires. The node
truthfully reports its own internal advertise address, which is
typically a container-only hostname like `rqlite1` for the
`sakiladb/rqlite` image or the container's short ID for the official
`rqlite/rqlite` image. Your host can't resolve either of those, and the
connection fails with:

```text
tried all peers unsuccessfully. ...
dial tcp: lookup rqlite1: no such host
```

The fix is `?disableClusterDiscovery=true` on the source URL. A
single-node setup has no peers to discover, so disabling discovery costs
nothing and avoids the hostname trap.

## Connection string

```text
rqlite://username:password@hostname:port
rqlite://username:password@hostname:port?param=value
```

Pass parameters as URL query strings:

```shell
$ sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?level=strong&disableClusterDiscovery=true'
```

### `level`

rqlite read consistency level. Defaults to `weak`.
See [rqlite consistency docs](https://rqlite.io/docs/api/read-consistency/).

| Value          | Behavior                                                             |
|----------------|----------------------------------------------------------------------|
| `none`         | Reads from any node. Fastest. May be stale.                          |
| `weak`         | Checks the receiving node is the leader.                             |
| `linearizable` | Confirms leader via Raft round-trip.                                 |
| `strong`       | Routes the read through the Raft log; reflects all committed writes. |

### `disableClusterDiscovery`

`true` or `false`. Turns off the driver's automatic peer discovery.
Required for the [single-node localhost](#single-node-localhost) case;
also useful when the rqlite node is reachable only through a proxy and
shouldn't be probed for cluster peers. Multi-node cluster users should
leave it off (the default) so leader redirects and failover work
automatically.

### `timeout`

HTTP client timeout in seconds, applied to every request the driver
makes to the rqlite node. Integer-valued; defaults to `10`. Increase it
for slow links or large multi-statement batches; decrease it to
fail-fast against a flaky node.

### `tls`

`true` or `false` (the default). Connect over HTTPS instead of plain
HTTP. Usually set automatically at add time: see
[HTTP vs HTTPS](#http-vs-https).

### `insecure`

`true` or `false` (the default). Skip TLS certificate verification.
Valid only in combination with `tls=true`. See
[Self-signed certificates](#self-signed-certificates).

## Notes

### Write behavior

rqlite has no interactive transactions; its HTTP API exposes single
statements via `/db/execute` and atomic batches via the same endpoint
with multiple statements. `sq` maps onto this as follows:

- **Single-statement writes** (`CreateTable`, `INSERT`, `UPDATE`,
  `DELETE`, `ALTER TABLE`) go through `database/sql` as usual. Each is
  one HTTP call and is atomic at the rqlite layer.
- **Multi-statement atomic operations** (`sq tbl copy`'s
  CREATE+INSERT-SELECT, and the `ALTER COLUMN TYPE` table-rebuild dance)
  are sent as a single atomic batch. If any statement fails, rqlite
  rolls the whole batch back.
- **`sq tbl truncate`** issues `DELETE FROM tbl` and (with reset) a
  follow-up `UPDATE sqlite_sequence`. These two statements are
  deliberately not atomic relative to each other. The simpler path
  reports the deleted-row count accurately, and the AUTOINCREMENT-counter
  reset is informational.

### Quirks

A few rqlite-specific behaviors are smoothed over inside the driver so
the cross-driver experience matches the rest of `sq`. Worth knowing if
you're comparing notes against rqlite's HTTP API:

- **Column types for empty tables.** With no rows to go on, a fresh
  `CREATE TABLE` followed by an empty `SELECT` would normally yield
  `kind.Unknown` for every column. The driver recovers the declared
  column types from rqlite's response metadata, so `sq inspect` and
  the SLQ engine see proper kinds even on empty tables.
- **JSON-numeric coercion.** rqlite returns all numeric column values
  as JSON numbers, which Go unmarshals to `float64` by default. The
  driver coerces these at materialization time: integer-kind columns
  return `int64`, decimal-kind columns return `decimal.Decimal` (with
  integer values surfacing as `int64` to match the cross-driver
  int contract), and float-kind columns stay `float64`. So
  `SELECT actor_id FROM actor` against an `INTEGER PRIMARY KEY` column
  comes back as `int64` in your output, not `float64`.

### Limitations

- **`sq tbl copy` and `ALTER TABLE` kind swaps don't carry indexes or
  triggers.** The table DDL itself is preserved via SQL-text rewrite
  (`UNIQUE`, `FOREIGN KEY`, `AUTOINCREMENT`, `CHECK`, composite
  `PRIMARY KEY`, exact `DEFAULT` expressions, `WITHOUT ROWID`, and
  column comments), matching the
  [sqlite3 driver](/docs/drivers/sqlite). Self-referential foreign
  keys are rewritten to point at the destination table: copying
  `actor` to `actor_bak` produces a `REFERENCES "actor_bak"(id)`
  clause. Indexes and triggers live as separate `sqlite_master` rows
  and aren't carried.
- **Schemas and catalogs are not supported.** SQLite has no schema or
  catalog concept, so `sq inspect` reports them as the conventional
  values `main` and `default` respectively. `CreateSchema`,
  `DropSchema`, and catalog operations return explicit "not supported"
  errors.

## Inspect

[`sq inspect`](/docs/inspect) populates the fields below from rqlite's HTTP status
endpoints, SQLite pragmas, and `sqlite_master`.

### Source-level fields

| Field         | Source                                                                          |
| ------------- | ------------------------------------------------------------------------------- |
| `name`        | first row of `pragma_database_list` (typically `main`)                          |
| `schema`      | same as `name`                                                                  |
| `catalog`     | hardcoded `default` (SQLite has no catalog concept)                             |
| `user`        | not populated by this driver                                                    |
| `db_product`  | `"rqlite (SQLite " + db_version + ")"`                                          |
| `db_version`  | `sqlite_version()` reported by the rqlite leader                                |
| `size`        | not reported. rqlite does not expose a single-file size over its HTTP API.      |

### Per-table fields

| Field       | Source                                                  |
| ----------- | ------------------------------------------------------- |
| `row_count` | live `SELECT COUNT(*) FROM "tbl"` per table             |
| `size`      | not reported. rqlite does not expose per-table storage. |

## Example usage

Both examples use the
[`sakiladb/rqlite`](https://hub.docker.com/r/sakiladb/rqlite) image, which
ships rqlite preloaded with the
[Sakila](https://dev.mysql.com/doc/sakila/en/) sample database. Default
credentials are `sakila` / `p_ssW0rd`.

### Single node

Start a single-node container, add the source, and inspect:

```shell
# Port 4001 is rqlite's HTTP API.
$ docker run --rm -d --name sakila-rq -p 4001:4001 sakiladb/rqlite:latest

# Add the source. ?disableClusterDiscovery=true is required when reaching
# the container from the host (see Single-node localhost above).
$ sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?disableClusterDiscovery=true' \
    --handle @rq

$ sq inspect @rq

# Tear down
$ docker stop sakila-rq
```

### Multiple nodes

This (macOS-tested) example demonstrates a real local cluster that exercises cluster discovery and
leader redirects (i.e. _without_ `?disableClusterDiscovery=true`). It starts three native `rqlited`
processes on `127.0.0.1`, each advertising a host-reachable address. Native processes, not Docker:
multi-node Docker setups such as `sakiladb/rqlite`'s
[`cluster-compose.yml`](https://github.com/sakiladb/rqlite/blob/master/cluster-compose.yml)
advertise container-internal hostnames that the host can't resolve, the same trap described in
[Single-node localhost](#single-node-localhost).

The example
[`sakila-start-rqlite-nodes.sh`](https://raw.githubusercontent.com/neilotoole/sq/master/drivers/rqlite/sakila-start-rqlite-nodes.sh)
script brings the cluster up and loads Sakila into the leader. It requires the `rqlited` binary
(`brew install rqlite` on macOS; see [rqlite.io](https://rqlite.io/docs/install-rqlite/) for other
platforms):

```shell
# In one terminal, download the helper and run it.
$ curl -fsSL -o sakila-start-rqlite-nodes.sh \
    https://raw.githubusercontent.com/neilotoole/sq/master/drivers/rqlite/sakila-start-rqlite-nodes.sh
$ chmod +x sakila-start-rqlite-nodes.sh
$ ./sakila-start-rqlite-nodes.sh
Starting rqlite cluster (data dir: /tmp/sakila-rq-nodes.XXXX)
Loading Sakila into leader...

Cluster ready: 3 nodes, leader on http://localhost:4001.
...
Press Ctrl-C here to stop the cluster.

# In another terminal:
$ sq add 'rqlite://localhost:4001' --handle @rq_local
$ sq inspect @rq_local
```

Ctrl-C in the first terminal tears the cluster down and removes its
data directory.
