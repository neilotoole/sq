# rqlite (`rqlite` driver)

Networked distributed SQLite via [rqlite](https://rqlite.io). Driver type in `sq driver ls`: **`rqlite`**.

**Canonical docs:** [rqlite driver](https://sq.io/docs/drivers/rqlite/)

## Add a source

Use [`sq add`](https://sq.io/docs/cmd/add) with an HTTP(S) URL:

```shell
sq add 'rqlite://localhost:4001'                            # plain HTTP
sq add 'rqlites://node.example.com:4001'                    # HTTPS
sq add 'rqlite://sakila:p_ssW0rd@localhost:4001' --handle @rq
```

Default port: `4001`. The driver type is inferred from the `rqlite://` or `rqlites://` scheme; no auto-detect from a bare URL.

## Common parameters

- **`level`**: read consistency. `none` (fastest, may be stale), `weak`, `linearizable`, `strong` (safest). See [rqlite consistency](https://rqlite.io/docs/api/read-consistency/).
- **`disableClusterDiscovery`**: `true` or `false`. Disable gorqlite's automatic peer discovery. Use when behind a proxy or talking to a single node.

Example:

```shell
sq add 'rqlite://sakila:p_ssW0rd@localhost:4001?level=strong&disableClusterDiscovery=true'
```

## Notes

- rqlite executes SQLite SQL, so queries written for `@sqlite_handle` translate verbatim to `@rqlite_handle`.
- **No interactive transactions.** rqlite's HTTP API exposes single statements or atomic batches; there is no `BEGIN…COMMIT`. `sq` handles this transparently: single-statement writes are atomic by themselves, and `sq tbl copy` / `AlterTableColumnKinds` use atomic batches under the hood.
- **`sq tbl truncate`** is intentionally non-atomic across `DELETE` and the AUTOINCREMENT-counter reset. The deleted-row count is accurate; counter reset is informational.
- **`sq tbl copy` and `ALTER COLUMN TYPE` are lossy.** UNIQUE, FOREIGN KEY, AUTOINCREMENT, CHECK constraints, indexes, triggers, and the original DEFAULT expression values are not preserved (substituted by canned per-kind defaults). Names, kinds, single-column PK, NOT NULL, and the *presence* of a default are preserved. Faithful preservation tracked at <https://github.com/neilotoole/sq/issues/737>.
- **No schemas or catalogs.** SQLite has no schema or catalog concept; `sq inspect` reports them as the conventional `main` and `default`.
