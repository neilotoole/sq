---
name: sq-actions-dependabot
description: >-
  Reviews and merges Dependabot pull requests for GitHub Actions (the
  github-actions ecosystem) that bump `uses:` pins in `.github/workflows/`. Use
  for Dependabot github_actions PRs (branches like `dependabot/github_actions/...`),
  not go.mod or site/ Bun PRs.
license: MIT
compatibility: >-
  Requires gh CLI (authenticated) and network access to GitHub. actionlint is
  optional for local linting; CI already runs it in the `lint` job.
metadata:
  author: Todd Papaioannou
  homepage: https://sq.io
  version: "0.1.0"
---

# sq-actions-dependabot

Maintainer workflow for Dependabot PRs in the **github-actions** ecosystem:
version bumps to `uses:` action pins under
[`.github/workflows/`](../../../.github/workflows/). For [`go.mod`](../../../go.mod) /
[`go.sum`](../../../go.sum) PRs use
[`sq-gomod-dependabot`](../sq-gomod-dependabot/SKILL.md); for
[`site/`](../../../site/) Bun/Hugo PRs use
[`sq-site-dependabot`](../sq-site-dependabot/SKILL.md).

Actions are pinned by **full commit SHA** with a trailing `# vX` tag comment (for
example `docker/login-action@650006c... # v4`). Dependabot bumps the SHA and the
comment together; confirm they stay in sync.

No lockfile is involved, so action PRs are loosely coupled. Two PRs can still
conflict when they edit adjacent `uses:` lines in the same workflow file
([`.github/workflows/main.yml`](../../../.github/workflows/main.yml) holds most
pins), so merge sequentially and `@dependabot rebase` the next PR after each
squash merge.

## Operating modes

| Mode         | Actions                              | Merge  |
| ------------ | ------------------------------------ | ------ |
| **Audit**    | List/classify; publisher + bump type | No     |
| **Validate** | Diff review; CI green; actionlint    | No     |
| **Full**     | Validate + merge with consent        | Per PR |

Default to **Audit** unless the user asks to merge.

## Phase 0 — Tool bootstrap

```bash
command -v gh >/dev/null && gh auth status
# Optional local workflow lint (CI already runs actionlint in the `lint` job):
#   bash <(curl https://raw.githubusercontent.com/rhysd/actionlint/main/scripts/download-actionlint.bash)
```

## Phase 1 — Discovery

From repository root:

```bash
gh pr list --author 'app/dependabot' --state open \
  --json number,title,headRefName,headRefOid,mergeable,statusCheckRollup \
  --jq '.[] | select(.headRefName | test("^dependabot/github_actions/"))'
```

Match on `headRefName`, not the title. Titles such as "bump **go**releaser-action"
or "bump **golang**ci-lint-action" trip the gomod skill's word filter but belong
here. Confirm the diff only touches `.github/workflows/` (`gh pr diff <n> --name-only`).

## Phase 2 — Risk

| Level  | Examples                                                                                    | Action                       |
| ------ | ------------------------------------------------------------------------------------------- | ---------------------------- |
| Low    | Patch/minor bump, first-party publisher (`actions/`, `docker/`, `golangci/`, `goreleaser/`) | Merge after CI               |
| Medium | Major bump (`v4` to `v5`); action used only in release/publish jobs                         | Read changelog; check inputs |
| High   | New/untrusted publisher; SHA and `# vX` comment disagree; non-SHA (tag or branch) pin       | Hold; manual review          |

Trusted publishers already in `.github/workflows/` carry low supply-chain risk. A
**major** bump can change or remove `with:` inputs, so read the release notes and
confirm the workflow still passes the right inputs.

**Release-only caveat:** PR CI runs the jobs triggered by the `pull_request` event
(`lint`, `test-nix`, `test-windows-smoke`). Steps and jobs gated to releases or tags
(`goreleaser`, `docker/login`, publish, `binaries-*`) are skipped on the
PR, so green CI does **not** exercise them. For those, lean on the changelog and
trusted-publisher status.

## Phase 3 — Validate

- Required checks green on the current head (`gh pr checks <n>`). The `lint` job
  runs `actionlint` over the workflow files.
- SHA/tag consistency: the new `# vX` comment matches the tag for the bumped SHA.
- Optional local lint after installing actionlint: `./actionlint -color`.
- Stale-head guard: after any `@dependabot rebase`, re-check that the required
  checks passed on the new `headRefOid` before merging.

## Phase 4 — Merge (consent-gated)

After required checks pass on the current head:

```bash
gh pr merge <n> --squash --delete-branch
```

Multiple action PRs: merge one, then `@dependabot rebase` the next and wait for
`mergeable` plus fresh CI before merging it. Use `--admin` only when the user
explicitly requests it and checks are green but merge is blocked.

## Verdict template

```markdown
## Dependabot github-actions PR #NNN — <action>

- **Publisher / bump:** first-party? patch | minor | major
- **Runs on PR CI:** yes | release-only (not exercised)
- **CI (lint/actionlint, test-nix):** pass / fail
- **SHA vs tag comment:** consistent?
- **Verdict:** merge | hold
```

See [AGENTS.md](../../../AGENTS.md#agent-skills-contributors) and
[`docs/WORKFLOW.md`](../../../docs/WORKFLOW.md) for the CI job map.
