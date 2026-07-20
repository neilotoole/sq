# High-risk packages (site/)

Package-specific notes for **T3/T4** or elevated **T2** PRs. Cross-check the
Dependabot PR title and `site/bun.lock` diff against this list.

## `@hyas/images` (T4)

- Build-time image processing; can change dimensions, formats, and page weight.
- **Hold** bulk merges; verify key doc pages and hero images on deploy preview.
- Replacement/alternate PRs may be needed if Dependabot cannot auto-resolve
  (see sq repo history: held PRs, manual migration).

## JS lint / formatting (moved to root toolchain)

Site JS linting (formerly ESLint) and formatting (formerly Stylelint /
markdownlint) moved to the repo-root Bun toolchain: **Biome** (JS lint) and
**dprint** (formatting). Those bumps arrive through the **root `/` bun
ecosystem**, not this site flow, so they are out of scope here.
`site/package.json` no longer carries any linter or formatter.

## `flexsearch` / search index (T4)

- Affects client-side search behavior and index build scripts.
- Smoke-test search on preview (`/` site search UI) before merge.

## `linkinator` (T2–T3)

- Timeout and skip-list changes affect CI noise, not just dependency version.
- Full external crawl remains **non-blocking** on PRs; do not block T0/T1 merges
  on nightly/external flake unless `make site-test` fails.

## `netlify-cli` (T2; can fail Layer A while Site CI passes)

- Bumping `netlify-cli` runs the **new** package `postinstall` during `bun install`
  on Netlify deploy previews (it is a site devDependency).
- If preview fails at **Install dependencies**, reproduce with
  `cd site && bun install` on the PR branch; use
  [netlify-build-debug.md](./netlify-build-debug.md).
- Example (#621): v26.0.2 postinstall failed on Netlify (Node 22) — `execa` CJS
  vs ESM in `@netlify/config`; hold until upstream fix or verified override.

## Netlify plugins (T2)

- `@netlify/plugin-lighthouse` — failed plugin can fail deploy preview checks
  even when `make ci` passes.
- `netlify-plugin-submit-sitemap` — production-oriented; preview still runs
  plugin hooks — watch build log for plugin errors.

## Hugo / Doks / Hyas stack (T3–T4)

- `hugo`, `hugo-mod-*`, theme-related modules: read release notes for breaking
  template API changes.
- Pin alignment: `site/netlify.toml` `HUGO_VERSION` and CI must stay consistent
  after bump PRs (sometimes a separate maintainer commit is required).

## When in doubt

- Classify as **T2** minimum, run **Validate** mode, and set verdict **hold**
  until a human confirms preview + Lighthouse.
