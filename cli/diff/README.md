# `sq diff` internals

Contributor notes for the `cli/diff` package, which implements
[`sq diff`](https://sq.io/docs/diff). This covers how the diff works and why it is designed the
way it is. For usage, see the [user docs](https://sq.io/docs/diff).

`sq diff` is marked beta. Expect the details here to change.

## What gets compared

`sq diff` compares two sources (`@a @b`) or two tables (`@a.t1 @b.t2`). The mode flags select
what is compared:

| Mode         | Compares                              | Implemented in |
| ------------ | ------------------------------------- | -------------- |
| `--overview` | source metadata, without schema       | `overview.go`  |
| `--dbprops`  | database/server properties            | `dbprops.go`   |
| `--schema`   | table structure, optionally row count | `schema.go`    |
| `--data`     | row values                            | `tabledata.go` |

The metadata modes render each side to YAML (`render.go`) and diff the text. `--data` is
different: it streams rows from both tables and compares them record by record. Most of the
complexity, and the rest of this document, is about `--data`.

`exec_source.go` and `exec_table.go` are the entry points for source-level and table-level diffs.
A source-level `--data` diff compares every table in the union of both sources' table names, so a
table that exists on only one side is diffed against a missing table (see
[Invariants](#invariants)).

## The `--data` pipeline

A table data diff (`diffTableData`) runs as a set of goroutines connected by channels:

```text
query td1 ──> rs1.recCh ─┐
                         ├─> collation ──> recPairsCh ──> recordDiffer.exec ──> RecordHunkWriter
query td2 ──> rs2.recCh ─┘     (pairs)                      (hunks)              (text, csv, ...)
```

1. **Queries.** Each table is read with an SLQ query built by `dataQuery`. Each query writes
   records into a `dbResults`, a minimal `libsq.RecordWriter`.
2. **Collation.** One goroutine reads both record streams and pairs them up into `record.Pair`
   values. A pair with a nil side means a row exists in only one table. This is the only place
   that decides which row is compared with which.
3. **Hunks.** `recordDiffer.exec` groups differing pairs, plus context pairs, into hunks. It keeps
   a small trailing buffer of pairs for the context lines before a difference.
4. **Rendering.** A `RecordHunkWriter` renders each hunk in the output format. The text and csv
   formats have native writers (`cli/output/tablew/tablewdiff.go`, `cli/output/csvw/csvdiff.go`)
   that walk the pairs directly. Other formats (json, yaml, markdown, ...) go through
   `recordHunkWriterAdapter`, which renders each side with the format's ordinary record writer and
   diffs the resulting text.

Nothing in the pipeline holds a whole table in memory. Memory is bounded by the channel buffers
(`tuning.record-buffer`) and the size of the largest hunk (`diff.max-hunk-size`).

## Row matching

How rows are paired decides whether the diff is useful. There are two strategies, chosen per
table pair before the queries start.

### Key-aware matching

When both tables have a usable primary key, both queries are ordered by the key
(`dataQuery` appends `| order_by(."pk")`), and `mergeRecordsByKey` merges the two ordered streams
the way a merge join does, with one record of lookahead per side:

- key only in the left table: removed (`-`)
- key only in the right table: added (`+`)
- key in both, values differ: changed (`-old`, `+new`)
- key in both, values equal: unchanged (context only)

The eligibility check is `pkMergeKey`. A primary key is usable only if:

- both tables have one, with the same column names in the same order;
- every key column is an integer (`kind.Int`);
- no key column is nullable.

The integer rule exists because the merge is only correct if the database's `ORDER BY` order
matches the order `compareIntKey` uses in process. Integer ordering is the same on every
database. String ordering isn't: it depends on the database's collation, and often on the
column's. Supporting string keys means solving that first.

The nullable rule exists because a NULL key can't be compared as an integer, and databases
disagree on where NULLs sort. Most databases don't allow NULL in a primary key, but SQLite does
for any key column that isn't the rowid alias, such as `id INT PRIMARY KEY`. For this to work,
drivers must report nullability accurately: the SQLite and rqlite drivers report an
`INTEGER PRIMARY KEY` rowid alias as not nullable, even without an explicit `NOT NULL`.
`TestDiff_Data_PKAlignAllDrivers` checks that key matching is used for each SQL driver.

### Positional matching

Without a usable key, `collatePositional` pairs row 1 with row 1, row 2 with row 2, and so on, in
whatever order the database returns them. This works when the tables differ only in place. A
single inserted or deleted row, though, puts every following row out of step, and the diff
reports all of them. For example, with ids `1,3,4,6` on the left and `1,2,3,4,5,6` on the right,
positional matching compares `3` with `2`, `4` with `3`, and so on to the end of the table.
Key-aware matching reports exactly two added rows.

This cascade was the original problem in
[#947](https://github.com/neilotoole/sq/issues/947): two copies of a MySQL table that differed by
5 rows produced a diff of over 31,000 lines. When the diff falls back to positional matching, it
says so with a note at the end of the output, so a cascade is never mistaken for real
differences.

### Prior art

Every database diff tool surveyed matches rows by key. Only generic text diffs compare by
position.

| Tool                                                                                                                                               | Matches rows by                                          | Without a key           |
| -------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------- | ----------------------- |
| [SQLite `sqldiff`](https://sqlite.org/sqldiff.html)                                                                                                | rowid, or the declared primary key with `--primarykey`   | uses rowid              |
| [SQL Server `tablediff`](https://learn.microsoft.com/en-us/sql/tools/tablediff-utility)                                                            | primary key, identity or ROWGUID column                  | refuses                 |
| [Redgate SQL Data Compare](https://documentation.red-gate.com/sdc/troubleshooting/unexpected-behavior-technical-questions/what-s-a-comparison-key) | primary key, unique index or constraint, or a custom key | can't compare the table |
| [Datafold `data-diff`](https://docs.datafold.com/data-diff/how-datafold-diffs-data)                                                                | a key given with `-k`, which needn't be declared         | requires a key          |
| [Dolt](https://docs.dolthub.com/concepts/dolt/sql/primary-key)                                                                                     | primary key                                              | adds and deletes only   |
| daff                                                                                                                                               | key columns given with `--id`                            | heuristic               |

Dolt's handling of keyless tables is described in
[this post](https://www.dolthub.com/blog/2020-12-21-keyless-tables): with no row identity, every
change is an add or a delete. That is a better fallback than positional matching, and a candidate
for future work.

## Invariants

Changes to this package must preserve these:

- **No whole-table buffering.** Rows are streamed, and memory is bounded by the hunk size
  (`diff.max-hunk-size`). An earlier version ran out of memory on large tables
  ([#353](https://github.com/neilotoole/sq/issues/353)).
- **`--stop` is never silent.** `--stop N` stops after N differences, plus their trailing context.
  Both collation paths then keep reading, without emitting, until they find another difference or
  reach the end. Only if another difference exists do they stop the queries (via
  `dbCancel(errz.ErrStop)`), and only then is the `stopped after N differences; more exist`
  trailer printed. The trailer must never claim a truncation that didn't happen.
- **A missing table is empty.** If a table doesn't exist on one side, the query engine never
  calls `dbResults.Open`, so the query goroutine calls `signalNotExist` to signal end-of-data.
  Without it, the collation would block forever waiting for records.
- **Valid unified-diff headers.** The native writers build `@@ -l,s +l,s @@` headers with
  `diffdoc.HunkRange`, using separate counts for each side and GNU diff's conventions, including
  the start line of an empty range. `TestDiff_Data_HunkHeaderCounts` checks them against GNU diff
  output.
- **Errors seal the document.** On error, the collation sets the cause before closing
  `recPairsCh`, so the consumer can't seal the diff document with a nil error while the real
  error is still in flight.
- **One `QueryContext` per query.** The two table queries run concurrently, and preparing a query
  mutates its `libsq.QueryContext`, so each query must get its own. Run `go test -race` on this
  package after changing the pipeline: CI doesn't run the race detector.

## Known limitations

Follow-up work is tracked in [#1390](https://github.com/neilotoole/sq/issues/1390). In brief:

- Only integer keys are used for key-aware matching. Other key types fall back to positional.
- Key column names must match exactly, so Oracle (which reports upper-case names) falls back
  when compared with another database.
- ClickHouse doesn't report a primary key, so its tables always fall back.
- The left start line in `@@` headers drifts after the first hunk once the two sides have
  different row counts. The diff body is correct.
- Formats that go through the adapter (json, yaml, markdown) re-diff rendered text, so
  pretty-printed multi-line rows can still cascade.
- MySQL `BIGINT UNSIGNED` values of 2⁶³ or more are compared incorrectly.

## Tests

- `keymerge_internal_test.go`: eligibility, the streaming merge, key comparison and query
  building.
- `tabledata_internal_test.go`: hunk grouping, the stop trailer, the fallback note and collation
  edge cases, using synthetic record streams.
- `diff_test.go`: end-to-end runs of `sq diff` against SQLite files, plus
  `TestDiff_Data_PKAlignAllDrivers`, which runs against every SQL driver's test database.
