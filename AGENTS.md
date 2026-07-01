# AGENTS.md

Guidance for AI coding assistants (Claude Code, Copilot, Cursor, Codex, etc.) and
human contributors working in this repo.

[`CLAUDE.md`](./CLAUDE.md) is the Claude Code entry point; it points here for
all shared rules.

> [!IMPORTANT]
> Before making a significant change, review [`docs/README.md`](./docs/README.md)
> and the contributor docs it links to (architecture, drivers, grammar, workflows,
> Sakila, releasing). Read the ones relevant to your task before you start, and
> if your change makes any of them inaccurate, update them in the same change.

## About `sq`

`sq` is a command-line data wrangler providing jq-style access to structured
data sources (SQL databases like Postgres, MySQL, SQLite, SQL Server,
ClickHouse, Oracle, DuckDB; and document formats like CSV, JSON, Excel). User
docs live at
[sq.io](https://sq.io).

## Key documents

Before making non-trivial changes, read the document most relevant to your
task:

- [`README.md`](./README.md): project overview and user-facing intro.
- [`CONTRIBUTING.md`](./CONTRIBUTING.md): full contributor guide (tooling,
  `Makefile` usage, driver implementation patterns, test handles,
  `CHANGELOG.md` format).
- [`docs/README.md`](./docs/README.md): index of the contributor docs under
  [`docs/`](./docs).
- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md): Mermaid ERD of core types
  (`Source`, `Driver`, `Grip`, `Registry`, `RecordWriter`, etc.).
- [`docs/DRIVERS.md`](./docs/DRIVERS.md): driver development guide (package
  structure, type mapping, the driver ship checklist).
- [`docs/GRAMMAR.md`](./docs/GRAMMAR.md): SLQ query language grammar guide
  (companion to [`grammar/SLQ.g4`](./grammar/SLQ.g4)).
- [`docs/WORKFLOW.md`](./docs/WORKFLOW.md): development and CI workflows (local
  `Makefile` loop and the GitHub Actions pipelines).
- [`docs/SAKILA.md`](./docs/SAKILA.md): the Sakila test dataset (`sakiladb`
  images, embedded vs external sources, engine matrix, repo-wide usage).
