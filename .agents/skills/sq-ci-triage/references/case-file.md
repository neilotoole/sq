# Case file: thirteen closed autofix PRs

Every PR the ci-autofix automation opened on `neilotoole/sq` between 2026-06-21 and 2026-09-16,
with the reason it was closed and what fixed the problem instead. The reasons are the maintainer's,
quoted or paraphrased from the closing comment. This is the evidence behind
[`SKILL.md`](../SKILL.md); read it when you want to know why a rule is there.

Thirteen PRs, none merged. Eight proposed silencing a test. Ten were superseded by a fix that
landed elsewhere. Seven carried commits belonging to another branch.

| PR                                                  | Opened     | Proposed                                    | Closed as                              | Fixed instead by |
| --------------------------------------------------- | ---------- | ------------------------------------------- | -------------------------------------- | ---------------- |
| [#921](https://github.com/neilotoole/sq/pull/921)   | 2026-06-21 | OS-independent file placeholder handles     | Superseded, part out of scope          | #926             |
| [#922](https://github.com/neilotoole/sq/pull/922)   | 2026-06-21 | Skip a macOS completion timeout test        | "Skipping the test is not the answer." | n/a              |
| [#925](https://github.com/neilotoole/sq/pull/925)   | 2026-06-21 | Windows file and source test fixes          | Superseded                             | #926, #932       |
| [#1061](https://github.com/neilotoole/sq/pull/1061) | 2026-07-23 | "Fix lint and format regressions", 21 files | Not a CI fix                           | n/a              |
| [#1073](https://github.com/neilotoole/sq/pull/1073) | 2026-07-25 | Fix a dead `/config` link via the lockfile  | Unrelated to the link                  | n/a              |
| [#1110](https://github.com/neilotoole/sq/pull/1110) | 2026-08-22 | Redirect the dead `cj.rs` vanity import     | Superseded, worse fix                  | #1111            |
| [#1145](https://github.com/neilotoole/sq/pull/1145) | 2026-09-09 | Repin completion cases off DuckDB           | Superseded, reverses #1143             | #1144, #1153     |
| [#1154](https://github.com/neilotoole/sq/pull/1154) | 2026-09-10 | Skip `TestStartMemStatsTracker` on Windows  | Root-caused as #1181 instead           | #1195            |
| [#1156](https://github.com/neilotoole/sq/pull/1156) | 2026-09-10 | Skip the downloader live-network test       | Superseded                             | #1158            |
| [#1159](https://github.com/neilotoole/sq/pull/1159) | 2026-09-10 | The same test, 43 minutes later             | Superseded                             | #1158            |
| [#1180](https://github.com/neilotoole/sq/pull/1180) | 2026-09-15 | That test again plus the memstats skip      | Superseded                             | #1158, #1195     |
| [#1197](https://github.com/neilotoole/sq/pull/1197) | 2026-09-15 | Skip `TestSakilaCrossDatabase` on Windows   | A no-op on the wrong target            | #1199, #1196     |
| [#1200](https://github.com/neilotoole/sq/pull/1200) | 2026-09-15 | Skip `TestCmdSQL_ExecMode/@sakila_duck`     | Same root cause as #1197               | #1199, #1196     |

## The instructive ones

**#1061** added 1,150 lines across 21 files, including whole new packages (`cli/footer`,
`cli/updatecheck`), under the title "Fix lint and format regressions". It was a feature branch
re-proposed against `master`.

**#1073** rewrote 2,465 lines of `site/bun.lock` to fix one broken documentation link.

**#1110** lost to #1111, a sibling automation's PR merged two hours later. #1111 kept the real
pseudo-version, kept the `cj.rs` hashes in `site/go.sum`, and explained why the redirect was
needed; #1110 dropped all three and swept in a stale `netlify-cli` bump that was still being cited
when it was closed a month later.

**#1145** opened ten minutes after #1144 merged. Its Oracle and join changes were byte-identical to
what had already landed, and repinning the completion cases to Postgres reversed the goal of #1143,
since no CI leg is guaranteed to have Postgres live.

**#1154** proposed a Windows skip inside a 10-file diff of unrelated DuckDB work. The flake was
Windows clock granularity: on windows/amd64 `nanotime` reads `InterruptTime`, which advances only on
clock ticks of up to 15.6ms, so a forced GC on a small heap records a zero pause. The diff also
predated #1195 and would have conflicted with it.

**#1197** was +517/-32 against `master` when the change it intended was one line, because the branch
started from the head of another open PR and carried that PR's work along. The skip could not have
mattered either way: Windows CI sets no `SQ_TEST_SRC__SAKILA_OR`, so the test already skipped there,
and the failure was in the fixture copy that ran before it.

## The three flakes it went after

All three had a real cause, and none of the tests it proposed skipping was at fault.

**DuckDB fixture lock on Windows** (#1197, #1200). Two tests opened the shared
`drivers/duckdb/testdata/sakila.duckdb` read-write. On Windows that is a sharing violation for any
concurrent copy of the file, so the failure landed on whichever test lost the race. Skipping either
would have silenced one symptom and left the cause to surface against the next test. Root cause
[#1198](https://github.com/neilotoole/sq/issues/1198), fixed by #1199 (`TestCmdAdd` opens a copy)
and #1196 (`Helper.Source` writes the copy's location back into the collection entry).

**`TestStartMemStatsTracker`** (#1154, #1180). Windows clock granularity, not a bad test. Fixed by
making the test deterministic in #1195.

**Downloader live-network test** (#1156, #1159, #1180). Three attempts at the same target. Handled
by #1158: fixtures are served from a local test server, and `tu.SkipNoNetwork` gates the handful of
tests that deliberately use a real host, with `SQ_TEST_NETWORK` set on the nightly schedule only.

## Collateral

- Two races against work already in flight: #1110 lost to a sibling automation's #1111, and #1145
  lost to the maintainer's #1144. Both left orphaned branches.
- Seven of the automation's own PRs were open at once on 2026-09-15 (#1145, #1154, #1156, #1159,
  #1180, #1197, #1200). All seven were closed unmerged.
- Seven orphaned branches were deleted by hand on 2026-09-16.
- The automation's own checks were green on #1197 and #1200 while `test-windows-full`, the job that
  had failed, was skipped. PRs do not run it. See [Proof](../SKILL.md#5-proof).
