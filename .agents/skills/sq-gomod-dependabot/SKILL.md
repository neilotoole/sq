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
  version: "0.1.0"
---

# sq-gomod-dependabot

Placeholder skill for Go module Dependabot PRs. Expand when gomod PR volume
warrants a detailed workflow.

## When to use

- Dependabot PRs updating `go.mod` / `go.sum` at the repository root
- Not for [`site/`](../../../site/) Bun/Hugo dependency PRs (use
  `sq-site-dependabot`)

## Minimal workflow

1. `gh auth status` — GitHub CLI required.
2. Review the diff: direct vs indirect dependency, security advisory if any.
3. `make test-short` (or `make test` for broader coverage).
4. `gh pr merge --squash --delete-branch` after required checks pass.

No `bun.lock` sequencing issue; multiple gomod PRs are less coupled than site
PRs.

See [AGENTS.md](../../../AGENTS.md#agent-skills-contributors).
