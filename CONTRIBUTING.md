# Contributing

`sq` welcomes new [issues](https://github.com/neilotoole/sq/issues), [pull requests](https://github.com/neilotoole/sq/pulls)
and [discussion](https://github.com/neilotoole/sq/discussions).

For user documentation, see [sq.io](https://sq.io). For contributor and
maintainer reference docs (architecture, drivers, grammar, workflows,
releasing), see the [`docs/`](./docs) index at
[`docs/README.md`](./docs/README.md).

For AI coding assistants working in this repo:

- [AGENTS.md](./AGENTS.md) is the cross-agent guide (shared rules plus
  contributor [Agent Skills](https://agentskills.io/specification) under
  [`.agents/skills/`](./.agents/skills/)).
- [CLAUDE.md](./CLAUDE.md) is the Claude Code entry point and links to AGENTS.md
  for shared rules. Claude Code also discovers skills via
  [`.claude/skills`](./.claude/skills) (a symlink to `.agents/skills` when
  checked out).
- Install a skill into your agent with
  `npx skills add neilotoole/sq --skill <name>` (see
  [AGENTS.md](./AGENTS.md#installing-and-verifying-skills-npx-skills)).
- On Windows, if symlinks are unavailable, use WSL, `npx skills add`, or copy
  `.agents/skills` under `.claude/skills`.

## Documentation site (`site/`)

The [sq.io](https://sq.io) website is a [Hugo](https://gohugo.io) project in
[`site/`](./site/). From `site/`, use **`make`** for the usual workflow (`make deps`,
`make site-local`, `make site-test`, `make site-build`, or `make ci` to match CI).

**If you are changing anything under `site/`, read [`site/README.md`](./site/README.md)
first.** It is the canonical guide: Bun equivalents for the `make` targets, the
**stable** vs **full** link-check split, what PR CI blocks on, the production-publish
flow (dispatch / stable release), and the Netlify and branch-protection setup for
maintainers.

Changes under `site/` are validated by
[`site-ci.yml`](./.github/workflows/site-ci.yml); merging updates `master` only, and
live [sq.io](https://sq.io) updates on a manual **Site Publish (dispatch)** or a stable
`sq` release.

To triage or merge a batch of **Dependabot PRs** for `site/`, use the
[`sq-site-dependabot`](./.agents/skills/sq-site-dependabot/) agent skill (invoke
explicitly in your agent, e.g. `/sq-site-dependabot` in Cursor). See
[AGENTS.md](./AGENTS.md#agent-skills-contributors).

## Tooling

This documentation presumes you are on macOS. If not, adapt appropriately.
This section also presumes you want to do full-stack `sq` development; if not,
you may not need all of these tools. You'll definitely need `go`.

- `go`: `brew install go`
- `make`: `brew install make`
- `bun`: `brew install oven-sh/bun/bun` (or see [bun.sh](https://bun.sh)).
  Runs the formatting and lint tooling (`dprint`, `biome`) used by `make fmt` /
  `make lint`, and builds the [`site/`](./site/).
- `shellcheck`: `brew install shellcheck`
- `docker`: needed for the `sakiladb/*` containers used by the SQL-driver
  integration tests (`make test`); not needed for `make test-short`. See
  [Docker Desktop](https://www.docker.com).
- `java`: `brew install java`. The [antlr](https://www.antlr.org) tool that
  generates the SLQ parser is Java-based (see [`docs/GRAMMAR.md`](./docs/GRAMMAR.md)).

## General advice

`sq` is a Go project, but a complex one: generated code,
[CGo](https://go.dev/wiki/cgo) (embedded SQLite), test containers, and a docs
[website](https://sq.io). So local development goes through the
[Makefile](./Makefile) rather than raw `go` commands; `make help` (the default
target) lists every target.

After cloning, run `make init` once: it installs dependencies (`bun` packages
and Go modules) and activates the repo's git hooks, including the `pre-commit`
`dprint` formatting check (bypass a single commit with `git commit --no-verify`).
Then run `make all` as a kick-off: it generates code, formats, lints, tests,
builds, and installs a local `sq`.

For the full local development loop (the inner-loop sequence, the Makefile
targets, and how they map to CI), see [`docs/WORKFLOW.md`](./docs/WORKFLOW.md).

## Opening issues

There are already GitHub templates in place: just use the usual GitHub process
to open an [issue for `sq`](https://github.com/neilotoole/sq/issues). Remember
to search the existing issues first.

## Opening a PR

Use the usual GitHub process to open a PR. Before you do so, please:

- Name your branch per [AGENTS.md: Git branch naming](./AGENTS.md#git-branch-naming).
- Merge the latest `master` into your branch: `git merge origin/master`.
- Run `make all`.
- If the PR adds a **new driver type**, complete the
  [driver ship checklist](./docs/DRIVERS.md#driver-ship-checklist) (sq.io and `skills/sq/`).

### CI

CI is PR-centric: a branch gets CI once a pull request exists. Every push runs a
fast lint + `-short` test set (plus a Windows smoke test); the full suites run
nightly against master and on release tags. For the job-by-job breakdown, see
[`docs/WORKFLOW.md`](./docs/WORKFLOW.md#github-actions).

Mark long-running tests with [`tu.SkipShort`](./testh/tu/skip.go) so they stay out of the dev loop
but still run in the nightly/release suites.

## CHANGELOG & releasing

[CHANGELOG.md](./CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Updating it at release time is a
**maintainer** task; as a PR author you generally don't need to touch it, and **site-only**
changes never need an entry.

For the entry conventions (markers, issue references, version links, US-English style) and the
release procedure, see [`docs/RELEASING.md`](./docs/RELEASING.md).

## New driver implementations

A "driver" implements a datasource type
([Postgres](https://sq.io/docs/drivers/postgres),
[MySQL](https://sq.io/docs/drivers/mysql), [CSV](https://sq.io/docs/drivers/csv),
[JSON](https://sq.io/docs/drivers/json), etc.). The full guide (SQL vs document
drivers, package structure, type mapping, dialect configuration, test handles,
and the **driver ship checklist** for adding a new driver type) lives in
[`docs/DRIVERS.md`](./docs/DRIVERS.md).

When you add a **new driver type**, complete the
[driver ship checklist](./docs/DRIVERS.md#driver-ship-checklist) (code, sq.io
docs, and the end-user agent skill) in the same PR.
