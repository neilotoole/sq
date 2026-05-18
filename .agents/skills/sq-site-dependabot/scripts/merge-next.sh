#!/usr/bin/env bash
# Consent-gated template: merge one site Dependabot PR after validation.
# Do not run without explicit user approval for the PR number(s) listed.
#
# Usage (from repo root):
#   PR=573 MESSAGE="dependabot shx" ./.agents/skills/sq-site-dependabot/scripts/merge-next.sh
#
# Requires: gh auth, NETLIFY_* for Layer B, PR branch checked out.
set -euo pipefail

export GH_PAGER=cat
export PAGER=cat

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
PR="${PR:?Set PR to the pull request number}"
MESSAGE="${MESSAGE:-PR #${PR} dependabot merge}"

if [ -z "${CONFIRM_MERGE:-}" ]; then
	echo "Refusing to merge: set CONFIRM_MERGE=1 after explicit user consent." >&2
	echo "Example: CONFIRM_MERGE=1 PR=${PR} MESSAGE='…' $0" >&2
	exit 1
fi

"${SCRIPT_DIR}/check-tools.sh" --netlify

cd "${REPO_ROOT}/site"
echo "==> Local CI (make ci)"
make ci

echo "==> Stale-head guard"
head_oid=$(gh pr view "${PR}" --json headRefOid --jq '.headRefOid')
echo "headRefOid=${head_oid}"
gh pr checks "${PR}" || true

echo "==> Netlify Layer B (site-netlify-validate)"
export MESSAGE="${MESSAGE}"
make site-netlify-validate

echo "==> Approve and merge PR #${PR}"
gh pr review "${PR}" --approve --body "Validated: make ci + Netlify deploy-preview CLI (Layer B). ${MESSAGE}"
gh pr merge "${PR}" --squash --delete-branch

echo "Done. Comment @dependabot rebase on the next PR before continuing the batch."
