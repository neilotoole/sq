---
title: "sq"
description: "Execute SLQ query against data source"
group: query
draft: false
images: []
menu:
  docs:
    parent: "cmd"
toc: true
url: /docs/cmd/sq
---
Use the root `sq` cmd to execute queries against data sources.

## Pipe Data

For file-based sources (such as CSV or XLSX), you can [`sq add`](/docs/cmd/add) the source file,
but you can also use the UNIX pipe mechanism:

```shell
$ cat ./example.xlsx | sq .Sheet1
```

Similarly, you can [`inspect`](/docs/cmd/inspect):

```shell
$ cat ./example.xlsx | sq inspect
```

## Predefined variables

The `--arg` flag passes a value to `sq` as a [predefined variable](/docs/query/#predefined-variables):

```shell
$ sq --arg first "TOM" '.actor | where(.first_name == $first)'
actor_id  first_name  last_name  last_update
38        TOM         MCKELLEN   2020-06-11T02:50:54Z
42        TOM         MIRANDA    2020-06-11T02:50:54Z
```

## Output

`sq` can output results in many formats.

| Flag      | Shorthand | Format                                       |
|-----------|-----------|----------------------------------------------|
| `--table` | `-t`      | Text table                                   |
| `--json`  | `-j`      | JSON                                         |
| `--jsona` | `-A`      | LF-delimited JSON arrays                     |
| `--jsonl` | `-J`      | LF-delimited JSON objects                    |
| `--raw`   | `-r`      | Raw bytes, without any encoding or delimiter |
| `--xlsx`  | `-x`      | Excel XLSX                                   |
| `--csv`   | `-c`      | CSV                                          |
| `--tsv`   | `-T`      | TSV                                          |
| `--md`    |           | Markdown table                               |
| `--html`  |           | HTML table                                   |
| `--xml`   | `-X`      | XML                                          |

By default, `sq` outputs to `stdout`. You can use shell redirection to write
`sq` output to a file:

```shell
$ sq --csv .actor  > actor.csv
```

But you can also use `--output` (`-o`) to specify a file:

```shell
$ sq --csv .actor -o actor.csv
```

## Render SQL

Use `--render-sql` to print the SQL that `sq` would execute against the
target database, instead of running the SLQ query. Handy for debugging,
learning the SLQ-to-SQL mapping, or piping the rendered SQL into another
tool.

```sql
-- $ sq --render-sql '.actor | .[0:2]'
SELECT * FROM "actor" LIMIT 2 OFFSET 0
```

With `--json` or `--yaml`, `sq` prints a
structured payload containing the original SLQ, the rendered SQL, the
dialect, the sources the query touches, and any `--arg` values:

```yaml
# $ sq --render-sql --yaml --arg first TOM '.actor | where(.first_name == $first) | .[0:2]'
args:
  first: TOM
slq: .actor | where(.first_name == $first) | .[0:2]
sql: SELECT * FROM "actor" WHERE "first_name" = 'TOM' LIMIT 2 OFFSET 0
dialect: sqlite3
sources:
  target: "@sakila"
  inputs:
  - "@sakila"
```

For cross-source queries, the rendered SQL targets the synthetic join
database; `sources.target` is the synthetic handle, and `sources.inputs`
lists each user source that would be staged into it. For example, joining
the `actor` table from a Postgres Sakila against the `film_actor` table
from a MySQL Sakila:

```yaml
# $ sq --render-sql --yaml '@sakila/pg.actor | join(@sakila/my.film_actor, .actor_id) | .first_name, .last_name, .film_id | .[0:5]'
slq: "@sakila/pg.actor | join(@sakila/my.film_actor, .actor_id) | .first_name, .last_name, .film_id | .[0:5]"
sql: SELECT "first_name", "last_name", "film_id" FROM "actor" INNER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id" LIMIT 5 OFFSET 0
dialect: sqlite3
sources:
  target: "@join_xukcx3ye"
  inputs:
  - "@sakila/pg"
  - "@sakila/my"
```

`sources.target` is the synthesized SQLite join DB into which both
inputs are staged before the rendered SQL runs against it.

## Override active source

As explained in the [sources](/docs/source#active-source) section,
when you [`add`](/docs/cmd/add) your first source, it becomes the active source. That means
that queries without an explicit `@handle` are assumed to refer to the
active source. You can change the active source using [`sq src @othersrc`](/docs/cmd/src).

Sometimes you may want to override the active source just for a single query.
You can do that using the `--src` flag:

```shell
# Show the active source
$ sq src
@staging

$ sq  '.actor | count'
count
200

# Execute the same query, this time against @prod
$ sq --src @prod '.actor | count'
count
199
```

## Override active schema

Similarly, you can override the active [schema](/docs/concepts#schema--catalog)
using `--src.schema`. For example,
let's say you have a Postgres source `@customers`, with a schema for each
customer. Use `--src.schema=SCHEMA_NAME` to override the active schema:

```shell
$ sq --src.schema=acme '.orders | count'
count
200

$ sq --src.schema=momcorp '.orders | count'
count
300
```

You can use the `sq` builtin [`schema()`](/docs/query#schema) function to output the active schema:

```shell
$ sq --src.schema=acme 'schema()'
schema()
acme
```

In addition to overriding the active schema, `--src.schema` can also
be used to override the active [catalog](/docs/concepts#schema--catalog)
when used in the `catalog.schema` form.
Let's say you had another catalog (database) named `projects` on the
same Postgres cluster, and a schema named `apollo` in that catalog.
You would specify `catalog.schema` like this:

```shell
$ sq --src.schema=projects.apollo '.missions | count'
count
17
```

There's also an equivalent [`catalog()`](/docs/query#catalog) builtin function
that returns the active catalog:

```shell
$ sq --src.schema=projects.apollo 'catalog(), schema()'
catalog()  schema()
projects   apollo
```

Note that not every database implements catalog support (this includes MySQL
and SQLite). See the driver support [matrix](/docs/concepts#catalog-schema-support).

## Reference

{{< readfile file="sq.help.txt" code="true" lang="text" >}}
