# CI and checks (site/)

What runs on Dependabot PRs and how to interpret results.

## Site CI (merge-blocking)

Workflow: [`.github/workflows/site-ci.yml`](../../../../.github/workflows/site-ci.yml)

From `site/`:

```bash
make ci   # deps → site-test (test:ci) → site-build
```

This matches what PRs must pass. It does **not** run Netlify's builder, plugins,
or edge functions.

### Stable vs full link crawl

| Step                         | Command                    | Blocks merge? |
| ---------------------------- | -------------------------- | ------------- |
| Stable lint + internal links | `make site-test` / test:ci | **Yes**       |
| External link crawl          | `bun run lint:links`       | **No**        |

False positives: third-party sites returning 403/timeout to linkinator. If
**only** the external crawl step failed, re-check `make site-test` locally before
holding a T0/T1 PR.

See [site/README.md](../../../../site/README.md#site-testing).

## Netlify deploy preview (Layer A)

- Netlify builds on PR push using `[context.deploy-preview]` in
  [`site/netlify.toml`](../../../../site/netlify.toml):
  `bun run build -- -b $DEPLOY_PRIME_URL`
- GitHub shows a Netlify check; use `gh pr checks <n>` and open the preview URL.
- `@netlify/plugin-lighthouse` may attach scores under `reports/lighthouse.html`
  on the preview deploy.

**Pending check:** poll ~5 minutes; do not merge on assumptions.

**Failed check:** do not merge. Triage with
[netlify-build-debug.md](./netlify-build-debug.md) or
`debug-netlify-pr.sh <pr>`; compare with Layer B after install/build succeeds.

## Netlify CLI validate (Layer B)

Makefile: `make site-netlify-validate` in `site/`.

See [netlify-cli-validate.md](./netlify-cli-validate.md). Required in **Full**
mode before approve/merge.

## Local Lighthouse (optional)

```bash
cd site && make site-lighthouse
```

Production-like static server with gzip/brotli — **not** identical to Netlify's
CDN/plugin pipeline. Use when preview Lighthouse is inconclusive (T2+).

## Production publish (out of band)

Merging Dependabot PRs does **not** update <https://sq.io>. Production uses
[Site Publish (dispatch)](../../../../.github/workflows/site-publish-dispatch.yml)
only.

## gh commands (checks on head SHA)

```bash
gh pr view <n> --json headRefOid,mergeable,statusCheckRollup
gh pr checks <n>
```

Compare `headRefOid` to the SHA checks ran against (**stale-head guard** in
Full mode).
