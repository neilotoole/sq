# Netlify build debugging (Layer A failures)

Use when `netlify/sq-web/deploy-preview` or Netlify sub-checks (Header rules,
Pages changed, Redirect rules) fail on a Dependabot PR. Site CI (`lint` / `build`)
can still be green — the remote Netlify install/build is a separate path.

## Quick triage (PR number)

From repo root (requires `site/.env` with `NETLIFY_*` and `gh` auth):

```bash
.agents/skills/sq-site-dependabot/scripts/debug-netlify-pr.sh <pr>
```

Manual equivalent is below.

## 1. Collect deploy id from GitHub

```bash
export GH_PAGER=cat
gh pr checks <pr>   # non-zero if any check failed
gh pr view <pr> --json headRefOid,statusCheckRollup
```

- Open the Netlify check link (e.g. `app.netlify.com/.../deploys/<deploy_id>`).
- Copy **`deploy_id`** (24-char hex) from the URL or checks output.
- Confirm **`headRefOid`** matches the SHA Netlify built (stale-head guard).

## 2. Netlify API — deploy summary (CLI)

From `site/` with credentials loaded:

```bash
cd site
set -a && . ./.env && set +a

DEPLOY_ID="<from checks URL>"
bun x netlify-cli api getDeploy --data "{\"deploy_id\":\"${DEPLOY_ID}\"}"
```

Read these fields first:

| Field           | Meaning                         |
| --------------- | ------------------------------- |
| `state`         | `ready` / `error` / `building`  |
| `error_message` | One-line failure (often enough) |
| `commit_ref`    | Should match PR `headRefOid`    |
| `context`       | `deploy-preview` for PRs        |
| `review_id`     | PR number when Git-integrated   |

Example (**PR #621**): `state: error`, `error_message`:

`Failed during stage 'Install dependencies': dependency_installation script returned non-zero exit code: 1`

## 3. Netlify API — build record

`getDeploy` includes `build_id`. Fetch build metadata:

```bash
BUILD_ID="<from getDeploy.build_id>"
bun x netlify-cli api getSiteBuild --data "{\"build_id\":\"${BUILD_ID}\"}"
```

Check `error` and `deploy_state`. Build logs in the Netlify UI:
`admin_url` from `getDeploy` → deploys → **Deploy log** (API log endpoints
vary; UI is reliable).

## 4. Local reproduction (install vs Hugo build)

Checkout the PR and mirror Netlify’s first step (`bun install` in `site/`):

```bash
gh pr checkout <pr>
cd site
bun install          # fails here if remote “Install dependencies” failed
make ci              # only if install succeeds
```

Netlify preview config: [`site/netlify.toml`](../../../../site/netlify.toml)
(`BUN_VERSION`, `NODE_VERSION`, `[context.deploy-preview]` command).

**Important:** `netlify-cli` is a **devDependency** of `site/`. A PR that bumps
`netlify-cli` runs the **new** package’s `postinstall` during `bun install` on
Netlify and locally. A broken CLI release can fail before Hugo ever runs.

## 5. Layer B comparison

If Layer A failed but you need to test the tree anyway (after fixing install):

```bash
cd site
make site-netlify-validate   # uses current checkout + remote build
```

Layer B can pass when Layer A failed on an older SHA — always compare `commit_ref`
/ `headRefOid`.

## 6. Verdict hints for the agent

| `error_message` pattern                | Likely cause                                       | Action                                                           |
| -------------------------------------- | -------------------------------------------------- | ---------------------------------------------------------------- |
| `Install dependencies` / exit code 1   | `bun install` / postinstall                        | Open deploy log; reproduce `cd site && bun install` on PR branch |
| `execa` / `Named export` in deploy log | `netlify-cli` postinstall + wrong `execa` in graph | See case study #621; prefer hold over blind `overrides`          |
| `Building site` / Hugo                 | `bun run build`                                    | Reproduce `make ci`; open full deploy log                        |
| Plugin / Lighthouse                    | `@netlify/plugin-lighthouse`                       | Open deploy log plugin section; compare `make ci` pass           |
| Unknown / empty                        | Stale check or transient                           | Re-run deploy; confirm `headRefOid`                              |

## Case study: PR #621 (`netlify-cli` 25 → 26)

- **Symptom:** Layer A failed; Site CI green.
- **`getDeploy.error_message`:** Install dependencies stage failed.
- **Netlify deploy log (authoritative):** Node **v22.13.1**, Bun **1.3.13**,
  during `netlify-cli` postinstall:

  ```text
  @netlify/config/lib/env/git.js: import { execa } from 'execa';
  SyntaxError: Named export 'execa' not found. The requested module 'execa' is a
  CommonJS module …
  error: postinstall script from "netlify-cli" exited with 1
  ```

  `@netlify/config` expects ESM named exports; the resolved `execa@5` is CJS.

- **Local repro:** Run `cd site && bun install` on the PR branch. Outcome can
  differ by Node version (e.g. pass on Node 26, fail on Netlify’s Node 22).
  Always trust the **deploy log** over a single local run.

- **Netlify AI “fix” (`overrides.execa: ^8.0.1`):** May unblock
  `@netlify/config` on Node 22 but can break `netlify-cli`’s own
  `import execaLib from 'execa'` postinstall on other Node versions. Treat as
  experimental; verify with a fresh `bun.lock` on the PR branch and a new
  deploy preview before merging.

- **Conclusion:** Broken `netlify-cli@26` install graph, not a Hugo/site bug.
  **Hold** or close PR; stay on **25.x** until Netlify ships a fixed CLI or you
  have a verified override + green preview. Do not merge for green Site CI alone.

## gh + curl helpers

```bash
# Deploy poll (same as site-netlify-validate)
curl -fsS -H "Authorization: Bearer ${NETLIFY_AUTH_TOKEN}" \
  "https://api.netlify.com/api/v1/sites/${NETLIFY_SITE_ID}/deploys/${DEPLOY_ID}"
```

Use `jq` for `state`, `error_message`, `commit_ref`.
