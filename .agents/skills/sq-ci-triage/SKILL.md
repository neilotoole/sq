---
name: sq-ci-triage
description: >-
  Use when CI is red on neilotoole/sq and you intend to act on it: an automated
  autofix pass, a nightly or dispatch failure, or a failing check on a pull
  request. Carries the triage order, what counts as proof that a fix works, and
  the rules that keep an attempted fix from landing as a duplicate or
  unmergeable PR.
license: MIT
compatibility: >-
  Requires gh CLI (authenticated) and network access to GitHub. Reproducing a Go
  failure locally needs the Go toolchain; the client/server driver legs need the
  sakiladb containers, or a workflow dispatch instead.
metadata:
  author: Todd Papaioannou
  homepage: https://sq.io
  version: "0.1.0"
---

# sq-ci-triage

Triage procedure for a red CI run on [`sq`](https://github.com/neilotoole/sq). Policy lives in
[`AGENTS.md`](../../../AGENTS.md), especially [Flaky tests](../../../AGENTS.md#flaky-tests) and
[Git branch naming](../../../AGENTS.md#git-branch-naming). How CI is wired is in
[`docs/CI.md`](../../../docs/CI.md). This skill is the operational layer: what to check, in what
order, and what to do with the answer.

It exists because of a measured failure mode. Between 2026-06-21 and 2026-09-16, thirteen
autofix PRs were opened against this repo and none merged.
[`references/case-file.md`](references/case-file.md) records each one and why it was closed. Four
causes account for all thirteen: the fix was a `t.Skip` on a flake with a real root cause, the
flake was already tracked or already being fixed, the diff carried work that belonged to another
branch, or the PR's own green checks never ran the job that had failed.

Two rules follow from that, and they outrank convenience:

1. **Never silence a failing test.** Find the mechanism that makes it fail. The cause outlives the
   symptom, and it resurfaces against whatever test loses the race next.
2. **Reporting is a real outcome.** A good issue beats a speculative PR. Opening nothing beats
   opening a PR you cannot defend.

## Order of work

Do these in order. Every step before "Classify" is a possible stop, and stopping early is cheap.

| Step                           | Question                                            |
| ------------------------------ | --------------------------------------------------- |
| 1. [Symptom](#1-symptom)       | What exactly failed, in which job, with what error? |
| 2. [Prior art](#2-prior-art)   | Does someone already own this?                      |
| 3. [Provenance](#3-provenance) | Which commit and branch was under test?             |
| 4. [Classify](#4-classify)     | Regression, flake, or infrastructure?               |
| 5. [Proof](#5-proof)           | Can you reproduce it and demonstrate the fix?       |
| 6. [Publish](#6-publish)       | Issue, comment, or PR, and against which base?      |
| 7. [Clean up](#7-clean-up)     | Does anything of yours need closing or deleting?    |

## 1. Symptom

Read the log. Not the check name, the log.

```bash
gh run view <run-id>                       # job list and conclusions
gh run view --job <job-id> --log-failed    # the failing step's output
```

Write down three things before going further:

- The failing job name, exactly (`test-windows-full`, `test-nix (macos-15)`, `run / test oracle:latest`).
- The failing test's full name including subtest path, or the workflow step name when no test is
  involved.
- The verbatim error line. The mechanism is usually named in it, and it is usually unrelated to
  what the test name suggests. `The process cannot access the file because it is being used by
  another process` is a Windows sharing violation, whatever test it lands on.

One symptom per pass. If several tests failed, take the first alphabetically and leave the rest.
Multiple symptoms in one PR cannot be reviewed or reverted independently.

## 2. Prior art

Search before you think about a fix. Every recent flake on this repo already had an issue, an open
PR, or a prior attempt by the same automation, and the search that finds it takes seconds.

```bash
gh search issues --repo neilotoole/sq --include-prs --limit 20 'TestStartMemStatsTracker'
gh search issues --repo neilotoole/sq --include-prs --limit 20 'cmd_sql_test.go'
gh search issues --repo neilotoole/sq --include-prs --limit 20 'sakila.duckdb'
gh pr list --repo neilotoole/sq --state open --limit 40 \
  --json number,title,headRefName,files
```

Search by identifier, not by prose, and run more than one search. Error text does not match: a
search for `sakila.duckdb sharing violation` returns nothing. Identifiers do, but which one hits
varies. At the moment PR #1200 was opened, its own test name returned nothing that owned the
failure, while `sakila.duckdb` and `cmd_sql_test.go` each returned the owning issue #1198 and the
merged fix #1199. Search the test name, the failing file's path, the fixture filename, and the
symbol named in the error.

Check all four:

- An **open or recently closed issue** describing the symptom. If it exists, it owns the problem.
- An **open PR** touching the files the failure implicates, including the test harness
  (`testh/`, `testh/tu/`) rather than only the failing test. A harness fix in flight will resolve
  failures in tests that look unrelated to it.
- A **merged PR from the last few days** that already fixed it. A nightly run can be older than
  its own fix, and a failure on a stale head is not a failure.
- Your **own open and recent PRs**. Three separate PRs went after the same downloader flake
  because nobody looked.

If any of those own the symptom, stop. Post a comment only if you carry a genuinely new data
point, such as a run link for an occurrence on a platform not yet listed, and never repeat what
the thread already says.

Count your own open PRs before continuing. Three or more open means stop and do
[Clean up](#7-clean-up) instead: on 2026-09-15 seven autofix PRs were open at once, and all seven
were closed unmerged.

## 3. Provenance

Establish what was actually tested, because it decides where a fix can go.

```bash
gh run view <run-id> --json headBranch,headSha,event,createdAt
```

- **Failure on `master` or a nightly or dispatch run of `master`.** The base for any fix is
  `master`.
- **Failure on a pull request's branch.** The problem belongs to that PR. Push to that branch if
  you can, or comment on the PR with your findings. Do not open a competing PR against `master`.
  That was the single largest source of unmergeable diffs here: PR #1197 was 517 added lines
  against `master` when the change it intended was one line, because it branched from the head of
  another open PR and carried that PR's work along with it.
- **Failure on a commit that is no longer the head.** Re-check on current head before doing
  anything.

Branch from the exact tested commit, never from another feature branch:

```bash
git fetch origin <headSha> && git checkout -b fix/gh<issue>-<short-desc> <headSha>
```

## 4. Classify

| Evidence                                                                                                              | Outcome                                                           |
| --------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| Fails every time on the tested commit, passes on its parent                                                           | Regression. Fix it, on the branch that introduced it.             |
| Intermittent, and you can state the mechanism and show it (what raced, what held the lock, what was nondeterministic) | Fix the mechanism.                                                |
| Intermittent, mechanism not established                                                                               | No code PR. File an issue, or add a run link to the existing one. |
| Runner outage, registry 5xx, expired credential, dead external host, container image unavailable                      | No code PR. Report, and say which of those it is.                 |
| Already owned per [Prior art](#2-prior-art)                                                                           | Stop.                                                             |

"Intermittent with the mechanism established" is a narrow category. It means an explanation that
predicts the observed pattern: why this OS, why this pair of tests, why now. If your explanation
does not predict which runs fail, it is a guess, and a guess belongs in an issue.

### Never silence a failure

These are not fixes, and a PR containing one will be closed:

- `t.Skip`, `t.Skipf`, `tu.SkipWindows`, `tu.SkipIf` or any conditional skip added because a test
  sometimes fails.
- Deleting or commenting out a test, a subtest, or an assertion.
- Raising a timeout, loosening a threshold, or adding retries so a flake surfaces less often.
- Moving a test behind an envar that no CI leg sets, which is a skip with extra steps.
- Excluding a job or a package from a workflow.

Gating on a real precondition is different, and these are correct because each one states what the
test requires:

| Gate                               | States                                    |
| ---------------------------------- | ----------------------------------------- |
| `tu.SkipShort(t, true)`            | Needs a live database or is long-running. |
| `tu.SkipNoNetwork(t)`              | Deliberately uses a real remote host.     |
| `tu.SkipReadOnlyFileUnenforceable` | The guard is a no-op on this platform.    |
| Driver `SQ_TEST_SRC__*` checks     | Needs that engine's container.            |

`tu.SkipIssue` and `tu.SkipIssueWindows` do exist for a tracked, accepted skip. They are a
maintainer's call, not yours. Propose one on the issue and wait; do not open the PR.

## 5. Proof

Reproduce before fixing, and demonstrate after. If you cannot do the first, you are reporting, not
fixing.

```bash
go test ./path/to/pkg -run '^TestName$' -count=1 -v
go test ./path/to/pkg -run '^TestName$' -count=20   # for an intermittent failure
go test ./... -short                                # the PR dev loop
```

A pipe hides the exit code: `go test ./... | tail` reports success on a failing suite. Capture the
status first. Server-backed driver tests skip silently without their `SQ_TEST_SRC__*` envar, so
"passes locally" can mean "never ran".

### Green PR checks are not proof

A pull request runs the fast loop only: `lint`, `test-nix` with `-short` on Linux and macOS,
`test-windows-smoke`, `format`, dependency review, plus the touched driver's bookend legs. These do
**not** run on a PR:

- `test-windows-full`, the full Windows suite, which is where most of this repo's flakes appear.
- The DB matrix beyond the touched driver's bookends.
- `Coverage`, `CodeQL`, and the nightly live-network test (`SQ_TEST_NETWORK`, gh #1158).

PRs #1197 and #1200 both showed every check green while `test-windows-full`, the job that had
failed, was skipped. Neither change was verified by anything.

To run the full suite on your branch, dispatch Main Pipeline on it. `workflow_dispatch` sets
`FULL_RUN` and enables `test-windows-full`:

```bash
gh workflow run main.yml --ref <your-branch>
gh run list --workflow main.yml --branch <your-branch> --limit 5
```

Then link the run. For an intermittent failure one green run proves nothing: dispatch at least
three times, five if the failure rate looked below half, and link them all.

Dispatch needs `actions: write`. If your token does not have it, or the workflow refuses, the fix is
unproven: say so in your report, and prefer an issue with a proposed fix over a PR whose only
evidence is a check set that never ran the failing job.

Last check before you believe your own fix: would the failing step have reached the line you
changed? PR #1197 skipped a test that Windows CI already skipped, because no
`SQ_TEST_SRC__SAKILA_OR` is set there. The failure was in the fixture copy that ran before it. The
change could not have mattered.

## 6. Publish

### An issue, when the mechanism is not established

This is the good outcome for a flake you cannot yet explain, and it is worth more than a skip.
[#1198](https://github.com/neilotoole/sq/issues/1198) is the model: the verbatim error, a "Seen
in" list of runs with dates and test names, the mechanism as far as it is known, a proposal, and
acceptance criteria. Label it `ci`, add `windows` when it is Windows-only. Say plainly what is not
yet known.

### A PR, when you have a fix and proof

Every one of these is a hard requirement.

- **Base** is the branch that failed, per [Provenance](#3-provenance).
- **Diff contains nothing but the fix.** Check it and read the list:

  ```bash
  git diff --stat origin/<base>...HEAD
  ```

  If a file you did not deliberately change appears, your base is wrong. Fix the base. Do not ship
  it with an explanation.
- **Size is proportionate.** A CI fix is small. More than about 50 non-test lines, more than five
  files, or any change to `go.sum`, a lockfile, or `site/` content that the failure did not
  implicate means stop and report instead. PR #1073 rewrote 2,465 lines of `site/bun.lock` to fix
  a broken documentation link.
- **No drive-by anything.** No dependency bumps, no reformatting, no unrelated doc edits. A stale
  `netlify-cli` bump swept into PR #1110 was still being cited when it was closed a month later.
- **Branch name** follows [`AGENTS.md`](../../../AGENTS.md#git-branch-naming): `fix/` for a bug
  fix, `chore/` for CI and tooling, plus `gh<ISSUE>-` when an issue exists. Not `feature/` for a
  fix, and not an opaque automation suffix.
- **`CHANGELOG.md`** gets an entry under `## Unreleased` for a user-visible fix. A test-only or
  CI-only change does not.
- **`make fmt && make lint`** before pushing, always.
- **PR body** states the failing run link, the mechanism, why this fix addresses that mechanism,
  and the dispatch runs that demonstrate it. If a related issue exists, link it rather than
  restating it.
- **Re-check prior art immediately before opening.** Minutes matter here: PR #1145 was opened ten
  minutes after the PR that superseded it merged.

## 7. Clean up

Leftover branches and stale PRs are part of the cost being measured. Seven orphaned branches from
this automation had to be deleted by hand.

- Abandoning a line of work means deleting the branch: `git push origin --delete <branch>`.
- If your PR is superseded or made obsolete, close it yourself, say in one line what supersedes
  it, and delete the branch. Do not wait to be told.
- Never leave a pushed branch with no PR and no explanation.
