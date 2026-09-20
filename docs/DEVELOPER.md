# Developing sq

The local development loop: the `Makefile` targets, the inner loop, the git hooks, and site
local dev. What happens once a pull request is open is in [`CI.md`](./CI.md).
[`CONTRIBUTING.md`](../CONTRIBUTING.md) covers the surrounding contributor process (opening
issues and PRs).

## Local development

The [`Makefile`](../Makefile) is the canonical developer entry point. `make help`
(the default target) lists every target with a one-line description.

```
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
3. `make fmt` before committing. A **`pre-commit` hook** (activated by
   `make init`) runs `dprint check` on staged files, so a formatting slip is
   caught locally instead of failing the **Format** CI job. Bypass for one
   commit with `git commit --no-verify`.
4. `make test-short` for the fast pass; `make test` for the full suite
   (needs Docker for the `sakiladb/*` images used by SQL-driver integration
   tests).
5. `make all` before opening a PR: it runs what the PR's fast loop runs (gen, fmt, lint,
   test, build). See [what gates what](./CI.md#what-gates-what) for how CI results are used.

Mark long-running tests with [`tu.SkipShort`](../testh/tu/skip.go) so they stay out of the dev loop
(`-short`) but still run in the nightly and release suites (see [`CI.md`](./CI.md)).

### Site (sq.io) local dev

The website under [`site/`](../site) has its own Bun/Hugo tooling and
`Makefile`. `make site-local` (from the repo root) delegates to it; for the
full set of site commands (dev server, link checking, Netlify validation) see
[`site/README.md`](../site/README.md) and [`site/CLAUDE.md`](../site/CLAUDE.md).
