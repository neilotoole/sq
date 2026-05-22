---
title: Inspect
description: Inspect source or table metadata
lead: ''
draft: false
images:
weight: 1037
toc: true
url: /docs/inspect
---
[`sq inspect`](/docs/cmd/inspect) inspects metadata (schema/structure, tables, columns) for a source,
or for an individual table. When used with `--json`, the output of `sq inspect` can
be fed into other tools such as [jq](https://jqlang.github.io/jq/) to enable complex data pipelines.

Let's start off with a single source, a Postgres [Sakila](/docs/develop/sakila/) database:

```shell
# Start the Postgres container
$ docker run -d -p 5432:5432 sakiladb/postgres:12

# Add the source
$ sq add postgres://sakila:p_ssW0rd@localhost/sakila --handle @sakila_pg
@sakila_pg  postgres  sakila@localhost/sakila
```

## Inspect source

Use `sq inspect @sakila_pg` to inspect the source.

{{< alert icon="👉" >}}
You can also use `sq inspect` with `stdin`, e.g.:

```shell
$ cat actor.csv | sq inspect
```

However, note that `stdin` sources can't take advantage of [ingest caching](/docs/source#ingest), because
the `stdin` pipe is "anonymous", and `sq` can't do a cache lookup for it. If you're going to
repeatedly inspect the same `stdin` data, you should probably just [`sq add`](#add) it.
{{< /alert >}}

This output includes the source
metadata, and the schema structure (tables, columns, etc.).

### `--text` (default)

```shell
$ sq inspect @sakila_pg
```
![sq inspect source text](sq_inspect_source_text.png)

### `--verbose`

To see more detail, use the `--verbose` (`-v`) flag with the `--text` format.

![sq inspect source verbose](sq_inspect_source_text_verbose.png)

### `--yaml`

To see the full output, use the `--yaml` (`-y`) flag. YAML has the advantage
of being reasonably human-readable.

![sq inspect source yaml](sq_inspect_source_yaml.png)

{{< alert icon="⚠️" >}}
If the schema is large and complex, it can take some time (a few seconds or longer)
for `sq` to introspect the schema.
{{< /alert >}}

### `--json`

The `--json` (`-j`) format renders the same content as `--yaml`, but is more
suited for use with other tools, such as [jq](https://jqlang.github.io/jq/).

![sq_inspect_source json](sq_inspect_source_json.png)

Here's an example of using `sq` with jq to list all table names:

```shell
$ sq inspect -j | jq -r '.tables[] | .name'
```

![sq_inspect_pipe_jq_table_names](sq_inspect_pipe_jq_table_names.png)

See more examples in the [cookbook](/docs/cookbook).

### `--markdown`

The `--markdown` format renders a schema document suited for embedding in
project docs or a pull request: a source overview, per-table
column / key / constraint / index detail, and a
[Mermaid](https://mermaid.js.org) entity-relationship diagram. The diagram
renders inline on GitHub, GitLab, and most Markdown viewers, showing every
table and its foreign-key relationships.

```shell
$ sq inspect @sakila_sl3 --markdown

# Equivalent, using the generic format flag:
$ sq inspect @sakila_sl3 -f markdown
```

The output begins with the source overview and the ER diagram (truncated
here):

````markdown
# @sakila_sl3

| Property | Value |
| --- | --- |
| Name | sakila.db |
| Driver | sqlite3 |
| Tables | 16 |
| Views | 5 |

## Entity Relationship Diagram

```mermaid
erDiagram
    actor {
        int actor_id PK
        text first_name
        text last_name
        datetime last_update
    }
    film_actor {
        int actor_id PK,FK
        int film_id PK,FK
        datetime last_update
    }
    actor ||--o{ film_actor : ""
    film ||--o{ film_actor : ""
```
````

Each table then gets its own section, led by a focused diagram of just that
table and its directly-related neighbors (related tables appear as bare,
column-less boxes), followed by a column table and the table's foreign-key,
unique-constraint, and index detail. Identifiers and the column type, key,
and default values are rendered as inline code. Inspecting a single table
(`sq inspect @sakila_sl3.film_actor --markdown`) renders that same per-table
section on its own.

## Source overview

Sometimes you don't need the full schema, but still want to view the source
metadata. Use the `--overview` (`-O`) mode to see just the top-level metadata.
This excludes the schema structure, and is also much faster to complete.


![sq inspect overview text](sq_inspect_source_overview_text.png)

Well, that's not a lot of detail. The `--yaml` output is more useful:

![sq_inspect_overview_yaml](sq_inspect_source_overview_yaml.png)

The `--json` format produces similar output.

## Database properties

The `--dbprops` mode displays the underlying database's properties, server config,
and the like.

```shell
$ sq inspect @sakila_pg --dbprops
```

![sq_inspect_source_dbprops_pg_text](sq_inspect_source_dbprops_pg_text.png)

Use `--dbprops` with `--yaml` or `--json` to get the properties in machine-readable
format. Note that while the returned structure is generally a set of key-value
pairs, the specifics can vary significantly from one driver type to another.
Here's `--dbprops` from a [SQLite](/docs/drivers/sqlite/) database (in `--yaml` format):

![sq inspect source dbprops sqlite yaml](sq_inspect_source_dbprops_sqlite_yaml.png)

## Catalogs

The `--catalogs` mode lists the [catalogs](/docs/concepts#schema--catalog) (databases)
available in the source.

![sq inspect source catalogs pg yaml](sq_inspect_source_catalogs_yaml.png)

## Schemata

Like `--catalogs`, the `--schemata` mode lists the [schemas](/docs/concepts#schema--catalog)
available in the source.

![sq inspect source schemata pg yaml](sq_inspect_source_schemata_yaml.png)

To list the schemas in a specific catalog, supply `CATALOG.` to the
[`--src.schema`](/docs/source#source-override) flag:

```shell
# List the schemas in the "inventory" catalog.
$ sq inspect @sakila/pg12 --schemata --src.schema inventory.
````


## Inspect table

In additional to inspecting a source, you can drill down on a specific table.

```shell
$ sq inspect @sakila_pg.actor
```
![sq inspect table text](sq_inspect_table_text.png)

Use `--verbose` mode for more detail:

![sq inspect table text verbose](sq_inspect_table_text_verbose.png)

And, as you might expect, you can also see the output in `--json` and `--yaml` formats.

![sq inspect table json](sq_inspect_table_json.png)

{{< alert icon="👉" >}}
Note that the `--overview` and `--dbprops` flags apply only to inspecting sources,
not tables.
{{< /alert >}}

## Foreign-key relationships

`sq inspect` reports foreign-key constraints for any SQL source that
supports them (SQLite, Postgres, MySQL, SQL Server, Oracle, DuckDB).
The relationships appear under each table's `fk` object — the same FK
shows up once under its owning table's `fk.outgoing` and once under
the referenced table's `fk.incoming`:

- `tables[].fk.outgoing` — constraints declared on this table (its
  outgoing edges). Tells you what rows in this table depend on.
- `tables[].fk.incoming` — constraints declared on other tables whose
  referenced side is this table (its incoming edges). Tells you what
  depends on rows in this table. Useful for "blast radius" questions
  ("if I delete this row, what else breaks?") and for visualization
  tools that want to render the schema as a directed graph without
  walking every table to discover incoming edges.

Composite foreign keys are supported by every driver. Cross-schema
and cross-catalog references are reported by Postgres, MySQL, and
SQL Server. Oracle reports cross-schema references via `ref_schema`
for *outgoing* FKs only — `fk.incoming` is scoped to the current
user's schema, so FKs from tables in other schemas are not surfaced.
DuckDB's `duckdb_constraints()` view does not expose the referenced
table's schema, so a DuckDB FK that crosses schemas is reported as
same-schema.

The `on_delete` and `on_update` referential actions are surfaced
where the driver reports them (Oracle exposes `on_delete` only;
DuckDB's `duckdb_constraints()` view doesn't expose either action, so
both fields are left empty for DuckDB sources).

For example, to list every parent → child relationship in the
[Sakila](/docs/develop/sakila/) schema:

```shell
$ sq inspect -j @sakila_pg | jq -r '
  .tables[]
  | .name as $child
  | .fk.outgoing[]?
  | "\($child).\(.columns | join(",")) -> \(.ref_table).\(.ref_columns | join(","))"'
film.original_language_id -> language.language_id
film.language_id -> language.language_id
film_actor.actor_id -> actor.actor_id
film_actor.film_id -> film.film_id
...
```

### Filtering composites

Composite constraints (FK, unique, or index) are represented as a
single entry whose `columns` slice carries every participating column
in declaration order — single-column constraints are just the
one-element case. There's no separate `composite` flag because the
arity is the slice length, so `(.columns | length) > 1` is the
idiomatic filter:

```shell
# Composite foreign keys only
$ sq inspect -j @sakila_pg | jq -r '
  .tables[] | .fk.outgoing[]?
  | select((.columns | length) > 1)
  | "\(.table)(\(.columns | join(","))) -> \(.ref_table)(\(.ref_columns | join(",")))"'
```

The same pattern works for `unique_constraints[]` and `indexes[]`.

The `--verbose` text output also gains an `FK` column listing the
referenced table and columns for each FK column.

{{< alert icon="👉" >}}
ClickHouse has no foreign-key concept and so reports no FK metadata.
{{< /alert >}}

## Unique constraints and indexes

In addition to foreign keys, each table reports its UNIQUE constraints
and the physical indexes that back it:

- `tables[].unique_constraints` — UNIQUE declarations (inline or via
  `ALTER TABLE ADD CONSTRAINT`). Primary keys are reported separately
  via `columns[].primary_key` and are not repeated here. Composite
  members appear in declaration order.
- `tables[].indexes` — physical indexes, including the implicit
  PK-backing index, unique-constraint-backing indexes, and any
  user-declared `CREATE INDEX` entries. Each entry carries `unique`,
  `primary`, and a driver-specific `type` (e.g. `BTREE`, `HASH`,
  `NONCLUSTERED`). DuckDB is the exception: its `duckdb_indexes()`
  catalog only lists explicit `CREATE INDEX` definitions, so
  PK-backing and UNIQUE-backing indexes don't appear there. The PK
  and UNIQUE information is still available via
  `columns[].primary_key` and `unique_constraints`.

For example, list non-unique indexes per table:

```shell
$ sq inspect -j @sakila_pg | jq -r '
  .tables[]
  | .name as $t
  | .indexes[]?
  | select(.unique == false)
  | "\($t).\(.name) (\(.columns | join(\",\"))) [\(.type)]"'
```

### How verbose text marks redundant index entries

The `INDEXES` column of `sq inspect --verbose` text output applies
two display rules so the column reads cleanly without losing
information:

- **PK-backing indexes are dropped** entirely. The `PK` column
  already marks the participating columns; repeating the implicit
  index name adds nothing.
- **UNIQUE-constraint-backing indexes are shown muted**, with the
  index name wrapped in parentheses and rendered in a subdued style
  (italic + faint, on color terminals). The UC name also appears
  under `UNIQUE CONSTRAINTS`, so the parens signal "this is the
  implicit backing index" while keeping the name visible. The match
  is by column-set, not name, so SQLite's auto-generated
  `sqlite_autoindex_*` entries pair up with the right UC.

```text
NAME     TYPE   ROWS  COLS  NAME        TYPE     PK  FK  INDEXES                                 UNIQUE CONSTRAINTS
demo_uc  table  0     5     id          int4     pk
                            email       varchar          (demo_uc_email_key), idx_solo_unique    demo_uc_email_key
                            first_name  varchar          (uniq_full_name)                        uniq_full_name
                            last_name   varchar          (uniq_full_name)                        uniq_full_name
                            nickname    varchar          idx_demo_nickname
```

A user-declared `CREATE UNIQUE INDEX` that doesn't back any formal
constraint (e.g. `idx_solo_unique` above) renders unmuted, alongside
the parenthesized constraint-backing entry.

These display rules apply only to the verbose text renderer. The
JSON and YAML formats always emit the full `tables[].indexes` slice
— every physical index, including PK- and UC-backing ones — so
tooling consuming the machine-readable forms sees the complete
picture.

## Override active schema

By default, `sq inspect` uses the active [schema](/docs/concepts#schema--catalog)
for the source. You can override the active schema (and catalog)
using the [`--src.schema`](/docs/source#source-override) flag. See the [sources](/docs/source#source-override) section
for a fuller explanation of `--src.schema`, but here's a quick example of
inspecting Postgres's `information_schema` schema:

```shell
$ sq inspect @sakila/pg12 --src.schema sakila.information_schema
SOURCE        DRIVER    NAME    FQ NAME                    SIZE    TABLES  VIEWS  LOCATION
@sakila/pg12  postgres  sakila  sakila.information_schema  16.6MB  7       61     postgres://sakila:xxxxx@192.168.50.132/sakila

NAME                                   TYPE   ROWS   COLS
sql_features                           table  716    feature_id, feature_name, sub_feature_id, sub_feature_name, is_supported, is_verified_by, comments
sql_implementation_info                table  12     implementation_info_id, implementation_info_name, integer_value, character_value, comments
sql_languages                          table  4      sql_language_source, sql_language_year, sql_language_conformance, sql_language_integrity, sql_language_implementation, sql_language_binding_style, sql_language_programming_language
sql_packages                           table  10     feature_id, feature_name, is_supported, is_verified_by, comments
sql_parts                              table  9      feature_id, feature_name, is_supported, is_verified_by, comments
sql_sizing                             table  23     sizing_id, sizing_name, supported_value, comments
sql_sizing_profiles                    table  0      sizing_id, sizing_name, profile_id, required_value, comments
_pg_foreign_data_wrappers              view   0      oid, fdwowner, fdwoptions, foreign_data_wrapper_catalog, foreign_da
```
