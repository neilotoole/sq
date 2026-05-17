---
name: sq-site-dependabot
description: >-
  Reviews, validates, and safely merges Dependabot pull requests for the sq.io
  site (site/, Bun lockfile). Use when clearing site dependency PRs, triaging
  Dependabot failures, or checking Lighthouse impact before merge.
license: MIT
compatibility: >-
  Requires gh CLI (authenticated), Bun 1.2+, make, and network access to GitHub
  and Netlify deploy previews.
metadata:
  author: Todd Papaioannou
  homepage: https://sq.io
  version: "0.1.0"
---

# sq-site-dependabot

Scaffold only in this PR. A follow-up PR adds the full workflow (tool bootstrap,
risk tiers, Netlify validation, merge scripts, and references).

## When to use

- Open Dependabot PRs touching [`site/`](../../../site/) or `site/bun.lock`
- Clearing a backlog of site dependency updates
- Investigating Site CI vs Netlify deploy-preview failures on dependabot branches

## Until the full skill ships

1. Read [AGENTS.md](../../../AGENTS.md#agent-skills-contributors) for skill paths
   and `npx skills add` usage.
2. From `site/`, run `make ci` before merging any site dependency PR.
3. Confirm the Netlify deploy preview is green on the PR’s current head SHA
   before approve/merge.

Do not merge site Dependabot PRs in bulk without rebasing between merges (shared
`bun.lock`).
