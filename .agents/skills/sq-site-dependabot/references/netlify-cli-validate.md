# Netlify CLI validate (Layer B)

`make site-netlify-validate` deploys the **current `site/` tree** to Netlify
using the **deploy-preview** context and Netlify's remote build (`--build`).
This catches failures that `make ci` and stale GitHub checks miss.

## When to run

| Mode       | Layer B                                              |
| ---------- | ---------------------------------------------------- |
| Audit      | No                                                   |
| Validate   | Yes, when user asks for full local validation        |
| Full       | **Required** before `gh pr review --approve`         |

## Prerequisites

1. PR branch checked out at **`headRefOid`** (`gh pr checkout <n>`); **clean**
   working tree (uncommitted `site/` changes validate the wrong tree).
2. `cd site && make ci` already passed (recommended).
3. `site/.env` filled from `.env.example` (see
   [tool-bootstrap.md](./tool-bootstrap.md)).
4. Phase 0: `check-tools.sh --netlify` or `make check-netlify`

## Command

From `site/`:

```bash
export MESSAGE="PR #573 dependabot shx"   # optional
make site-netlify-validate
```

Implementation: [`site/scripts/netlify-deploy-validate.sh`](../../../../site/scripts/netlify-deploy-validate.sh)

## Behavior

1. `bun x netlify-cli deploy --build --context deploy-preview --message "…" --json`
2. Parse `deploy_id` from JSON (CLI exit code alone is not trusted).
3. Poll `https://api.netlify.com/api/v1/sites/{SITE_ID}/deploys/{deploy_id}` until
   `state` is `ready`, or fail on `error` / `rejected` / timeout (~2.5 min).

Stdout includes `Deploy ID`, `Deploy URL`, and final `state=ready`.

## Degraded path (no `site/.env`)

Without a filled `site/.env`, **do not** run Full-mode merge automation.
You may still:

- Rely on Layer A (Netlify Git deploy preview green on current `headRefOid`)
- Run `make ci` locally

Document in the verdict that Layer B was skipped and merge is manual.

## Not in v1

- `site-netlify-validate` is **not** wired into `site-ci.yml` (secrets + duplicate
  build cost). Skill + Makefile only.

## TODO (maintainers)

- Extract shared **deploy JSON parse + API poll-until-ready** logic from
  [`netlify-deploy-validate.sh`](../../../../site/scripts/netlify-deploy-validate.sh)
  and [Site Publish (dispatch)](../../../../.github/workflows/site-publish-dispatch.yml)
  into one script; keep separate deploy flags (`--build` preview vs `--prod`).
