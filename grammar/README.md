# SLQ grammar

This directory contains the formal grammar for **SLQ**, the query language used
by [`sq`](https://sq.io):

- [`SLQ.g4`](./SLQ.g4): the grammar itself ([ANTLR4](https://www.antlr.org/)
  syntax), extensively commented.
- [`generate.sh`](./generate.sh): regenerates the Go parser into
  `libsq/ast/internal/slq/` (also runs via `go generate ./...`).
- `testdata/`: a corpus of valid SLQ snippets used as parser smoke tests.

The high-level guide — how SLQ becomes SQL, the element catalog, the lexical
layer, operator precedence, and a grammar-editing checklist — lives in
[`docs/GRAMMAR.md`](../docs/GRAMMAR.md). Read that first, then dive into
[`SLQ.g4`](./SLQ.g4) for rule-by-rule detail.

The grammar is **not yet stable**: it may change in any new `sq` release.
