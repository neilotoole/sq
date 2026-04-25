---
title: "Agent Skills"
description: >-
  Install the sq Agent Skill for Claude Code, Cursor, and other assistants that
  support the Agent Skills format.
lead: ""
draft: false
images: []
weight: 1025
toc: true
url: /docs/agent-skills
---

The **sq** [Agent Skill](https://agentskills.io/) helps coding assistants use an
already-installed `sq` CLI: SLQ and native SQL, `@` handles, output formats,
`inspect`, `diff`, and `tbl`, plus driver-specific notes under `references/`. It
does **not** install the `sq` binary—[install `sq` first](/docs/install), then
[add the skill](#install-the-skill) to your assistant.

Canonical sources live in the GitHub repository under
[`skills/sq/`](https://github.com/neilotoole/sq/tree/master/skills/sq/)
(`SKILL.md` and `references/*.md`).

## Prerequisites

- `sq` on your `PATH` from a normal [install](/docs/install).
- Verify the CLI:

```shell
sq --version
sq help
sq driver ls
```

## Install the skill {#install-the-skill}

Pick one path below. Adjust GitHub URLs if you use a fork or mirror.

### `npx skills` (recommended)

```shell
npx skills add neilotoole/sq
```

This pulls the **`/sq`** skill from the default location of `https://github.com/neilotoole/sq`.

### Claude Code plugin

[Claude Code](https://code.claude.com/docs/en/plugins) can load the skill as a
plugin via the in-repo
[`.claude-plugin/`](https://github.com/neilotoole/sq/tree/master/.claude-plugin)
catalog (**marketplace `sq-io`**, **plugin `sq`**).

1. Add the marketplace (GitHub URL or local checkout path):

   ```text
   /plugin marketplace add https://github.com/neilotoole/sq
   ```

1. Install the plugin:

   ```text
   /plugin install sq@sq-io
   ```

1. Run `/reload-plugins` if the skill does not appear.

For a local checkout, pass the repository path to `marketplace add` instead of
the GitHub URL. See Claude’s
[plugin marketplaces](https://code.claude.com/docs/en/plugin-marketplaces) doc
for branches, updates, and troubleshooting.

### Cursor and other editors

Follow your editor’s documentation for **Agent Skills** or **project skills**
paths. The payload is the same tree as `skills/sq/` in this repository; only
the install location differs by product and version. The easiest path
is usually to use the `npx skills` [instructions flow](#install-the-skill), as this currently
supports more than 20 different agents out of the box.

### Manual copy

Copy the
[`skills/sq/`](https://github.com/neilotoole/sq/tree/master/skills/sq) directory
into your agent’s skills location (see your tool’s Agent Skills documentation
for the expected directory layout), or open
[`SKILL.md`](https://github.com/neilotoole/sq/blob/master/skills/sq/SKILL.md) and
follow its headings.

## Usage

Once installed, assistants should treat the skill as **progressive
disclosure**:

- Start from `SKILL.md` for command summaries and links into
  [sq.io](https://sq.io/) docs.
- Load driver-specific files under `references/` when the task names a
  database or file format (see the table in `SKILL.md`).

The skill intentionally defers to **`sq help`**, **`sq <cmd> --help`**, and
sq.io rather than duplicating full reference material. For the raw source, see
[`SKILL.md` on GitHub](https://github.com/neilotoole/sq/blob/master/skills/sq/SKILL.md).

## Updating

- **`npx skills`:** Re-run `npx skills add …` after releases or when you want the
  latest `skills/sq/` from the default branch.
- **Claude plugin:** Use your marketplace’s update flow (see Claude Code docs).
- **`sq` binary:** Upgrade `sq` separately via your package manager; the skill
  and the CLI are versioned independently.

## Example

![Skill Screenshot](/images/sq_skill.png)

## FAQ

**Does the skill replace reading sq.io?**
No. It steers assistants toward official docs and `--help` output.

**Do I need the skill to use sq?**
No. It is optional context for AI-assisted workflows.

**Air-gapped or offline?**
Copy `skills/sq/` from a release tarball or clone; you do not need `npx` if you
install manually.

**Forks**
Point `npx skills add` and marketplace URLs at your fork’s GitHub URL or
local path.
