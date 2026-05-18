# Tool bootstrap (Phase 0)

Run before discovery, validation, or merges. **Stop** if any required check
fails and install or authenticate before continuing.

## Required tools

- **GitHub CLI** — `gh api user` (authenticated). Audit, Full.
- **Site tools** — `cd site && make check`. Validate, Full.
- **Netlify env** — `cd site && make check-netlify`. Layer B, Full only.

## Netlify credentials (Validate Layer B / Full only)

Maintainers store credentials in **`site/.env`** (gitignored). Template:
**`site/.env.example`** (committed).

```bash
cd site
cp .env.example .env
# Edit .env — see "Getting tokens" below
make check-netlify    # tools + checkenv.bash validates .env
```

`make site-netlify-validate` loads `.env` automatically after `check-netlify`
passes.

Same variables as [Site Publish (dispatch)](../../../../.github/workflows/site-publish-dispatch.yml)
GitHub Actions secrets. Never commit `.env`.

### Getting tokens (Netlify UI)

You need deploy rights on the **sq.io** Netlify site. UI login alone does not
set shell variables — create these two values:

1. **`NETLIFY_AUTH_TOKEN`**
   - [Netlify → User settings → Applications → Personal access tokens](https://app.netlify.com/user/applications#personal-access-tokens)
   - **New access token** → copy the token once (shown only at creation).
   - Use a token that can deploy to the sq.io site (your Netlify team).

2. **`NETLIFY_SITE_ID`**
   - [Netlify dashboard](https://app.netlify.com/) → select the **sq.io** site →
     **Site configuration** → **General** → **Site ID** (UUID, not the site URL
     slug).
   - Or from `site/`: `bun x netlify-cli link` (select the sq.io site), then
     `bun x netlify-cli status` (shows site id).

Paste both into `site/.env`, then `make check-netlify`.

## One-liner (repo root)

```bash
.agents/skills/sq-site-dependabot/scripts/check-tools.sh
.agents/skills/sq-site-dependabot/scripts/check-tools.sh --netlify
```

Or:

```bash
gh api user -q .login
cd site && make check
cd site && make check-netlify
```

## Install hints

- **gh:** [cli.github.com](https://cli.github.com/) — `brew install gh`;
  `gh auth login`. `check-tools.sh` sets `GH_PAGER=cat` so a shell
  `PAGER=less` does not pause at `(END)` mid-run.
- **Bun:** [bun.sh](https://bun.sh) — pin to `site/netlify.toml` /
  [site-ci.yml](../../../../.github/workflows/site-ci.yml)
- **Netlify CLI:** `cd site && bun install` (devDependency)
- **checkenv:** `site/scripts/checkenv.bash` (from
  [code-red-base](https://github.com/Bain/code-red-base))

## Working directory

- `gh` commands: **repository root** (`sq/`)
- `make ci`, `make site-netlify-validate`: **`site/`** on the PR branch
