# AGENTS.md

Guidance for AI coding assistants (Claude Code, Copilot, Cursor, Codex, etc.) and
human contributors working in this repo.

For Claude Code, [`CLAUDE.md`](./CLAUDE.md) mirrors the essentials here and links
to this file for expanded contributor content.

## About `sq`

`sq` is a command-line data wrangler providing jq-style access to structured
data sources (SQL databases like Postgres, MySQL, SQLite, SQL Server,
ClickHouse, Oracle, DuckDB; and document formats like CSV, JSON, Excel). User
docs live at
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

### English spelling

Use US English in all prose: code comments, godoc, user-facing strings,
commit messages, PR descriptions, CHANGELOG entries, and site docs. For
example, "honors" not "honours", "color" not "colour", "behavior" not
"behaviour", "optimize" not "optimise".

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
**Site-only** changes under `site/` do not require CHANGELOG entries.

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

**Adding a new driver type:** you must complete the
[driver ship checklist](./CONTRIBUTING.md#driver-ship-checklist) in the same
PR — including [`site/content/en/docs/drivers/`](site/content/en/docs/drivers/)
and [`skills/sq/`](skills/sq/SKILL.md) (`SKILL.md` driver table plus
`references/{driver}.md`). Do not mark driver work done until those files are
updated; copy an existing `skills/sq/references/*.md` as a template.

For a visual map of the driver interfaces (`driver.Driver`,
`driver.SQLDriver`, `driver.Grip`, `driver.Registry`) and how they relate to
the rest of the system, see [`ARCHITECTURE.md`](./ARCHITECTURE.md).

## Agent skills (contributors)

This repo ships [Agent Skills](https://agentskills.io/specification) for
**maintainer** workflows. They live under [`.agents/skills/`](.agents/skills/).

| Location                             | Audience                                                          |
| ------------------------------------ | ----------------------------------------------------------------- |
| [`.agents/skills/`](.agents/skills/) | Contributors (Dependabot triage, etc.)                            |
| [`skills/sq/`](skills/sq/SKILL.md)   | End users of the `sq` CLI (distribution; not repo auto-discovery) |

When you **add a new driver type**, update [`skills/sq/`](skills/sq/SKILL.md)
per the [driver ship checklist](./CONTRIBUTING.md#driver-ship-checklist): add
`references/{driver}.md` and a row in `SKILL.md`.

Claude Code discovers the same tree via [`.claude/skills`](.claude/skills)
(symlink to `.agents/skills`). Cursor and Codex load `.agents/skills/`
directly. On Windows, if symlinks are unavailable, use WSL or duplicate the
tree as documented in [`CONTRIBUTING.md`](./CONTRIBUTING.md).

### Skills in this repo

| Skill                                                        | Use when                                                              |
| ------------------------------------------------------------ | --------------------------------------------------------------------- |
| [`sq-site-dependabot`](.agents/skills/sq-site-dependabot/)   | Triaging or merging Dependabot PRs for [`site/`](site/) (Bun / Hugo). |
| [`sq-gomod-dependabot`](.agents/skills/sq-gomod-dependabot/) | Dependabot PRs for Go modules at repo root (placeholder).             |

Invoke explicitly when your agent supports it (e.g. `/sq-site-dependabot` in
Cursor, `$sq-site-dependabot` in Codex) or ask to “clear site dependabot PRs”.

Full site-dependabot workflow content lands in a follow-up PR; this scaffold
adds directories and docs only.

### Installing and verifying skills (`npx skills`)

Use the [Skills CLI](https://skills.sh/docs/cli) (no global install required):

```bash
npx skills add <owner/repo> --skill <skill-name>
```

**From this repository on GitHub** (install into your agent’s skill directories):

```bash
npx skills add neilotoole/sq --skill sq-site-dependabot
npx skills add neilotoole/sq --skill sq-gomod-dependabot
```

**From a local checkout** (verify before opening a PR that touches skills):

```bash
npx skills add . -l
npx skills add . --skill sq-site-dependabot -y
```

The `-l` / `--list` flag prints discoverable skills and descriptions without
installing. Use `-y` to skip prompts in CI or scripts.

**In-repo discovery (no install):** Cursor and Codex load
[`.agents/skills/`](.agents/skills/) from the working tree. Claude Code also
reads [`.claude/skills`](.claude/skills) when the symlink to `.agents/skills`
is present.

Optional: set `DISABLE_TELEMETRY=1` to opt out of anonymous Skills CLI
telemetry ([docs](https://skills.sh/docs/cli)).
