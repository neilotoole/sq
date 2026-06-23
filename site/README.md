# sq.io website

[![Netlify Status](https://api.netlify.com/api/v1/badges/7caea069-2a8d-4f0b-bafe-b053bbc5eb08/deploy-status)](https://app.netlify.com/)

This directory is the source for the [sq.io](https://sq.io) website inside the
[sq](https://github.com/neilotoole/sq) monorepo (`site/`). It hosts documentation for the
[`sq`](https://github.com/neilotoole/sq) CLI.

This site is built using:

- [Hugo](https://gohugo.io) site generator
- [Doks](https://getdoks.org) theme
- [Bun](https://bun.sh) tooling
- [Netlify](https://www.netlify.com) hosting

Production publishes to [sq.io](https://sq.io) are **manual** (workflow dispatch) or
**automatic on stable sq releases**. Merging `site/**` to `master` runs Site CI only.
See [CI Workflow](#ci-workflow) below.

## Contributing

Open an [issue](https://github.com/neilotoole/sq/issues) or submit a [pull request](https://github.com/neilotoole/sq/pulls)
against [`neilotoole/sq`](https://github.com/neilotoole/sq) in this monorepo.

### 1. Clone the monorepo and enter this directory

```bash
git clone https://github.com/neilotoole/sq.git && cd sq/site
```

### 2. Install dependencies

From this directory (`site/`), use the **Makefile** (recommended) or Bun directly:

```bash
make deps
# same as: bun install
```

### 3. Make changes and test locally

```bash
# Dev server at http://localhost:1313 (may take a minute to come up; be patient)
make site-local
# same as: bun start

# Stable checks (what PRs block on): scripts, styles, markdown, and internal links
make site-test
# same as: bun run test:ci

# Optional: Docker smoke checks against a containerized dev server (:8080)
make smoke-test

# Optional: full link crawl (includes third-party URLs; can be noisy/slow)
make site-test-full
# same as: bun run test:full

# Production build to public/
make site-build
# same as: bun run build
```

Verify local tooling (Bun, Hugo, Netlify CLI, jq, curl): `make check`.

**Netlify API credentials** (maintainers only, for `make site-netlify-validate`):

```bash
cp .env.example .env
# Edit .env — see .env.example for where to get each value
make check-netlify
```

`check-netlify` runs `scripts/checkenv.bash --merge` against `.env` (sync keys
from `.env.example`, then validate; no interactive prompts).
`.env` is gitignored; never commit it.

One-shot check matching **Site CI** (`deps` → test → build): `make ci`.

**Netlify deploy-preview validate** (Dependabot Full mode / Layer B):

Check out the **PR branch** at its current head (`gh pr checkout <n>`) with a
**clean** working tree before validating — Layer B uploads the local `site/`
tree, not GitHub’s PR ref.

```bash
make check-netlify                        # once per machine / after editing site/.env
export MESSAGE="PR #573 dependabot shx"   # optional
make site-netlify-validate
```

Same variables as [Site Publish (dispatch)](../.github/workflows/site-publish-dispatch.yml)
GitHub Actions secrets.

Runs `netlify-cli deploy --build --context deploy-preview` and polls the
Netlify API until `state=ready`. See
[`.agents/skills/sq-site-dependabot/`](../.agents/skills/sq-site-dependabot/).

### Site testing

Think of site testing as two layers:

1. **Stable checks (merge-blocking on PRs)**
   `make ci` runs `make site-test`, which runs `bun run test:ci`. That includes
   link checking against the **temporary local build** we serve for linting, but
   it does **not** crawl arbitrary third-party websites. This is what you want
   to be strict about: broken docs routes, missing local assets, bad internal
   links, etc.

2. **External link crawl (informational on PRs)**
   GitHub Actions also runs a full `bun run lint:links` crawl after `make ci`.
   That step is configured as **non-fatal** because third-party sites can be
   slow, flaky, or rate-limit automated requests even when your change is fine.

If you want the full external crawl locally, use `make site-test-full` (or
`bun run lint:links`).

**Heads up (`bun run test` vs CI):** `bun run test` currently runs the **full**
lint pipeline (including external link crawling). PR CI uses `bun run test:ci`
via `make site-test`, which is the **stable** lane.

### 4. Submit a Pull Request

Create a PR in [`neilotoole/sq`](https://github.com/neilotoole/sq/pulls) with your changes under `site/`.

## Content Style Guide

### Alerts

Use Hugo alert shortcodes to highlight important information:

```gotemplate
{{< alert icon="👉" >}}
This is an important note for the reader.
{{< /alert >}}
```

## Development

### CI Workflow

The project uses GitHub Actions and Netlify for continuous integration:

| Trigger                                 | Action                                                                                                                    |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Push to `master` or `develop` (and PRs) | `.github/workflows/site-ci.yml` runs `make ci` when `site/**` changes, plus an informational full link crawl              |
| Pull request                            | Netlify deploy preview (when configured)                                                                                  |
| Stable GitHub release (`vX.Y.Z`)        | `.github/workflows/site-publish-release.yml` builds, deploys to [sq.io](https://sq.io), and runs post-deploy smoke checks |
| Manual `workflow_dispatch`              | `.github/workflows/site-publish-dispatch.yml` — publish doc or dependency changes before the next release                 |
| Daily schedule / manual                 | `.github/workflows/site-links-nightly.yml` runs a full external link crawl                                                |
| Daily schedule / manual                 | `.github/workflows/site-data-nightly.yml` refreshes `data/github.toml`; push triggers Site CI only (no production deploy) |

Merging to `master` with changes under `site/**` runs Site CI (lint, build, artifact
validation) but does **not** update production. Netlify's git integration remains
suppressed via `[context.production] ignore = "exit 0"` in `netlify.toml` — production
deploys go through GitHub Actions + the Netlify CLI
([`site-publish-netlify.yml`](../.github/workflows/site-publish-netlify.yml)).

To publish before the next sq release, use **Site Publish (dispatch)**: Actions tab →
**Site Publish (dispatch)** → Run workflow → select ref → type `DEPLOY`. Stable sq
releases trigger **Site Publish (release)** automatically when GoReleaser publishes
the GitHub release. Requires `NETLIFY_AUTH_TOKEN` and `NETLIFY_SITE_ID` repo secrets.
The GitHub `production` environment must not require manual approval or publish stalls.

Netlify still provides deploy previews for every PR with Lighthouse audits for
performance, accessibility, best practices, and SEO. Before merging, click
through to the deploy preview (e.g.,
the deploy-preview URL from the PR checks) to verify your changes look
correct.

### Build-time data & nightly refresh

The header's **version badge** and **GitHub star count** are computed at build time rather than
fetched in the browser. `scripts/gen-site-data.js` fetches the latest `sq` release and the repo
star count and writes `site/data/github.toml`; it also writes `static/version.json` (served at
[`/version`](https://sq.io/version) via a redirect). The header template renders those values. The
generator runs during `prebuild` (so every `bun run build` / `make site-build` refreshes them), and
the committed `data/github.toml` is the offline / fetch-failure fallback — a GitHub hiccup never
fails a build.

The nightly [`site-data-nightly.yml`](../.github/workflows/site-data-nightly.yml) workflow (07:00
UTC) keeps `data/github.toml` current in `master`. When it commits a change, Site CI runs on
`master` (build only — no production deploy until manual dispatch or the next stable release).

Since `master` uses classic branch protection (`enforce_admins=false`, no per-app bypass), that push
authenticates with the **`SITE_DATA_PUSH_TOKEN`** repo secret — a fine-grained PAT owned by a repo
admin (Contents: read/write on `neilotoole/sq`); an admin's push bypasses the PR requirement. Keep
the token scoped to this repo only, and prefer an expiration or periodic rotation (it is currently
set without one). Revoke it if leaked, or if this workflow is removed.

### Commands

Prefer **`make`** from this directory for install, test, and production build (see `Makefile`).
In **Common.make**, `make build` builds the **Docker** image, not the Hugo site;
use **`make site-build`** for a normal production build to `public/`.

| Make target                  | Bun equivalent              | Description                                           |
| ---------------------------- | --------------------------- | ----------------------------------------------------- |
| `make check`                 | —                           | Verify Bun, Hugo, Netlify CLI, jq, curl               |
| `make check-netlify`         | —                           | `make check` + `checkenv` on `.env`                   |
| `make deps`                  | `bun install`               | Install dependencies                                  |
| `make site-local`            | `bun scripts/dev-server.js` | Hugo dev server                                       |
| `make smoke-test`            | (script)                    | Docker smoke checks (`validate-build.sh --start`)     |
| `make site-test`             | `bun run test:ci`           | Stable linters + internal link check                  |
| `make site-test-full`        | `bun run test:full`         | Full linters + external link crawl                    |
| `make site-build`            | `bun run build`             | Production site → `public/`                           |
| `make ci`                    | (sequence below)            | `deps`, then `site-test`, then `site-build` (CI)      |
| `make site-netlify-validate` | —                           | Netlify deploy-preview build + API poll (`NETLIFY_*`) |

Other **package.json** scripts (call with `bun run …`):

| Command                  | Description                                             |
| ------------------------ | ------------------------------------------------------- |
| `bun run preview`        | Build and serve locally at http://localhost:1313        |
| `bun run gen:cmd-help`   | Regenerate command help files in `content/en/docs/cmd/` |
| `bun run gen:syntax-css` | Regenerate syntax highlighting CSS                      |

### Regenerating Command Documentation

The `gen:cmd-help` script (`./content/en/docs/cmd/generate-cmd-help.sh`) regenerates the `.help.txt`
files in `content/en/docs/cmd/`. These files contain the help text for each `sq` command and are
included in the documentation pages.

### Link Checking

Link checking uses [linkinator](https://github.com/JustinBeckwith/linkinator).

- **Stable / PR-blocking (`test:ci`)** uses `lint:links:internal`, which checks the
  locally served build without following arbitrary third-party `http(s)` links.
- **Full crawl (`lint:links`)** follows third-party links too. This is useful,
  but inherently more flaky.

Some sites (e.g., StackOverflow) block automated crawlers, returning 403 errors
in CI. Those domains are excluded in `linkinator.config.json`.

Note: `linkinator` timeouts are configured in **milliseconds** in
`linkinator.config.json` (see linkinator CLI help).

## Redirects

- You can use the Hugo [alias](https://gohugo.io/content-management/urls/#aliases) mechanism to
  maintain an old path that will redirect to the new path.
- If you need a redirect that's not associated with Hugo content, add an entry to
  the [`static/_redirects`](/static/_redirects) file. This is what the site uses to
  serve the [sq.io/install.sh](https://sq.io/install.sh) script.

## Misc

- Doks comes with [commands](https://getdoks.org/docs/prologue/commands/) for common tasks.
- Use `bun run gen:syntax-css` to regenerate the syntax highlight theme. The themes (light and dark)
  are specified in [generate-syntax-css.sh](./generate-syntax-css.sh).

### Asciinema

The site makes use of [asciinema](https://asciinema.org) via
the [gohugo-asciinema](https://github.com/cljoly/gohugo-asciinema) Hugo module.

Typically, casts are stored in `./static/casts`. To include a cast, use this shortcode:

```gotemplate
{{< asciicast src="/casts/home-quick.cast" theme="monokai" poster="npt:0:20" rows="10" speed="1.5" idleTimeLimit="3" >}}
```

- `poster="npt:0:20"` specifies that the "poster" or cover image should be taken from 0m20s into the
  cast.
- Add `autoPlay="true"` if the cast should start immediately. This is usually not the case.

If you see this problem:

```shell
Error: Error building site: "content/en/_index.md:9:1": failed to extract shortcode: template for shortcode "asciicast" not found
```

It probably means that the hugo module cache is out of whack. Run `bun install` and try again.

## Documentation

- [Netlify](https://docs.netlify.com/)
- [Hugo](https://gohugo.io/documentation/)
- [Doks](https://getdoks.org/)

## Communities

- [sq discussions](https://github.com/neilotoole/sq/discussions)
- [Netlify Community](https://community.netlify.com/)
- [Hugo Forums](https://discourse.gohugo.io/)
- [Doks Discussions](https://github.com/h-enk/doks/discussions)

## Acknowledgements

- Special thanks to [Netlify](https://www.netlify.com), who provide
  free hosting for [sq.io](https://sq.io) via
  their [open source program](https://www.netlify.com/open-source/).
- A bunch of stuff has been lifted from the [Docsy theme](https://www.docsy.dev).

[![Deploys by Netlify](https://www.netlify.com/v3/img/components/netlify-dark.svg)](https://www.netlify.com)
