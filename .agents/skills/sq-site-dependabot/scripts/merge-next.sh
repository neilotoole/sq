#!/usr/bin/env bash
# Consent-gated template: merge one site Dependabot PR after validation.
# Do not run without explicit user approval for the PR number(s) listed.
#
# Usage (from repo root):
#   gh pr checkout <n>
#   CONFIRM_MERGE=1 PR=<n> MESSAGE="dependabot shx" \
#     ./.agents/skills/sq-site-dependabot/scripts/merge-next.sh
#
# Requires: gh auth, NETLIFY_* for Layer B, PR branch checked out at headRefOid,
# clean working tree (or ALLOW_DIRTY_TREE=1).
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

# Ensure HEAD matches the PR head SHA (label is for log messages).
verify_pr_head() {
	local label="$1"
	local head_oid current_oid
	head_oid=$(gh pr view "${PR}" --json headRefOid --jq '.headRefOid')
	current_oid=$(git -C "${REPO_ROOT}" rev-parse HEAD)
	if [ "${current_oid}" != "${head_oid}" ]; then
		echo "error: ${label}: HEAD ${current_oid} != PR #${PR} headRefOid ${head_oid}" >&2
		echo "Checkout the PR at its current head: gh pr checkout ${PR}" >&2
		exit 1
	fi
	echo "headRefOid=${head_oid} (${label})"
}

verify_clean_tree() {
	if [ "${ALLOW_DIRTY_TREE:-}" = "1" ]; then
		echo "warning: ALLOW_DIRTY_TREE=1; skipping clean working tree check" >&2
		return 0
	fi
	if [ -n "$(git -C "${REPO_ROOT}" status --porcelain)" ]; then
		echo "error: working tree has uncommitted changes (commit, stash, or ALLOW_DIRTY_TREE=1)" >&2
		exit 1
	fi
}

# Layer A: all PR checks must pass (exits non-zero on failure or pending).
verify_pr_checks() {
	local label="$1"
	echo "==> Layer A: PR checks (${label})"
	if ! gh pr checks "${PR}"; then
		echo "error: PR #${PR} checks not all successful (run: gh pr checks ${PR})" >&2
		exit 1
	fi
}

"${SCRIPT_DIR}/check-tools.sh" --netlify

echo "==> Pre-flight (PR head + clean tree)"
verify_clean_tree
verify_pr_head "pre-flight"

cd "${REPO_ROOT}/site"
echo "==> Local CI (make ci)"
make ci

verify_pr_head "after make ci"
verify_pr_checks "after make ci"

echo "==> Netlify Layer B (site-netlify-validate)"
export MESSAGE="${MESSAGE}"
make site-netlify-validate

verify_pr_head "before merge"
verify_pr_checks "before merge"

echo "==> Approve and merge PR #${PR}"
gh pr review "${PR}" --approve \
	--body "Validated: make ci, Layer A (PR checks on head), Layer B (site-netlify-validate). ${MESSAGE}"
gh pr merge "${PR}" --squash --delete-branch

echo "Done. Comment @dependabot rebase on the next PR before continuing the batch."
