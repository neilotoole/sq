# CLAUDE.md

Guidance for AI coding assistants (Claude Code, Copilot, Cursor, etc.) and
human contributors working in this repo.

## About `sq`

`sq` is a command-line data wrangler providing jq-style access to structured
data sources (SQL databases like Postgres, MySQL, SQLite, SQL Server,
ClickHouse; and document formats like CSV, JSON, Excel). User docs live at
[sq.io](https://sq.io).

## Key documents

Before making non-trivial changes, read the document most relevant to your
task:

- [`README.md`](./README.md) — project overview and user-facing intro.
- [`CONTRIBUTING.md`](./CONTRIBUTING.md) — full contributor guide: tooling,
  `Makefile` usage, driver implementation patterns, test handles,
  `CHANGELOG.md` format.
- [`ARCHITECTURE.md`](./ARCHITECTURE.md) — Mermaid ERD of core types
  (`Source`, `Driver`, `Grip`, `Registry`, `RecordWriter`, etc.).
- [sq.io](https://sq.io) — end-user documentation for commands and query
  syntax.

## Common commands

This project uses a `Makefile` as its canonical developer entry point (see
[`CONTRIBUTING.md`](./CONTRIBUTING.md#makefile) for why).

```bash
make all         # gen + fmt + lint + test + build + install
make test        # run all tests (may require Docker for SQL driver tests)
make test-short  # skip long-running / container-backed tests
make lint        # golangci-lint + shellcheck
make fmt         # goimports-reviser + gofumpt
make build       # build binary to dist/sq
```

Driver integration tests for Postgres, MySQL, SQL Server, and ClickHouse
require the `sakiladb/*` Docker images to be reachable. Use `make test-short`
or `go test -short ./...` to skip them.

## Conventions

### Go linting

Run `make lint` after any change to `*.go` files. Fix all reported issues
before committing. Common lint categories:

- `godot` — comments must end with a period.
- `gofumpt` — formatting (extra blank lines, spacing).
- `unused` — unused variables, constants, functions.

Don't wait to be asked; treat `make lint` as part of "done".

### Testing

Prefer `github.com/stretchr/testify` for assertions, and prefer `require`
over `assert`:

- `require.*` — fails fast, stopping the test on first failure. Default
  choice.
- `assert.*` — continues after failure. Use only when you genuinely want to
  report multiple independent failures in one run.

```go
import (
    "testing"

    "github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
    result := someFunction()
    require.NotNil(t, result)
    require.Equal(t, expected, result)
}
```

Integration tests that need a real database should call `tu.SkipShort(t, true)`
so they're skipped under `go test -short`. See
[`CONTRIBUTING.md`](./CONTRIBUTING.md#test-handles) for driver test handle
conventions.

### Markdown

- Wrap lines at 80 characters where feasible.
- Run `markdownlint` on any markdown file you create or modify. Fix all
  issues before committing.

```bash
markdownlint '**/*.md' --ignore node_modules
markdownlint '**/*.md' --ignore node_modules --fix
```

### `CHANGELOG.md`

See [`CONTRIBUTING.md`](./CONTRIBUTING.md#changelogmd) for the full format.
In short: work-in-progress goes under an `## Unreleased` section at the top
with `Fixed` / `Changed` / `Added` subsections, and the first reference to
an `sq` command in a release section links to its `sq.io` documentation.

### Git branch naming

Use the pattern:

```text
gh{GITHUB_ISSUE_NUMBER}-{short-description}
```

Examples: `gh531-sq-version-slow`, `gh412-add-db-filter`,
`gh503-fix-migration`.

### Commits and pull requests

See [`CONTRIBUTING.md`](./CONTRIBUTING.md#opening-a-pr) for the PR pre-flight
checklist (merge `master`, run `make all`).

Write commit messages in the imperative mood, focused on *what* changed and
*why*. Keep the subject line under ~70 characters; use the body for detail.
There's no need to add AI / Claude attribution text; this is assumed these days.

## Drivers

`sq` is driver-oriented: each supported data source type is implemented as a
driver under [`drivers/`](./drivers/). When adding or modifying a driver,
read the
["New driver implementations"](./CONTRIBUTING.md#new-driver-implementations)
section of `CONTRIBUTING.md` — it covers package structure, type mapping,
dialect configuration, test handles, and the SQL-vs-document driver split.

For a visual map of the driver interfaces (`driver.Driver`,
`driver.SQLDriver`, `driver.Grip`, `driver.Registry`) and how they relate to
the rest of the system, see [`ARCHITECTURE.md`](./ARCHITECTURE.md).
