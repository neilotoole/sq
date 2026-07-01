# Development & CI workflows

How work flows through this repo: the local development loop (driven by the
`Makefile`) and the GitHub Actions workflows under
[`.github/workflows/`](../.github/workflows).

This is the canonical reference for the dev loop and CI.
[`CONTRIBUTING.md`](../CONTRIBUTING.md) covers the surrounding contributor
process (opening issues and PRs).

## Local development

The [`Makefile`](../Makefile) is the canonical developer entry point. `make help`
(the default target) lists every target with a one-line description.

```bash
make init        # one-time clone setup: install deps + activate git hooks
make deps        # install dev deps (bun packages: dprint, biome + Go modules)
make all         # the full local pipeline: gen + fmt + lint + test + build + install
make test        # run all tests (SQL-driver tests need the sakiladb/* Docker images)
make test-short  # run tests with -short (skips long-running / container-backed tests)
make gen         # code generation (go generate + betteralign on generated code)
make fmt         # format the repo: Go imports (goimports-reviser) + dprint for the rest
make fmt-check   # dprint check, read-only (does not modify files)
make lint        # golangci-lint + shellcheck + dprint check + biome (site JS)
make build       # build the sq binary to dist/sq
make install     # build + install sq into the Go bin dir
```

### The inner loop

1. `make init` once after cloning: installs dependencies and activates the
   repo git hooks (`.githooks`).
2. Edit code. If you touch generated inputs (the [SLQ grammar](./GRAMMAR.md),
   anything under `go generate`), run `make gen`.
3. `make fmt` before committing. A **`pre-commit` hook** (installed by
   `make init`) runs `dprint check` on staged files, so a formatting slip is
   caught locally instead of failing the **Format** CI job. Bypass for one
   commit with `git commit --no-verify`.
4. `make test-short` for the fast pass; `make test` for the full suite
   (needs Docker for the `sakiladb/*` images used by SQL-driver integration
   tests).
5. `make all` before opening a PR: it mirrors the merge-blocking CI set
   (gen, fmt, lint, test, build).

Mark long-running tests with `tu.SkipShort` so they stay out of the dev loop
(`-short`) but still run in the nightly / release suites.

### Site (sq.io) local dev

The website under [`site/`](../site) has its own Bun/Hugo tooling and
`Makefile`. `make site-local` (from the repo root) delegates to it; for the
full set of site commands (dev server, link checking, Netlify validation) see
[`site/README.md`](../site/README.md) and [`site/CLAUDE.md`](../site/CLAUDE.md).

## GitHub Actions

CI is **PR-centric**: a branch gets CI once a pull request exists. The dev loop
is deliberately fast; the expensive suites run nightly and on release tags, and
can be run on demand from the Actions tab (**Run workflow** →
`workflow_dispatch`).

| Workflow                   | File                                                                          | Trigger                                        | Purpose                                              |
| -------------------------- | ----------------------------------------------------------------------------- | ---------------------------------------------- | ---------------------------------------------------- |
| Main Pipeline              | [`main.yml`](../.github/workflows/main.yml)                                   | PR, push `master`, tag `v*`, nightly, dispatch | Build, test, and release the Go project              |
| Format                     | [`format.yml`](../.github/workflows/format.yml)                               | push/PR touching formatted file types          | `dprint check` + Biome (repo-wide formatting gate)   |
| DB integration             | [`db-integration.yml`](../.github/workflows/db-integration.yml)               | `workflow_call`, dispatch                      | Reusable per-engine SQL-driver integration tests     |
| DB integration (scheduled) | [`db-scheduled.yml`](../.github/workflows/db-scheduled.yml)                   | nightly, weekly, dispatch                      | Drives `db-integration` across engine versions       |
| Coverage                   | [`coverage.yml`](../.github/workflows/coverage.yml)                           | nightly, dispatch                              | Full test run with coverage → Codecov                |
| CodeQL                     | [`codeql.yml`](../.github/workflows/codeql.yml)                               | tag `v*`, nightly, dispatch                    | Go security analysis                                 |
| CodeQL site                | [`codeql-site.yml`](../.github/workflows/codeql-site.yml)                     | push/PR on `site/**`, weekly                   | JS security analysis for the site                    |
| Dependency Review          | [`dependency-review.yml`](../.github/workflows/dependency-review.yml)         | PR (non-doc paths)                             | Flags risky dependency changes                       |
| Test Install               | [`test-install.yml`](../.github/workflows/test-install.yml)                   | `workflow_call`, dispatch                      | Verifies install mechanisms across platforms         |
| Site CI                    | [`site-ci.yml`](../.github/workflows/site-ci.yml)                             | push/PR on `site/**`                           | Lint + build the sq.io site (`make ci`)              |
| Site Publish (dispatch)    | [`site-publish-dispatch.yml`](../.github/workflows/site-publish-dispatch.yml) | manual (`workflow_dispatch`)                   | Publish sq.io to Netlify production before a release |
| Site Publish (release)     | [`site-publish-release.yml`](../.github/workflows/site-publish-release.yml)   | stable release published                       | Auto-publish sq.io after a stable release            |
| Site Publish to Netlify    | [`site-publish-netlify.yml`](../.github/workflows/site-publish-netlify.yml)   | `workflow_call`                                | Shared build + Netlify upload + post-deploy smoke    |
| Site data (nightly)        | [`site-data-nightly.yml`](../.github/workflows/site-data-nightly.yml)         | daily 07:00 UTC, dispatch                      | Refresh `site/data/github.toml` (version + stars)    |
| Site Links (nightly)       | [`site-links-nightly.yml`](../.github/workflows/site-links-nightly.yml)       | daily 07:15 UTC, dispatch                      | External link crawl for sq.io                        |

