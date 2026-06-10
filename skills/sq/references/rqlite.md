# rqlite (`rqlite` driver)

Networked distributed SQLite via [rqlite](https://rqlite.io). Driver type in `sq driver ls`: **`rqlite`**.

**Canonical docs:** [rqlite driver](https://sq.io/docs/drivers/rqlite/)

## Add a source

Use [`sq add`](https://sq.io/docs/cmd/add) with an HTTP(S) URL. Default port: `4001` (auto-applied when missing). Driver type is inferred from the `rqlite://` scheme. TLS is opt-in via `?tls=true`, but `sq add` auto-detects TLS-only endpoints and applies it (skipped with `--skip-verify`, explicit `tls`/`insecure` params, or `${...}` placeholder locations); add `?tls=true&insecure=true` to accept self-signed certificates.

**Common setups:**

- Single-node `docker run -p 4001:4001 rqlite/rqlite` from the host:

  ```shell
  sq add 'rqlite://localhost:4001?disableClusterDiscovery=true'
  ```

- Single-node `sakiladb/rqlite` from the host (Sakila preloaded; default creds `sakila` / `p_ssW0rd`):

  ```shell
  sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?disableClusterDiscovery=true' --handle @rq
  ```

- Multi-node cluster (production):

  ```shell
  sq add 'rqlite://user:pass@node1:4001'   # any node; leave discovery on
  ```

**Why `?disableClusterDiscovery=true` for the localhost case:** gorqlite asks the node for its cluster peers. A single-node Docker container reports its own internal hostname (e.g. `rqlite1` for `sakiladb/rqlite`, or the container short ID for `rqlite/rqlite`) which is unresolvable from the host. The connection then fails with `dial tcp: lookup rqlite1: no such host`. Disabling discovery sidesteps this. Single-node setups have no peers to discover anyway.

## Common parameters

- **`level`**: read consistency. `none` (fastest, may be stale), `weak`, `linearizable`, `strong` (safest). See [rqlite consistency](https://rqlite.io/docs/api/read-consistency/).
- **`disableClusterDiscovery`**: `true` or `false`. Set `true` for single-node localhost (see above). Leave off (the default) for multi-node clusters so leader redirects and failover work automatically.

## Notes

- rqlite executes SQLite SQL, so queries written for `@sqlite_handle` translate verbatim to `@rqlite_handle`.
- **No interactive transactions.** rqlite's HTTP API exposes single statements or atomic batches; there is no `BEGIN…COMMIT`. `sq` handles this transparently: single-statement writes are atomic by themselves. `sq tbl copy --data` and `AlterTableColumnKinds` use atomic batches under the hood; `sq tbl copy` with structure only is a single CREATE TABLE.
- **`sq tbl truncate`** is intentionally non-atomic across `DELETE` and the AUTOINCREMENT-counter reset. The deleted-row count is accurate; counter reset is informational.
- **`sq tbl copy` and `ALTER COLUMN TYPE` preserve DDL constraints** (UNIQUE, FOREIGN KEY, AUTOINCREMENT, CHECK, composite PRIMARY KEY, exact DEFAULT expressions). Indexes and triggers are not copied; tracked at <https://github.com/neilotoole/sq/issues/758>.
- **No schemas or catalogs.** SQLite has no schema or catalog concept; `sq inspect` reports them as the conventional `main` and `default`.
