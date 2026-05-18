# Merge failures and recovery

Use when a batch merge, rebase, or Netlify validation fails.

## Stale-head guard

Before approve/merge in Full mode:

```bash
gh pr view <n> --json headRefOid,mergeable,statusCheckRollup
```

If Site CI or Netlify completed on an **older** SHA than current `headRefOid`,
wait for checks on the current head or re-run Phase 3 validation. Do not merge
on outdated Lighthouse/CI.

## Sequential merges and rebase

After merging PR *N*:

```bash
gh pr comment <next> --body "@dependabot rebase"
```

Poll until mergeable:

```bash
gh pr view <next> --json mergeable
# every 10s, max ~5 min
```

- **Rebase never `MERGEABLE` (timeout):** Stop batch; list PR numbers; manual
  `@dependabot rebase` or close/reopen; do not skip to the next PR with conflicts.
- **Site CI fails after rebase:** Hold that PR; `make ci` locally; T3 vs real
  breakage; no merge until green.
- **Netlify preview failed or pending:** Do not merge; open build log; run
  `make site-netlify-validate` to reproduce.
- **`site-netlify-validate` failed:** Do not merge; compare Layer A log; fix;
  re-run from clean `make ci`.
- **Missing `site/.env`:** Copy `site/.env.example`, fill tokens, run
  `make check-netlify`. No Full automation until set (note in verdict).
- **Merge blocked (reviews, permissions):** Stop; check `gh pr view` merge state;
  approve from a non-author account or adjust branch protection.
- **Head SHA changed mid-batch:** Re-run validation for affected PRs.

## Admin merge (exception only)

Use **only** when the user explicitly requests it **and** required checks are
green but merge is still blocked:

```bash
gh pr merge <n> --squash --admin --delete-branch
```

Default remains:

```bash
gh pr merge <n> --squash --delete-branch
```

## Happy-path sequence (Full mode)

1. Phase 0 — `check-tools.sh --netlify`
2. Checkout PR branch; `cd site && make ci`
3. Layer A — Netlify preview green on `headRefOid`
4. Layer B — `make site-netlify-validate`
5. Stale-head guard (re-check `headRefOid`)
6. `gh pr review <n> --approve --body "…"`
7. `gh pr merge <n> --squash --delete-branch`
8. `@dependabot rebase` on next PR

Script template: [`../scripts/merge-next.sh`](../scripts/merge-next.sh) (consent-gated).
Requires `gh pr checkout <n>`, clean tree, passing `gh pr checks` (Layer A), then
Layer B. Set `ALLOW_DIRTY_TREE=1` only to override the clean-tree guard.

## Post-batch

- List remaining open site Dependabot PRs.
- Remind: production <https://sq.io> requires manual **Site Publish (dispatch)**.
