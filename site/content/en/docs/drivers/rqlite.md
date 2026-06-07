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
# Plain HTTP
$ sq add 'rqlite://localhost:4001'

# HTTPS
$ sq add 'rqlites://node.example.com:4001'

# With credentials and a custom handle
$ sq add 'rqlite://sakila:p_ssW0rd@localhost:4001' --handle @rq
```

If the port is omitted, `sq` auto-applies the default port `4001`.
rqlite does not auto-detect from a URL alone, so the driver type is
inferred from the `rqlite://` / `rqlites://` scheme prefix.

## Connection string format

```text
rqlite://username:password@hostname:port
rqlite://username:password@hostname:port?param=value
rqlites://username:password@hostname:port?param=value
```

For multi-node clusters, point at any node. gorqlite discovers peers
and follows leader redirects automatically. Disable that with
`?disableClusterDiscovery=true` when talking to a single node behind a
proxy that should not receive cluster gossip.

## Connection parameters

Pass parameters as URL query strings:

```shell
$ sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?level=strong'
```

**`level`**: rqlite read consistency level. Default is rqlite's default.

| Value          | Behavior                                                 |
|----------------|----------------------------------------------------------|
| `none`         | Reads from any node. Fastest. May be stale.              |
| `weak`         | Checks the receiving node is the leader.                 |
| `linearizable` | Confirms leader via Raft round-trip.                     |
| `strong`       | Safest; serializable read with full leader verification. |

See [rqlite consistency docs](https://rqlite.io/docs/api/read-consistency/).

**`disableClusterDiscovery`**: `true` or `false`. Disable gorqlite's
automatic peer discovery. Useful when the rqlite node is reachable only
through a proxy and shouldn't be probed for cluster peers.

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

## Limitations

- **`sq tbl copy` and `ALTER TABLE` kind swaps are lossy.** The driver
  rebuilds the target table from `sq`'s metadata model, which preserves
  column names, kinds, single-column primary keys, NOT NULL, and the
  *presence* of column defaults. It does **not** preserve UNIQUE
  constraints, FOREIGN KEY constraints, AUTOINCREMENT, CHECK
  constraints, indexes, triggers, or the original DEFAULT *expression
  values* (substituted by canned per-kind defaults like `0` or `''`).
  Faithful preservation via SQL-text rewrite is tracked in
  [#737](https://github.com/neilotoole/sq/issues/737).
- **Schemas and catalogs are not supported.** SQLite has no schema or
  catalog concept, so `sq inspect` reports them as the conventional
  values `main` and `default` respectively. `CreateSchema`,
  `DropSchema`, and catalog operations return explicit "not supported"
  errors.

## Inspect field provenance

`sq inspect` populates the fields below from rqlite's HTTP status
endpoints, SQLite pragmas, and `sqlite_master`.

### Source-level fields

| Field         | Source                                                                          |
| ------------- | ------------------------------------------------------------------------------- |
| `name`        | first row of `pragma_database_list` (typically `main`)                          |
| `schema`      | same as `name`                                                                  |
| `catalog`     | hardcoded `default` (SQLite has no catalog concept)                             |
| `user`        | populated from the URL's userinfo (e.g. `sakila`) if present                    |
| `db_product`  | `"SQLite3 v" + db_version` (rqlite's storage engine)                            |
| `db_version`  | `sqlite_version()` reported by the rqlite leader                                |
| `size`        | not reported. rqlite does not expose a single-file size over its HTTP API.      |

### Per-table fields

| Field       | Source                                                  |
| ----------- | ------------------------------------------------------- |
| `row_count` | live `SELECT COUNT(*) FROM "tbl"` per table             |
| `size`      | not reported. rqlite does not expose per-table storage. |
