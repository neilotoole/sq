---
name: code-review
description: >-
  Use when reviewing a pull request or a diff in the neilotoole/sq repository.
  Carries the repo conventions a reviewer cannot infer from the diff itself:
  prose and spelling rules, the split between what lint catches and what it
  does not, test gating, generated fixtures that must not be hand edited, and
  commit and PR requirements.
license: MIT
metadata:
  homepage: https://sq.io
  version: "0.1.0"
---

# code-review

Review conventions for [`sq`](https://github.com/neilotoole/sq). The canonical
source is [`AGENTS.md`](../../../AGENTS.md); this is the reviewer-facing subset.

Spend review effort on what a general-purpose reviewer would miss. Ordinary Go
correctness, nil handling, and error wrapping are already covered without this
skill. What follows is repo knowledge that is invisible in a diff.

## Highest-value checks

| Check                       | Flag when                                                                                                                                             |
| --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Em dashes**               | A `—` or `–` appears in prose, godoc, a code comment, markdown, or a YAML comment. Use a period, comma, parentheses, or ": ". Ranges use `-` or "to". |
| **US English**              | British spelling in prose or comments: "honours", "colour", "behaviour", "optimise".                                                                  |
| **Envar**                   | "env var", "env-var" or "environment variable" in prose. The repo term is "envar".                                                                    |
| **`godot`**                 | A comment block whose last line does not end with a period.                                                                                           |
| **`require` over `assert`** | New test code uses `assert.*` without needing to report several independent failures in one run.                                                      |
| **Skipped flaky test**      | A `t.Skip` is added because a test sometimes fails. See [Flaky tests](#flaky-tests).                                                                  |
| **AI attribution**          | A commit message or PR description contains "Generated with", a co-author trailer, or any Claude / AI attribution.                                    |
| **Generated fixtures**      | `site/static/testdata/` is edited by hand. See [Generated fixtures](#generated-fixtures).                                                             |

Those first three do not apply to code itself: string literals, test fixtures
and sample data are exempt. An em dash inside a `testdata` CSV is data, not
prose.

## What lint does and does not catch

`make lint` runs golangci-lint, shellcheck, `dprint check` and biome. It does
**not** catch everything, so these need a human or an agent reviewer:

- **Import grouping.** Enforced by a separate CI step
  (`scripts/fmt-go-imports.sh`). `go build`, `go vet` and golangci-lint all
  pass on wrongly grouped imports. `make fmt` fixes it, and it must be run
  before `make lint`.
- **Prose style.** No linter checks em dashes, US spelling, or "envar".
- **Workflow formatting.** `actionlint` validates workflow syntax, not `dprint`
  style. A workflow can pass `actionlint` and still fail the `Format` CI job.
  Any touched `.yml`, `.json` or `.toml` needs `make fmt`.

## Flaky tests

Do not skip a flaky test; find the root cause and fix it. A `t.Skip` on an
intermittent failure removes the signal and leaves the cause to resurface
against whatever test loses the race next.

Gating on a real precondition is not skipping. These are correct:

- `tu.SkipShort(t, true)` for a test that needs a live database.
- `tu.SkipNoNetwork(t)` for a test that deliberately uses a real remote host.
- The envar checks behind the driver test handles.

Each states what the test requires. A skip added because a test sometimes
failed states nothing. If the cause cannot be fixed in the same change, the PR
should open an issue with the failure output rather than silence the test.

## Generated fixtures

`site/static/testdata/` is generated from the canonical in-repo fixtures by
`go run ./test/fixtures/internal/gentestdata`, and `test/fixtures` guards it
against drift. A PR that edits those files by hand, or that changes a canonical
fixture without regenerating, should be flagged.

Editing a Sakila fixture also changes documented query output, so check whether
`site/content` still matches.

## Test evidence

- A pipe masks a command's exit code. `go test ./... | tail` reports success
  even when the suite fails. Capture the status before piping.
- Server-backed driver tests skip silently when the engine's `SQ_TEST_SRC__*`
  envar is unset, so "tests pass" from a machine without the `sakiladb`
  containers proves less than it appears to.

## Other conventions

- **CHANGELOG.** Work in progress goes under `## Unreleased`. Changes confined
  to `site/` need no entry.
- **Markdown.** Wrap at 100 characters where feasible; `dprint` formats it.
- **Commit messages.** Imperative mood, subject under roughly 70 characters,
  body for the why.
- **Branch names.** `feature/`, `fix/` or `chore/`, plus `gh<ISSUE>-` when a
  GitHub issue is linked.
