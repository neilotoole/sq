---
name: sq-site-dependabot
description: >-
  Reviews, validates, and safely merges Dependabot pull requests for the sq.io
  site (site/, Bun lockfile). Use when clearing site dependency PRs, triaging
  Dependabot failures, or checking Lighthouse impact before merge.
license: MIT
compatibility: >-
  Requires gh CLI (authenticated), Bun 1.2+, make, jq, curl, and network
  access to GitHub and Netlify. Full merges need NETLIFY_AUTH_TOKEN and
  NETLIFY_SITE_ID for make site-netlify-validate.
metadata:
  author: Todd Papaioannou
  homepage: https://sq.io
  version: "0.2.1"
---

# sq-site-dependabot

Maintainer workflow for Dependabot PRs touching [`site/`](../../../site/) or
`site/bun.lock`. Read [AGENTS.md](../../../AGENTS.md#agent-skills-contributors)
for skill install paths.

**Do not** merge site Dependabot PRs in bulk without rebasing between merges
(shared `bun.lock`).

## Operating modes

| Mode         | Actions                         | Merge   |
| ------------ | ------------------------------- | ------- |
| **Audit**    | List/classify; CI; ordered plan | No      |
| **Validate** | Branch checkout; `make ci`      | No      |
| **Full**     | Audit + validate + merge loop   | Consent |

Default to **Audit** unless the user says "merge", "clear them", or "full".

## Phase 0 — Tool bootstrap

Run first in every mode. Stop on failure.

```bash
# gh auth + site deps (bun install if needed) + make check
.agents/skills/sq-site-dependabot/scripts/check-tools.sh
# Full / Layer B (+ NETLIFY_* via make check-netlify):
.agents/skills/sq-site-dependabot/scripts/check-tools.sh --netlify
# Or: gh api user -q .login && cd site && bun install && make check-netlify
```

`check-tools.sh` runs `bun install` in `site/` when `bun x netlify-cli` is missing
(fresh clone, agent sandbox). Needs network. `SKIP_SITE_DEPS=1` skips that step.
Layer B (`site-netlify-validate`) always uses `bun x netlify-cli` — a global/brew
CLI does not replace `bun install`.

Details: [references/tool-bootstrap.md](references/tool-bootstrap.md).

## Phase 1 — Discovery

From repository root:

```bash
gh pr list --author 'app/dependabot' --state open \
  --json number,title,headRefName,mergeable,statusCheckRollup,createdAt \
  --jq '.[] | select(.headRefName | test("^dependabot/"))'
```

Confirm each candidate touches `site/` (`gh pr diff <n> --name-only`). Treat the list as
**candidates** — refine by path if the filter is too broad.

For each PR:

- Confirm changes are under `site/`.
- Record mergeable state, Site CI, Netlify deploy-preview URL, Lighthouse if present.
- Flag false-positive Site CI noise (external link crawl) — see
  [references/ci-and-checks.md](references/ci-and-checks.md).

## Phase 2 — Risk classification

Read [references/risk-tiers.md](references/risk-tiers.md) before ordering merges.
Package notes: [references/high-risk-packages.md](references/high-risk-packages.md).

Produce an ordered plan (T0 → T1 → T2; hold T3/T4).

## Phase 3 — Local validation

Checkout the PR branch. From `site/`:

```bash
make deps    # if needed after checkout
make ci      # matches Site CI (necessary, not sufficient for Netlify)
```

Pin Bun to [`site/netlify.toml`](../../../site/netlify.toml) `BUN_VERSION` and
[site-ci.yml](../../../.github/workflows/site-ci.yml).

Optional: `make site-lighthouse` for T2+ when preview Lighthouse is unclear.

## Netlify validation before merge

### Layer A — PR deploy preview (Git integration)

After `make ci` on the PR branch:

1. Stale-head guard: `gh pr view <n> --json headRefOid,mergeable,statusCheckRollup`
2. `gh pr checks <n>` — Netlify check **success** on current `headRefOid`
3. Open deploy-preview URL; confirm published (not building/failed)
4. T1+: review `@netlify/plugin-lighthouse` on preview if available

If pending: poll ~5 min. If failed: do not merge; run
`debug-netlify-pr.sh <n>` or see [references/netlify-build-debug.md](references/netlify-build-debug.md);
recovery steps in [references/merge-failures.md](references/merge-failures.md).

### Layer B — Netlify CLI (required in Full mode)

From `site/` on the PR branch (after Layer A is green on the same head):

```bash
# site/.env from .env.example (see tool-bootstrap.md)
export MESSAGE="PR #NNN dependabot <package>"   # optional
make site-netlify-validate
```

See [references/netlify-cli-validate.md](references/netlify-cli-validate.md).

**Full mode sequence:**

```text
check-tools --netlify → make ci → Layer A → site-netlify-validate → merge
```

Without `site/.env`, do not run Full automation; document degraded path
in the verdict.

## Phase 4 — Merge automation (consent-gated)

Only with explicit user consent per PR or batch.

Template script (sets `CONFIRM_MERGE=1` only after consent). Checkout the PR
first; working tree must match `headRefOid` (clean tree, or `ALLOW_DIRTY_TREE=1`):

```bash
gh pr checkout 573
CONFIRM_MERGE=1 PR=573 MESSAGE="dependabot shx" \
  ./.agents/skills/sq-site-dependabot/scripts/merge-next.sh
```

`merge-next.sh` enforces Layer A (`gh pr checks`), HEAD = `headRefOid`, then Layer B.

Happy path:

1. Stale-head guard (re-check `headRefOid`)
2. Layer A green on current head
3. `make site-netlify-validate` (Layer B)
4. `gh pr review <n> --approve --body "…"`
5. `gh pr merge <n> --squash --delete-branch` (default; no `--admin`)
6. `gh pr comment <next> --body "@dependabot rebase"`
7. Poll `gh pr view <next> --json mergeable` every 10s (max ~5 min)

**Admin merge** only when user explicitly requests and checks are green but merge
is blocked: `gh pr merge <n> --squash --admin --delete-branch`.

Failures: [references/merge-failures.md](references/merge-failures.md).

## Phase 5 — Verdict template

Per PR (GitHub comment or chat):

```markdown
## Dependabot PR #NNN — <package>

- **Tier:** T0–T4
- **Site CI:** pass / fail (root cause)
- **Netlify preview (A):** URL + check on head SHA
- **Netlify CLI (B):** deploy_id, deploy_url, state (or skipped)
- **Lighthouse:** perf/a11y/bp/seo deltas (or N/A)
- **Local `make ci`:** pass / fail
- **Verdict:** merge | hold | close + migration PR
- **Next step:** …
```

## Phase 6 — Post-batch cleanup

- List remaining open site Dependabot PRs.
- Note stale local branches for prune.
- Remind: merging Dependabot PRs does **not** update <https://sq.io>. Production
  updates on a stable sq release ([Site Publish (release)](../../../.github/workflows/site-publish-release.yml))
  or manual [Site Publish (dispatch)](../../../.github/workflows/site-publish-dispatch.yml).
  Use dispatch when dependency changes should go live before the next release.

## Repo cross-links

- [site/README.md](../../../site/README.md) — testing, `site-netlify-validate`
- [site/Makefile](../../../site/Makefile) — `check`, `ci`, validate, Lighthouse
- [site/netlify.toml](../../../site/netlify.toml) — Bun/Hugo pins, preview build

## Reference index

- [tool-bootstrap.md](references/tool-bootstrap.md)
- [risk-tiers.md](references/risk-tiers.md)
- [high-risk-packages.md](references/high-risk-packages.md)
- [ci-and-checks.md](references/ci-and-checks.md)
- [netlify-cli-validate.md](references/netlify-cli-validate.md)
- [merge-failures.md](references/merge-failures.md)
- [netlify-build-debug.md](references/netlify-build-debug.md)
