---
name: sq-gomod-dependabot
description: >-
  Reviews and merges Dependabot pull requests for Go modules (gomod) at the sq
  repo root. Use for dependabot gomod PRs, go.mod/go.sum updates, and Go module
  security bumps—not site/ Bun PRs.
license: MIT
compatibility: >-
  Requires gh CLI (authenticated), Go toolchain, make test-short, and network
  access to GitHub.
metadata:
  author: Todd Papaioannou
  homepage: https://sq.io
  version: "0.2.0"
---

# sq-gomod-dependabot

Maintainer workflow for Dependabot PRs updating [`go.mod`](../../../go.mod) /
[`go.sum`](../../../go.sum) at the repository root. For [`site/`](../../../site/)
Bun/Hugo PRs, use [`sq-site-dependabot`](../sq-site-dependabot/SKILL.md).

No `bun.lock` sequencing — multiple gomod PRs are less coupled than site PRs,
but still prefer merging after CI is green.

## Operating modes

| Mode         | Actions                          | Merge       |
| ------------ | -------------------------------- | ----------- |
| **Audit**    | List/classify; direct vs indirect| No          |
| **Validate** | Diff review; `make test-short`   | No          |
| **Full**     | Validate + merge with consent    | Per PR      |

Default to **Audit** unless the user asks to merge.

## Phase 0 — Tool bootstrap

```bash
command -v gh >/dev/null && gh auth status
command -v go >/dev/null && go version
```

## Phase 1 — Discovery

From repository root:

```bash
gh pr list --author 'app/dependabot' --state open \
  --json number,title,headRefName,mergeable,statusCheckRollup \
  --jq '.[] | select(.title | test("go|gomod|golang"; "i"))'
```

Confirm the PR does **not** only touch `site/`. If it touches both, split
judgment: site hunks → `sq-site-dependabot`.

## Phase 2 — Risk

| Level    | Examples                         | Action              |
| -------- | -------------------------------- | ------------------- |
| Low      | Patch indirect, test-only modules| Merge after CI      |
| Medium   | Direct minor/patch runtime dep   | Notes + test-short  |
| High     | Major, `replace`, breaking sec   | Hold; full review   |

## Phase 3 — Validate

On PR branch:

```bash
make test-short
# or make test for full driver integration (Docker)
```

Review `go mod why` / diff for unexpected indirect churn.

## Phase 4 — Merge (consent-gated)

After required checks pass:

```bash
gh pr merge <n> --squash --delete-branch
```

Use `--admin` only when the user explicitly requests and checks are green.

## Verdict template

```markdown
## Dependabot gomod PR #NNN — <module>

- **Direct/indirect:** …
- **CI:** pass / fail
- **make test-short:** pass / fail
- **Verdict:** merge | hold
```

See [AGENTS.md](../../../AGENTS.md#agent-skills-contributors).
