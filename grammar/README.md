# SLQ Grammar

The query language used by `sq` is formally known as _SLQ_. The
grammar is defined in `SLQ.g4`, which is an [ANTLR4](https://www.antlr.org/) grammar.

There's a bunch of valid sample input in the `testdata` dir.

The `antlr4` tool generates the parser / lexer files from the grammar.
Being that `antlr4` is Java-based, Java must be installed to regenerate
from the grammar. The process is encapsulated in `generate.sh` (or execute
`go generate ./...`). 

The generated `.go` files ultimately end up in package `libsq/ast/internal/slq`. Files
in this directory should not be directly edited.

The `libsq/ast.Parse` function takes a `SLQ` input string and returns an `*ast.AST`.
The entrypoint that accepts the SLQ string is `libsq.ExecuteSLQ`, which ultimately
invokes `ast.Parse`.

## Working with the grammar

You probably should install the [antlr tools](https://github.com/antlr/antlr4-tools).

```shell
pip install antlr4-tools
```

You may also find [antlr4ts](https://github.com/tunnelvisionlabs/antlr4ts) useful.

```shell
npm install antlr4ts
```

And there are various [IDE plugins](https://github.com/antlr/antlr4-tools) available.

In particular, note the [VS Code extension](https://github.com/mike-lischke/vscode-antlr4).

