---
name: sq
description: >-
  Guides use of the sq CLI to query SQL databases and tabular files with SLQ (jq-like SQL syntax)
  or native SQL, manage sources, choose output formats, and run inspect, diff, and table
  commands. Use when the user mentions sq, SLQ, wrangling CSV/Excel/JSON/DB data, cross-source
  joins, or command-line data pipelines after installing sq from https://sq.io.
license: MIT
compatibility: Requires the sq CLI on PATH; install from https://sq.io/docs/install/
metadata:
  author: Todd Papaioannou
  version: "0.50.0"
  homepage: https://sq.io
---

# sq (CLI)

`sq` is a command-line tool for structured data: SQL databases and formats like CSV, TSV, JSON, and Excel. It combines SQL- and `jq`-style querying. Official documentation lives at [sq.io](https://sq.io/).

This skill assumes **sq is already installed** and the user is **not** working from the `sq` source repository. Prefer **`sq help`**, **`sq <command> --help`**, and [sq.io](https://sq.io/) over guessing flags.

## Verify the install

```shell
sq --version
sq help
sq driver ls
```

`sq driver ls` lists drivers available in this build (e.g. `postgres`, `sqlite3`, `csv`).

## Sources and handles

1. **Add** a data source with [`sq add`](https://sq.io/docs/cmd/add). You get a **handle** (e.g. `@my_pg`).
2. **List** sources: `sq ls`.
3. **Active source**: [`sq src`](https://sq.io/docs/cmd/src) shows or sets which source SLQ queries use when you omit an explicit handle.

Use **`@handle`** to target a source in queries (e.g. `@my_pg.actor`). Concepts: [handle](https://sq.io/docs/concepts#handle), [sources](https://sq.io/docs/source).

## Query modes

| Mode              | When to use                                                                                          |
| ----------------- | ---------------------------------------------------------------------------------------------------- |
| **SLQ** (default) | `sq`’s jq-like query language on sources and tables. See [Query language](https://sq.io/docs/query). |
| **Native SQL**    | Database-specific SQL via [`sq sql`](https://sq.io/docs/cmd/sql/).                                   |

Cross-source joins (e.g. CSV to Postgres): [Cross-source joins](https://sq.io/docs/query#cross-source-joins).

## Ping and inspect

- **`sq ping @handle`** — connectivity check ([ping](https://sq.io/docs/cmd/ping)).
- **`sq inspect …`** — schema, columns, sizes ([inspect](https://sq.io/docs/inspect)).

## Output formats

Results can be printed as text, JSON, CSV, HTML, Markdown, XML, XLSX, etc. See [Output formats](https://sq.io/docs/output#formats) and [insert](https://sq.io/docs/output#insert) for writing query results into a database.

Common flags: `-j` / `--json`, `-t` text, `-o` file; details in **`sq --help`** and the docs above.

## Diff and table operations

- **`sq diff`** — compare metadata or row data between sources or tables ([diff](https://sq.io/docs/diff)).
- **`sq tbl`** — copy, truncate, drop tables ([tbl copy](https://sq.io/docs/cmd/tbl-copy), [truncate](https://sq.io/docs/cmd/tbl-truncate), [drop](https://sq.io/docs/cmd/tbl-drop)).

## Driver-specific help (load on demand)

When the task involves a **specific driver** (connection strings, options, caveats), open the matching file under `references/`:

| Driver (as in `sq driver ls`) | Reference                                            |
| ----------------------------- | ---------------------------------------------------- |
| `sqlite3`                     | [references/sqlite3.md](references/sqlite3.md)       |
| `postgres`                    | [references/postgres.md](references/postgres.md)     |
| `sqlserver`                   | [references/sqlserver.md](references/sqlserver.md)   |
| `mysql`                       | [references/mysql.md](references/mysql.md)           |
| `clickhouse`                  | [references/clickhouse.md](references/clickhouse.md) |
| `csv`                         | [references/csv.md](references/csv.md)               |
| `tsv`                         | [references/tsv.md](references/tsv.md)               |
| `json`                        | [references/json.md](references/json.md)             |
| `jsona`                       | [references/jsona.md](references/jsona.md)           |
| `jsonl`                       | [references/jsonl.md](references/jsonl.md)           |
| `xlsx`                        | [references/xlsx.md](references/xlsx.md)             |

Overview of all drivers: [Drivers](https://sq.io/docs/drivers/).

## Online resources

- [Tutorial](https://sq.io/docs/tutorial/), [Cookbook](https://sq.io/docs/cookbook/), [Concepts](https://sq.io/docs/concepts/)
