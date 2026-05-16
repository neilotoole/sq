---
title: "sq explore"
description: "Inspect data source schema interactively (TUI)"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2065
toc: false
url: /docs/cmd/explore
---
`sq explore` opens an interactive terminal UI that browses the schema,
columns, indexes, foreign keys, and a preview of rows for a data source.
It works against every source type `sq` supports, including SQL databases
(Postgres, MySQL, SQLite, SQL Server, Oracle, ClickHouse, DuckDB) and
document sources (CSV/TSV, JSON, Excel).

`sq explore` is a companion to [`sq inspect`](/docs/cmd/inspect): it surfaces
the same metadata interactively, with lazy-loaded per-table detail and
keyboard-driven navigation.

## Keys

| Key | Action |
| --- | --- |
| `j` / `k` / `↓` / `↑` | Move selection |
| `h` / `l` / `←` / `→` | Move pane focus |
| `tab` / `shift+tab` | Cycle pane focus |
| `enter` | Open the selection in the detail pane |
| `space` | Expand / collapse a tree node |
| `/` | Filter the focused pane |
| `r` | (Re)fetch preview rows |
| `R` | Refetch metadata (bypass cache) |
| `y` | Copy current handle to clipboard |
| `q` / `ctrl+c` | Quit |

The full key reference is always shown across the top of the TUI.

## Shell composition

`--emit-handle` (or `-q`) writes the last-focused handle to stdout on quit,
so you can navigate to a table and then run a downstream query in one shot:

```bash
sq $(sq explore -q @sakila) --csv > rows.csv
```

## When the TUI doesn't start

The TUI refuses to start when stdout is not a TTY (for example, piped to a
file or another command). In that case `sq explore` prints the source
overview instead — the same output as `sq inspect --overview`. You can
also force this behavior on a TTY with `--no-tui`.

## Reference

{{< readfile file="explore.help.txt" code="true" lang="text" >}}
