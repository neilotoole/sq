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
[rqlite](https://rqlite.io), the lightweight distributed SQLite database.
It uses the [`rqlite/gorqlite`](https://github.com/rqlite/gorqlite)
library and talks to rqlite over HTTP.

Unlike `sq`'s built-in SQLite driver, rqlite is networked: there is no
local file mode. The driver implements all optional `sq` driver
features. Because the SQL dialect underneath rqlite is SQLite, queries
written for `@my_sqlite` translate verbatim to `@my_rqlite`.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source. The location argument is
an HTTP(S) URL using one of two schemes:

```shell
# Single-node setup (the common local case): disable cluster discovery
# so the client talks directly to localhost rather than chasing a
# container-internal Raft hostname. See "Single-node localhost" below.
$ sq add 'rqlite://localhost:4001?disableClusterDiscovery=true'

# With credentials and a custom handle
$ sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?disableClusterDiscovery=true' --handle @rq

# Multi-node cluster: leave discovery on. gorqlite follows leader
# redirects automatically.
$ sq add 'rqlite://node1.example.com:4001'

# HTTPS
$ sq add 'rqlites://node.example.com:4001'
```

If the port is omitted, `sq` auto-applies the default port `4001`.
The driver type is inferred from the `rqlite://` / `rqlites://`
scheme prefix; there is no file-based auto-detection.

## Connection string format

```text
rqlite://username:password@hostname:port
rqlite://username:password@hostname:port?param=value
rqlites://username:password@hostname:port?param=value
```

## Common setups

| Setup                                                       | Recommended URL                                                                  |
|-------------------------------------------------------------|----------------------------------------------------------------------------------|
| Single-node `docker run -p 4001:4001 rqlite/rqlite` (host)  | `rqlite://localhost:4001?disableClusterDiscovery=true`                           |
| Single-node `sakiladb/rqlite` (host, with Sakila preloaded) | `rqlite://sakila:p_ssW0rd@localhost:4001?disableClusterDiscovery=true`           |
| Multi-node cluster (production)                             | `rqlite://user:pass@node1:4001` (any node; leave discovery on)                   |

## Single-node localhost

When you run a single rqlite node in Docker and connect to it from your
host (the most common newcomer setup), gorqlite's default behavior is to
ask the node for its cluster peers. The node truthfully reports its own
internal advertise address, which is typically a container-only hostname
like `rqlite1` for the `sakiladb/rqlite` image or the container's short
ID for the official `rqlite/rqlite` image. Your host can't resolve
either of those, and the connection fails with:

```text
tried all peers unsuccessfully. ...
dial tcp: lookup rqlite1: no such host
```

The fix is `?disableClusterDiscovery=true` on the source URL. A
single-node setup has no peers to discover, so disabling discovery costs
nothing and avoids the hostname trap. The
[Common setups](#common-setups) table above includes this for both
common images.

## Connection parameters

Pass parameters as URL query strings:

```shell
$ sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?level=strong&disableClusterDiscovery=true'
```

**`level`**: rqlite read consistency level. Default is rqlite's default.

| Value          | Behavior                                                             |
|----------------|----------------------------------------------------------------------|
| `none`         | Reads from any node. Fastest. May be stale.                          |
| `weak`         | Checks the receiving node is the leader.                             |
| `linearizable` | Confirms leader via Raft round-trip.                                 |
| `strong`       | Routes the read through the Raft log; reflects all committed writes. |

See [rqlite consistency docs](https://rqlite.io/docs/api/read-consistency/).

**`disableClusterDiscovery`**: `true` or `false`. Turns off gorqlite's
automatic peer discovery. Required for the
[single-node localhost](#single-node-localhost) case described above;
also useful when the rqlite node is reachable only through a proxy and
shouldn't be probed for cluster peers. Multi-node cluster users should
leave it off (the default) so leader redirects and failover work
automatically.

## Write behavior

rqlite has no interactive transactions; its HTTP API exposes single
statements via `/db/execute` and atomic batches via the same endpoint
with multiple statements. `sq` maps onto this as follows:

- **Single-statement writes** (`CreateTable`, `INSERT`, `UPDATE`,
  `DELETE`, `ALTER TABLE`) go through `database/sql` as usual. Each is
  one HTTP call and is atomic at the rqlite layer.
- **Multi-statement atomic operations** (`sq tbl copy`'s
  CREATE+INSERT-SELECT, and the `ALTER COLUMN TYPE` table-rebuild dance)
  are sent as a single atomic batch via gorqlite's
  `WriteParameterizedContext`. If any statement fails, rqlite rolls the
  whole batch back.
- **`sq tbl truncate`** issues `DELETE FROM tbl` and (with reset) a
  follow-up `UPDATE sqlite_sequence`. These two statements are
  deliberately not atomic relative to each other. The simpler path
  reports the deleted-row count accurately, and the AUTOINCREMENT-counter
  reset is informational.

## How sq handles rqlite quirks

A few rqlite-specific behaviors are smoothed over inside the driver so
the cross-driver experience matches the rest of `sq`. Worth knowing if
you're comparing notes against raw `gorqlite` results:

- **Column types for empty tables.** gorqlite's `database/sql` adapter
  doesn't expose column type names to callers, so a fresh
  `CREATE TABLE` followed by an empty `SELECT` would normally yield
  `kind.Unknown` for every column. `sq`'s rqlite driver wraps the
  underlying gorqlite SQL driver to expose the type names that gorqlite
  has been carrying all along, so `sq inspect` and the SLQ engine see
  proper kinds even on empty tables.
- **JSON-numeric coercion.** rqlite returns all numeric column values
  as JSON numbers, which Go unmarshals to `float64` by default. The
  driver coerces these at materialization time: integer-kind columns
  return `int64`, decimal-kind columns return `decimal.Decimal` (with
  integer values surfacing as `int64` to match the cross-driver
  int contract), and float-kind columns stay `float64`. So
  `SELECT actor_id FROM actor` against an `INTEGER PRIMARY KEY` column
  comes back as `int64` in your output, not `float64`.

## Limitations

- **`sq tbl copy` and `ALTER TABLE` kind swaps are lossy.** The driver
  rebuilds the target table from `sq`'s metadata model, which preserves
  column names, kinds, single-column primary keys, `NOT NULL`, and the
  *presence* of column defaults. It does **not** preserve `UNIQUE`
  constraints, `FOREIGN KEY` constraints, `AUTOINCREMENT`, `CHECK`
  constraints, indexes, triggers, or the original `DEFAULT` *expression
  values* (substituted by canned per-kind defaults like `0` or `''`).
  Faithful preservation via SQL-text rewrite is tracked in
  [#737](https://github.com/neilotoole/sq/issues/737).
- **Schemas and catalogs are not supported.** SQLite has no schema or
  catalog concept, so `sq inspect` reports them as the conventional
  values `main` and `default` respectively. `CreateSchema`,
  `DropSchema`, and catalog operations return explicit "not supported"
  errors.

## Inspect field provenance

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

The `sakiladb/rqlite` project publishes a three-node cluster compose file at
[`cluster-compose.yml`](https://github.com/sakiladb/rqlite/blob/master/cluster-compose.yml).
Fetch it and bring the cluster up:

```shell
$ curl -fLO https://raw.githubusercontent.com/sakiladb/rqlite/master/cluster-compose.yml
$ docker compose -f cluster-compose.yml up -d
```

That brings up `rqlite1` (leader, host port `4001`), `rqlite2` (follower,
host port `4003`), and `rqlite3` (follower, host port `4005`). The
followers start with empty volumes and receive Sakila from the leader via
Raft snapshot within a few seconds.

Each node advertises its container hostname over `/status`, which isn't
resolvable from the host. From a developer machine the simplest approach
is to disable cluster discovery and point `sq` at one specific node. The
SQL layer still works; `sq` just won't follow leader redirects.

```shell
$ sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?disableClusterDiscovery=true' \
    --handle @rq_cluster

$ sq inspect @rq_cluster
```

In a real deployment where the node hostnames are resolvable from
clients, leave discovery enabled (the default) so leader redirects and
failover work automatically:

```shell
$ sq add 'rqlite://user:pass@rqlite1.internal:4001' --handle @rq_prod
```

Tear down the cluster:

```shell
$ docker compose -f cluster-compose.yml down -v
```
