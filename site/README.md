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

Production publishes to [sq.io](https://sq.io) are **manual** — merges to
`master` no longer auto-deploy. See [CI Workflow](#ci-workflow) below.

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

| Trigger                                 | Action                                                                                                                |
|-----------------------------------------|-----------------------------------------------------------------------------------------------------------------------|
| Push to `master` or `develop` (and PRs) | `.github/workflows/site-ci.yml` runs `make ci` when `site/**` changes, plus an informational full link crawl          |
| Pull request                            | Netlify deploy preview (when configured)                                                                              |
| Merge to `master`                       | **No auto-deploy.** Netlify's production build is suppressed via `[context.production] ignore = "exit 0"` in `netlify.toml` |
| Manual `workflow_dispatch`              | `.github/workflows/site-publish-dispatch.yml` builds locally and uploads `site/public` to [sq.io](https://sq.io) via the Netlify CLI |
| Daily schedule / manual                 | `.github/workflows/site-links-nightly.yml` runs a full external link crawl                                            |

Production publishes to [sq.io](https://sq.io) are manual-only. To deploy,
go to the repo's **Actions** tab, pick **Site Publish (dispatch)**, click
**Run workflow**, select the ref you want to publish (any branch, or a
SemVer-style `vX.Y.Z` tag — pre-release suffixes like `v1.0.0-rc1` and
build metadata are also accepted), and type `DEPLOY` into the confirmation
field. The workflow rejects refs that are neither a branch nor a SemVer
v-tag, and rejects any confirmation value other than the literal string
`DEPLOY`.
Requires the `NETLIFY_AUTH_TOKEN` and `NETLIFY_SITE_ID` repo secrets to be
set.

Netlify still provides deploy previews for every PR with Lighthouse audits for
performance, accessibility, best practices, and SEO. Before merging, click
through to the deploy preview (e.g.,
the deploy-preview URL from the PR checks) to verify your changes look
correct.

### Commands

Prefer **`make`** from this directory for install, test, and production build (see `Makefile`).
In **Common.make**, `make build` builds the **Docker** image, not the Hugo site; use **`make site-build`** for a normal production build to `public/`.

| Make target      | Bun equivalent        | Description                                        |
|------------------|-----------------------|----------------------------------------------------|
| `make check`     | —                     | Verify Bun, Hugo, Netlify CLI, jq, curl            |
| `make check-netlify` | —                 | `make check` + `checkenv` on `.env`              |
| `make deps`      | `bun install`         | Install dependencies                               |
| `make site-local`| `bun scripts/dev-server.js` | Hugo dev server                                   |
| `make smoke-test`| (script)              | Docker smoke checks (`validate-build.sh --start`)  |
| `make site-test` | `bun run test:ci`     | Stable linters + internal link check               |
| `make site-test-full` | `bun run test:full` | Full linters + external link crawl                 |
| `make site-build`| `bun run build`       | Production site → `public/`                        |
| `make ci`        | (sequence below)      | `deps`, then `site-test`, then `site-build` (CI)   |
| `make site-netlify-validate` | — | Netlify deploy-preview build + API poll (`NETLIFY_*`) |

Other **package.json** scripts (call with `bun run …`):

| Command                  | Description                                             |
|--------------------------|---------------------------------------------------------|
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
