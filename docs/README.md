# sq contributor docs

> [!IMPORTANT]
> This `docs/` directory documents working **on** `sq` itself (for **contributors
> and maintainers** of the codebase): architecture, drivers, grammar, development, CI,
> and such. Documentation for **using** `sq` (commands, query syntax,
> installation) lives on [sq.io](https://sq.io), whose source is under
> [`site/`](../site); see [`site/README.md`](../site/README.md).

## Contents

- [`ARCHITECTURE.md`](./ARCHITECTURE.md): Mermaid ERD of the core types
  (`Source`, `Driver`, `Grip`, `Registry`, `RecordWriter`, etc.) and how they
  fit together, plus SQL dialects, the type system, and an extension guide.
- [`DRIVERS.md`](./DRIVERS.md): driver development guide (package structure,
  type mapping, dialect configuration, test handles, the
  [driver ship checklist](./DRIVERS.md#driver-ship-checklist), and the
  SQL-vs-document driver split).
- [`GRAMMAR.md`](./GRAMMAR.md): the SLQ query language grammar (how SLQ becomes
  SQL, the element catalog, the lexical layer, operator precedence, and a
  grammar-editing checklist). Companion to
  [`grammar/SLQ.g4`](../grammar/SLQ.g4).
- [`DEVELOPER.md`](./DEVELOPER.md): the local development loop (the `Makefile`
  targets, the inner loop, git hooks, site local dev).
- [`CI.md`](./CI.md): how CI works, for contributors and maintainers (what runs
  on a PR, nightly, and on a release tag; the DB integration matrix; the
  release path; what gates a merge; changing workflows safely).
- [`SAKILA.md`](./SAKILA.md): the Sakila test dataset (the
  [`sakiladb`](https://github.com/sakiladb) Docker images, embedded vs external
  sources, the engine matrix, and how Sakila is used across the repo).
- [`RELEASING.md`](./RELEASING.md): `CHANGELOG.md` entry conventions and the
  release procedure (tag → GoReleaser → publish).

## See also

- [`../README.md`](../README.md): project overview.
- [`../CONTRIBUTING.md`](../CONTRIBUTING.md): contributor guide (tooling,
  `Makefile`, CI, `CHANGELOG.md` format).
- [`../AGENTS.md`](../AGENTS.md): conventions for AI coding assistants.
- [`../drivers/README.md`](../drivers/README.md): the `drivers/` directory
  orientation stub.
- [`../site/README.md`](../site/README.md): the [sq.io](https://sq.io) website
  source (`site/`): Hugo/Bun tooling, local dev, and Netlify CI/publish.
