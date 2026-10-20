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

### Website (sq.io) local dev

The website under [`site/`](../site) builds [sq.io](https://sq.io). It has its own Bun/Hugo tooling
and `Makefile`. `make site-local` (from the repo root) delegates to it; for the
full set of site commands (dev server, link checking, Netlify validation) see
[`site/README.md`](../site/README.md) and [`site/CLAUDE.md`](../site/CLAUDE.md).

## Dev container

If you'd rather not install the [tools](../CONTRIBUTING.md#tooling) on your machine, or just
prefer working in a container, [`.devcontainer/`](../.devcontainer) defines a
[dev container](https://containers.dev) with all of them. It is a Debian image with Go, gcc (for
cgo), Java (for ANTLR), bun, and various libraries and utilities such as shellcheck and jq. The
container runs `make init` when it's created, so the inner loop above works as is. Start it from
any of these:

- **Command line.** Install the [Dev Container CLI](https://github.com/devcontainers/cli)
  (`brew install devcontainer`, or `npm install -g @devcontainers/cli`), then from the repo root:

  ```bash
  devcontainer up --workspace-folder .                     # build and start the container
  devcontainer exec --workspace-folder . make test-short   # run a command in it
  devcontainer exec --workspace-folder . bash              # or open a shell
  ```

- **GoLand** (or IntelliJ IDEA with the Go plugin). Open `.devcontainer/devcontainer.json`,
  click the container icon in the gutter, and choose **Create Dev Container and Mount Sources**.
  The IDE backend runs inside the container.
- **VS Code.** With the
  [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
  extension installed, run **Dev Containers: Reopen in Container** from the Command Palette.
- **GitHub Codespaces.** On the repo page choose **Code → Codespaces → Create codespace**, or
  run `gh codespace create -R neilotoole/sq`. Nothing to install locally, and no Docker needed.

The first three need Docker on your machine and bind-mount your clone into the container.

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
- **Git worktrees.** A worktree's `.git` is a file pointing into the main clone, so the container
  needs the main clone's `.git` mounted too, and the link must be a relative path (git 2.48 or
  later) to resolve inside the container. Convert your worktrees once, from the main clone:

  ```bash
  git config worktree.useRelativePaths true   # new worktrees get relative paths
  git worktree repair --relative-paths        # convert the existing ones
  ```

  Then add `--mount-git-worktree-common-dir` to `devcontainer up`, or in VS Code turn on the
  `dev.containers.experimentalMountGitWorktreeCommonDir` setting. GoLand has no equivalent, so
  use a regular clone there. Relative worktrees mark the repository with
  `extensions.relativeWorktrees`, which tools built on libgit2 older than 1.9.4 refuse to open.

The [**Dev Container**](./CI.md#workflow-inventory) workflow builds the image and runs
`make gen`, `make lint` and `make test-short` inside it, on changes to `.devcontainer/` or the
`Makefile` and weekly.
