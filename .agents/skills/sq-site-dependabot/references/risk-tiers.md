# Risk tiers (site Dependabot)

Classify every open site PR before ordering merges. Read this file during
**Phase 2 — Risk classification** (Audit mode) and when drafting the verdict template.

## T0 — Trivial (merge first)

- Patch bumps to dev-only scripts (`shx`, `npm-run-all`, small typings)
- Documentation-only dependency metadata with no runtime/build change
- Lockfile-only churn with identical resolved tree (rare; verify diff)

**Validation:** Site CI green on head; Netlify preview green; optional local
`make ci` if checks are stale.

## T1 — Low risk

- Minor/patch bumps to Hugo modules, lint plugins, or build helpers with no
  config migration
- Dependabot PRs that only touch `site/package.json` + `site/bun.lock` for a
  single direct dependency

**Validation:** `make ci` on PR branch; Netlify Layer A; Layer B in Full mode.

## T2 — Medium (review changelog)

- Biome or dprint **minor** updates or new rules that may fail the Format gate
  (`make lint` / `make fmt-check`)
- `linkinator` or test-runner bumps
- Netlify plugin version bumps (`@netlify/plugin-lighthouse`, sitemap submit)

**Validation:** Full `make ci`; scan Netlify Lighthouse report on deploy preview
for regressions; consider `make site-lighthouse` locally for T2+ if preview
Lighthouse is ambiguous.

## T3 — High (hold or dedicated migration PR)

- **Major** Biome or dprint migrations (config-schema changes)
- Hugo/Doks major upgrades, theme swaps, or `bunfig` / build pipeline changes
- Anything that rewrites `site/config/`, `dprint.json` / `biome.json`, or CI scripts

**Action:** Do **not** merge via bulk Dependabot flow. Close or leave open;
open a focused migration PR with human review and expanded test plan.

## T4 — Critical / manual QA

- Image pipeline (`@hyas/images`, responsive srcsets, build-time image processing)
- Search (`flexsearch` index rebuild), sitemap/SEO plugins with URL changes
- Security-sensitive auth or redirect behavior

**Action:** Hold for manual QA on deploy preview (visual + Lighthouse). See
[high-risk-packages.md](./high-risk-packages.md).

## Default merge order (batch clears)

1. All **T0** PRs (lowest risk first within tier)
2. **T1** PRs
3. **T2** PRs (one at a time; watch lint failures after rebase)
4. Defer **T3/T4** — do not include in unattended batch merges

**Hyas / image stack last** even when classified T2: `@hyas/images` and friends
often need visual diff on preview.

## May 2026 lessons (sq.io site)

- Merge **sequentially** — shared `site/bun.lock`; rebase the next PR after
  each squash merge (`@dependabot rebase` on the following PR).
- Site CI **pass** does not guarantee Netlify preview **pass** (plugins,
  `DEPLOY_PRIME_URL` build, functions). Full mode requires Layer A + B.
- External link crawl failures in Site CI are often **informational** — see
  [ci-and-checks.md](./ci-and-checks.md).
