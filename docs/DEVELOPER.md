# Developing sq

The local development loop: the `Makefile` targets, the inner loop, the git hooks, site local
dev, and the dev container. What happens once a pull request is open is in [`CI.md`](./CI.md).
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

## Dev container

The repo ships a [dev container](https://containers.dev) in
[`.devcontainer/`](../.devcontainer) for contributors who aren't set up for the macOS toolchain
described in [`CONTRIBUTING.md`](../CONTRIBUTING.md#tooling). It is a Debian image with Go, gcc
(for cgo), Java (for ANTLR), bun, shellcheck and jq. Open the repo in VS Code with the Dev
Containers extension and choose **Reopen in Container**, or create a GitHub Codespace from the
repo page. The container runs `make init` when it's created, so the inner loop above works as
is.

Things to know:

- **Go version.** The image sets `GOTOOLCHAIN=auto`, so the `go` command downloads the version
  that [`go.mod`](../go.mod) names if the image's Go is older.
- **Caches.** The Go build and module caches, golangci-lint's cache and DuckDB's extension
  downloads live on named Docker volumes, so rebuilding the container doesn't start from cold.
- **`node_modules`.** Both `node_modules` directories (repo root and `site/`) are volumes too.
  They hold platform-specific binaries (dprint, biome, hugo), and the volumes keep the
  container's Linux binaries from replacing your host's when the repo is bind-mounted from macOS
  or Windows.
- **Sakila databases.** The container has no Docker access, so it can't start the `sakiladb/*`
  images. `make test` still runs, but the external-engine tests skip themselves because their
  envars are unset. To run those tests, use
  [`sakila-start-local.sh`](../sakila-start-local.sh) on a host with Docker (see
  [`SAKILA.md`](./SAKILA.md#running-external-engines-locally)).
- **Linux only.** The container can't reproduce macOS- or Windows-specific behavior. CI covers
  those (see [`CI.md`](./CI.md)).

The [**Dev Container**](./CI.md#workflow-inventory) workflow builds the image and runs
`make gen`, `make lint` and `make test-short` inside it, on changes to `.devcontainer/` or the
`Makefile` and weekly.
