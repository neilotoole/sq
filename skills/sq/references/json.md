# JSON (`json` driver)

A single JSON file containing an **array of objects**.

**Canonical docs:** [JSON drivers](https://sq.io/docs/drivers/json/) (variants section: JSON)

## Add a source

Pass the **file path** to [`sq add`](https://sq.io/docs/cmd/add):

```shell
sq add ./data.json
sq add --driver=json ./data.json
```

## Monotable

Use **`@handle.data`**. Nested objects are **flattened** with underscore-separated field names (see [sq.io](https://sq.io/docs/drivers/json/)).

## Document source

JSON is a [document source](https://sq.io/docs/source#document-source): **ingested** and **cached**; **read-only** as a source (no inserting back into the file).

## Variants

**JSON Array** (`jsona`) and **JSON Lines** (`jsonl`) are documented under **Variants** on [JSON drivers](https://sq.io/docs/drivers/json/). The skill lists matching `references/` files in `SKILL.md` for agent-side progressive disclosure.
