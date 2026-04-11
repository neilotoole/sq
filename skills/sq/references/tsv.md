# TSV (`tsv` driver)

[Tab-separated values](https://en.wikipedia.org/wiki/Tab-separated_values). Implemented as a **CSV family** driver with tab delimiter. **Read-only** document source.

**Canonical docs:** [CSV & friends](https://sq.io/docs/drivers/csv/) (TSV is covered there alongside CSV)

## Add a source

```shell
sq add ./data.tsv
sq add --driver=tsv ./data.tsv
```

`sq` can usually [detect](https://sq.io/docs/detect/#driver-type) TSV; explicit `--driver=tsv` is equivalent to CSV with tab delimiter (see [sq.io](https://sq.io/docs/drivers/csv/)).

## Monotable

Use **`@handle.data`** like CSV.

## Delimiters

For generic delimiter control, see **`--driver.csv.delim`** on the [CSV driver page](https://sq.io/docs/drivers/csv/).