- [sq.io](https://sq.io): end-user documentation for commands and query
  syntax.

## Common commands

This project uses a `Makefile` as its canonical developer entry point (see
[`CONTRIBUTING.md`](./CONTRIBUTING.md#general-advice) for why).

`make help` (the default target) lists every target with a one-line
description; each target is documented inline in the `Makefile`.

```bash
make help        # list all targets with descriptions (default target)
make init        # one-time clone setup: install deps + activate git hooks
make deps        # install dev deps (bun packages + Go modules)
make all         # gen + fmt + lint + test + build + install
make test        # run all tests (may require Docker for SQL driver tests)
make test-short  # skip long-running / container-backed tests
make fmt         # goimports-reviser (Go imports) + dprint fmt (everything else)
make fmt-check   # dprint check (read-only; verify formatting)
make lint        # golangci-lint + shellcheck + dprint check + biome (site JS)
make build       # build binary to dist/sq
```

Run `make init` once after cloning: it installs dependencies and activates the
repo's git hooks (`.githooks`), including a `pre-commit` hook that runs
`dprint check` on staged files so a formatting slip is caught locally instead
of failing the `Format` CI job. Bypass the hook for one commit with
`git commit --no-verify`.

Driver integration tests for Postgres, MySQL, SQL Server, and ClickHouse
require the `sakiladb/*` Docker images to be reachable. Use `make test-short`
or `go test -short ./...` to skip them.

## Conventions

### Go linting

Run `make lint` after any change to `*.go` files. Fix all reported issues
before committing. Common lint categories:

- `godot`: comments must end with a period.
- `unused`: unused variables, constants, functions.

Go formatting (gofumpt rules + import ordering) is handled by `make fmt`, not
golangci-lint: `dprint` runs the gofumpt plugin (`modulePath` + `extraRules`)
and `goimports-reviser` orders imports. Run `make fmt` before `make lint`.

Don't wait to be asked; treat `make lint` as part of "done".

### Testing

Prefer `github.com/stretchr/testify` for assertions, and prefer `require`
over `assert`:

- `require.*`: fails fast, stopping the test on first failure. Default
  choice.
- `assert.*`: continues after failure. Use only when you genuinely want to
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
[`docs/DRIVERS.md`](./docs/DRIVERS.md#test-handles) for driver test handle
conventions.

### Error handling

Use [`libsq/core/errz`](./libsq/core/errz) for every error produced inside
sq. `errz` wrappers attach a stack trace at the call site, which the CLI's
debug output and `%+v` formatting rely on (errz errors implement
`fmt.Formatter`). `errors.Is` / `errors.As` continue to work because `errz`
exposes `Unwrap`.

| Situation                                         | Use                            |
| ------------------------------------------------- | ------------------------------ |
| Brand-new error, plain string                     | `errz.New("...")`              |
| Brand-new error, formatted                        | `errz.Errorf("... %s", x)`     |
| Wrap an existing error                            | `errz.Wrap(err, "context")`    |
| Wrap an existing error, formatted                 | `errz.Wrapf(err, "fmt %s", x)` |
| Annotate an external error with no extra context  | `errz.Err(err)`                |
| Package-level sentinel for `errors.Is` comparison | `errors.New("...")`            |

Sentinels (e.g. `secret.ErrNotFound`, `errz.ErrStop`) intentionally stay on
`errors.New`. Wrapping at package-init would attach a stack trace from
program startup, which is meaningless. Wrap them with `errz` at the point
they're returned.

Errors crossing into sq from external libraries (stdlib, third-party
packages) MUST be wrapped at the boundary so the stack trace anchors at the
sq-side caller, not deep inside the external code.

Avoid `fmt.Errorf` and `errors.New` outside sentinel declarations: they
produce stackless errors that surface upstream with no useful trace.

Command-authoring conventions for the `cli` package (including the rule that
every argument-taking command must offer shell completion) live in
[`cli/CLAUDE.md`](./cli/CLAUDE.md).

### Passing data: prefer explicit signatures over `context.Value`

Don't smuggle values through `context.Context` to avoid a signature or
interface change. `context.Value` is for request-scoped metadata
(cancellation, deadlines, trace/logging IDs), not for data a function needs
to compute its result. If a value materially affects what a function returns
or does, thread it as an explicit parameter (or field), even when that means
changing an exported signature or a driver-interface method. sq is pre-v1.0.0,
so such changes are acceptable and expected; reach for the cleaner API rather
than working around the old one.

If a `context.Value`-based approach genuinely seems warranted (it rarely is),
stop and ask before taking that path.

For a worked example of this rule applied, see `SQLDriver.RecordMeta`: its
result-column kind hints are threaded as an explicit parameter rather than
smuggled through `context.Value` (resolved in
[#848](https://github.com/neilotoole/sq/issues/848)).

### English spelling

Use US English in all prose: code comments, godoc, user-facing strings,
commit messages, PR descriptions, CHANGELOG entries, and site docs. For
example, "honors" not "honours", "color" not "colour", "behavior" not
"behaviour", "optimize" not "optimise".

### Prose style (no AI-isms)

Applies to all written content in this repo: `README.md`, `CHANGELOG.md`,
other root-level markdown, godoc and code comments, commit messages, PR
descriptions, and everything under [`site/`](./site/). Does **not** apply to
code itself (string literals, test fixtures, sample data).

Write like a human contributor, not a generative model. Specifically, avoid:

- **Em dashes** (`—`). Use a period, comma, parentheses, or ": ".
  No en dashes (`–`) for ranges either; use `-` or "to".
- **"Not just X, it's Y"** / "not only X but Y" sloganeering and other
  marketing-style antitheses.
- **Decorative emoji** in prose. GitHub callout admonitions
  (`> [!NOTE]` etc.) are fine when they convey real information.

When in doubt: shorter, plainer, more specific. Concrete nouns and verbs
beat adjectives.

### Markdown

- Wrap lines at 100 characters where feasible.
- Markdown is formatted by `dprint` (the `dprint-plugin-markdown` plugin), the
  same tool that formats the rest of the repo. Format any file you touch and
  verify before committing.

```bash
make fmt        # format everything (markdown included) via dprint
make fmt-check  # verify formatting (read-only)
```

This covers all markdown in the repo (root docs, `skills/`, and `site/`) under
the single root [`dprint.json`](./dprint.json). There is no separate markdown
linter or per-directory config anymore.

### GitHub Actions workflows

`dprint` also formats workflow YAML (`.github/workflows/*.yml`), JSON, and TOML
via the same `make fmt` / `make fmt-check` as everything else. The
`Format` CI job runs `dprint check` repo-wide and fails on any unformatted file.
`actionlint` validates workflow _syntax_, not dprint _style_, so a workflow can
pass `actionlint` and still fail `Format`. Run `make fmt` on any workflow (or
JSON) file you touch before committing.

### `CHANGELOG.md`

See [`docs/RELEASING.md`](./docs/RELEASING.md) for the full format.
In short: work-in-progress goes under an `## Unreleased` section at the top
with `Fixed` / `Changed` / `Added` subsections, and the first reference to
an `sq` command in a release section links to its `sq.io` documentation.
**Site-only** changes under `site/` do not require CHANGELOG entries.

### Git branch naming

Choose a prefix by change type:

- `feature/`: new capability or enhancement
- `fix/`: bug fix
- `chore/`: maintenance, deps, CI, tooling, docs-only housekeeping

No linked GitHub issue:

```text
{prefix}{short-kebab-description}
```

Linked GitHub issue:

```text
{prefix}gh{ISSUE_NUMBER}-{short-kebab-description}
```

Examples: `feature/upgrade-gofumpt`, `fix/gh914-nightly-link-check`,
`chore/gh928-update-gofumpt`.

### Commits and pull requests

See [`CONTRIBUTING.md`](./CONTRIBUTING.md#opening-a-pr) for the PR pre-flight
checklist (merge `master`, run `make all`).

Write commit messages in the imperative mood, focused on _what_ changed and
_why_. Keep the subject line under ~70 characters; use the body for detail.
Do not add AI / Claude attribution text to commits or PRs.

## Drivers

`sq` is driver-oriented: each supported data source type is implemented as a
driver under [`drivers/`](./drivers/). When adding or modifying a driver,
read the
["New driver implementations"](./docs/DRIVERS.md#new-driver-implementations)
guide in [`docs/DRIVERS.md`](./docs/DRIVERS.md). It covers package structure,
type mapping, dialect configuration, test handles, and the SQL-vs-document
driver split.

**Adding a new driver type:** you must complete the
[driver ship checklist](./docs/DRIVERS.md#driver-ship-checklist) in the same
PR, including [`site/content/en/docs/drivers/`](site/content/en/docs/drivers/)
and [`skills/sq/`](skills/sq/SKILL.md) (`SKILL.md` driver table plus
`references/{driver}.md`). Do not mark driver work done until those files are
updated; copy an existing `skills/sq/references/*.md` as a template.

For a visual map of the driver interfaces (`driver.Driver`,
`driver.SQLDriver`, `driver.Grip`, `driver.Registry`) and how they relate to
the rest of the system, see [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md).

## Agent skills (contributors)

This repo ships [Agent Skills](https://agentskills.io/specification) for
**maintainer** workflows. They live under [`.agents/skills/`](.agents/skills/).

| Location                             | Audience                                                          |
| ------------------------------------ | ----------------------------------------------------------------- |
| [`.agents/skills/`](.agents/skills/) | Contributors (Dependabot triage, etc.)                            |
| [`skills/sq/`](skills/sq/SKILL.md)   | End users of the `sq` CLI (distribution; not repo auto-discovery) |

Adding a new driver type also requires updating
[`skills/sq/`](skills/sq/SKILL.md); see [Drivers](#drivers) and the
[driver ship checklist](./docs/DRIVERS.md#driver-ship-checklist).

Claude Code discovers the same tree via [`.claude/skills`](.claude/skills)
(symlink to `.agents/skills`). Cursor and Codex load `.agents/skills/`
directly. On Windows, if symlinks are unavailable, use WSL or duplicate the
tree as documented in [`CONTRIBUTING.md`](./CONTRIBUTING.md).

### Skills in this repo

| Skill                                                        | Use when                                                              |
| ------------------------------------------------------------ | --------------------------------------------------------------------- |
| [`sq-site-dependabot`](.agents/skills/sq-site-dependabot/)   | Triaging or merging Dependabot PRs for [`site/`](site/) (Bun / Hugo). |
| [`sq-gomod-dependabot`](.agents/skills/sq-gomod-dependabot/) | Dependabot PRs for Go modules (`go.mod`/`go.sum`) at repo root.       |

Invoke explicitly when your agent supports it (e.g. `/sq-site-dependabot` in
Cursor, `$sq-site-dependabot` in Codex) or ask to “clear site dependabot PRs”.

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
