# JSON Lines (`jsonl` driver)

[JSON Lines](https://jsonlines.org/): newline-delimited **JSON objects**, one per line.

**Canonical docs:** [JSON drivers — JSON Lines](https://sq.io/docs/drivers/json/) (section “JSON Lines”)

## Add a source

```shell
sq add ./data.jsonl
sq add --driver=jsonl ./data.jsonl
```

## Monotable

Use **`@handle.data`**.

## Also see

- [JSON drivers](https://sq.io/docs/drivers/json/) — all JSON variants
