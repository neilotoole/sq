# JSON Array (`jsona` driver)

Newline-delimited lines where each line is a **JSON array** (JSON Array / `jsona`).

**Canonical docs:** [JSON drivers — JSON Array](https://sq.io/docs/drivers/json/) (section “JSON Array”)

## Add a source

```shell
sq add --driver=jsona ./data.jsona
```

`sq` can often infer `json` vs `jsona` vs `jsonl` from the file; set **`--driver=jsona`** when you need to force this variant.

## Monotable

Use **`@handle.data`**. Same flattening behavior as other JSON drivers where applicable; see [JSON driver](https://sq.io/docs/drivers/json/).

## Also see

- [JSON drivers](https://sq.io/docs/drivers/json/) — full variant comparison
- JSON object files (`json`) and JSON Lines (`jsonl`) on the same documentation page
