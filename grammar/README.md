# SLQ Grammar

The query language used by `sq` is formally known as _SLQ_. The
grammar is defined in `SLQ.g4`, which is an [ANTLR4](https://www.antlr.org/) grammar.

The `antlr4` tool generates the parser / lexer files from the grammar.
Being that `antlr4` is Java-based, Java must be installed to regenerate
from the grammar. This process is encapsulated in a `mage` target:

```sh
# from SQ_PROJ_ROOT
mage generateparser
```

The generated .go files ultimately end up in package `libsq/slq`. Files
in this directory should not be directly edited.

The `libsq/ast.Parse` function takes a `SLQ` input string and returns an `*ast.AST`.
It is the `libsq.ExecuteSLQ` function that invokes `ast.Parse`.


