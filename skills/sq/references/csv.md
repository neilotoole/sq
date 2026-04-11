# CSV (`csv` driver)

[Comma-separated values](https://en.wikipedia.org/wiki/Comma-separated_values) files. **Read-only** document source (query only; no inserts into the CSV file itself).

**Canonical docs:** [CSV & friends](https://sq.io/docs/drivers/csv/)

## Add a source

Pass the **file path** as the location to [`sq add`](https://sq.io/docs/cmd/add):

```shell
sq add ./data.csv
sq add --driver=csv ./data.csv
```

Paths are stored absolute in config. `sq` can often [detect](https://sq.io/docs/detect/#driver-type) CSV; explicit `--driver=csv` is safer.

## Monotable

Data is accessed via the synthetic **`.data`** table, e.g. `@handle.data`.

## Delimiters

Non-comma delimiters use **`--driver.csv.delim`** with aliases (`comma`, `tab`, `pipe`, etc.). See the delimiter table on [sq.io](https://sq.io/docs/drivers/csv/).

## Document source behavior

CSV is a [document source](https://sq.io/docs/source#document-source): data is **ingested** and **cached**. See alerts on the main driver page.

## Header row

Detection is automatic; override with [`--ingest.header`](https://sq.io/docs/config/#ingestheader) if needed.