### Main Pipeline (Go build / test / release)

[`main.yml`](../.github/workflows/main.yml) is the core Go workflow. It skips
doc-only, `site/**`, and `sq.json` (scoop) changes on PRs, but always runs on
`master` merges and `v*` tags.

**Fast loop** (every PR push and `master` merge; on a PR, pushing again cancels
the superseded run, while `master` merges and release tags always run to
completion):

- **`lint`**: actionlint (workflow files), shellcheck, Go import-grouping
  check, and golangci-lint.
- **`test-nix`**: Linux + macOS tests with `-short` (long-running tests
  skipped).
- **`test-windows-smoke`**: focused smoke suite (`./test/smoke/...`) to catch
  CGO/SQLite breakage.

**Slow suites** (nightly against `master`, on `v*` tags, and on manual
dispatch):

- **`test-nix`** without `-short` (full suite).
- **`test-windows-full`**: the full Windows suite; **gates `publish`**.

**Release** (only on `v*` tags):

- `binaries-darwin`, `binaries-linux-amd64`, `binaries-linux-arm64`,
  `binaries-windows`: per-platform GoReleaser builds.
- **`publish`**: GoReleaser release (gated by lint + full test suites +
  binaries).
- **`docker-publish`**: builds and pushes the `ghcr.io` image from the linux
  release binaries; a sibling of `publish`, so the image ships with every
  release rather than being withheld by an unrelated install-test failure.
- **`test-install`**: runs _after_ publish and docker-publish as a
  **post-publish canary** (it gates nothing); a failure means users are already
  hitting the problem.

### Database integration tests

SQL-driver integration tests run against real databases and are split out of
the main pipeline:

- [`db-integration.yml`](../.github/workflows/db-integration.yml) is a
  **reusable** workflow (`workflow_call`) that takes a JSON
  `{engine: [tags]}` selection and a scope (`full` = `go test ./...`,
  `narrow` = per-engine packages). It stands up each engine as a service
  container, waits for health, and runs the tests.
- [`db-scheduled.yml`](../.github/workflows/db-scheduled.yml) drives it on a
  schedule: **nightly** every engine at `:latest` (full scope), and a
  **weekly** (Monday) full version sweep (narrow scope). Manual dispatch
  behaves like the nightly run. The engine/version matrix is defined in
  [`.github/sakila-db.json`](../.github/sakila-db.json) (Postgres, MySQL, SQL
  Server, ClickHouse, Oracle, rqlite).

A manual `db-integration` dispatch names the engines it wants (blank engine
inputs are skipped); "run everything at `:latest`" is the scheduler's job.

### Formatting, security & supply chain

- **Format** ([`format.yml`](../.github/workflows/format.yml)) runs `dprint
  check` and Biome on push/PR whenever a formatted file type changes. Because
  almost no one runs `make fmt` locally, the git `pre-commit` hook is the first
  line of defense and this workflow is the backstop.
- **CodeQL** ([`codeql.yml`](../.github/workflows/codeql.yml) for Go,
  [`codeql-site.yml`](../.github/workflows/codeql-site.yml) for the site JS)
  provides scheduled security analysis; the Go run also validates release tags.
- **Dependency Review**
  ([`dependency-review.yml`](../.github/workflows/dependency-review.yml)) flags
  risky dependency changes on PRs. Dependabot PRs are triaged via the maintainer
  skills under [`.agents/skills/`](../.agents/skills).
- **Coverage** ([`coverage.yml`](../.github/workflows/coverage.yml)) runs the
  full test suite nightly and uploads to Codecov.

### Site (sq.io) publishing

Production deploys go through GitHub Actions + the Netlify CLI; Netlify's own
git integration is suppressed (`production` ignore in `site/netlify.toml`).

- **Site CI** ([`site-ci.yml`](../.github/workflows/site-ci.yml)) lints and
  builds the site on `site/**` push/PR (via `make ci`); it does **not** deploy.
- **Site Publish (dispatch)**
  ([`site-publish-dispatch.yml`](../.github/workflows/site-publish-dispatch.yml))
  is the manual production publish (type `DEPLOY` to confirm) for shipping doc
  or dependency changes before the next release.
- **Site Publish (release)**
  ([`site-publish-release.yml`](../.github/workflows/site-publish-release.yml))
  auto-publishes after a stable `sq` GitHub release.
- Both call the reusable **Site Publish to Netlify**
  ([`site-publish-netlify.yml`](../.github/workflows/site-publish-netlify.yml)),
  which builds, uploads to Netlify, and runs post-deploy smoke checks. Requires
  the `NETLIFY_AUTH_TOKEN` and `NETLIFY_SITE_ID` secrets.
- **Site data (nightly)**
  ([`site-data-nightly.yml`](../.github/workflows/site-data-nightly.yml))
  refreshes `site/data/github.toml`; its commit to `master` (via the
  `SITE_DATA_PUSH_TOKEN` PAT) triggers Site CI only, never a production deploy.
- **Site Links (nightly)**
  ([`site-links-nightly.yml`](../.github/workflows/site-links-nightly.yml))
  runs the flaky external link crawl out of band, so PR CI stays fast and
  deterministic.

See [`site/README.md`](../site/README.md) and [`site/CLAUDE.md`](../site/CLAUDE.md)
for the site build, testing layers, and Netlify details.
