# CLAUDE.md (cli package)

Conventions specific to the `cli` package: cobra command construction, flags,
shell completion, and output writers. Repo-wide rules in
[AGENTS.md](../AGENTS.md) still apply here.

## Shell completion

Every command that accepts arguments must offer shell completion via the cobra
command's `ValidArgsFunction`. Completion is part of the command, not an
optional extra: a new command, or a new positional argument on an existing one,
is not done until its arguments tab-complete.

Reuse the helpers in [`complete.go`](./complete.go) rather than hand-rolling
completion:

- Source handles: `completeHandle(maxVals, includeActive)`, or
  `completeHandleOrGroup` where a group is also valid.
- Keyring entry paths: `completeKeyringPath`.
- Other values (booleans, catalogs, schemas, drivers): the matching `complete*`
  helper.

Cap positional completion at the number of args the command takes (`maxVals`),
and set `includeActive` by whether the `@active` shortcut is a sensible target.

Commands with no positional arguments need no `ValidArgsFunction`. Arguments
whose values are inherently free-form are likewise exempt, since there is
nothing to enumerate: a new name the user invents, a literal value to store, an
arbitrary SQL or SLQ query. For example, `config keyring create PATH` takes no
completion for `PATH`, because it must be a new entry that does not yet exist;
suggesting existing paths would only offer names the command rejects.

Add a completion test with the command, using the `testComplete` helper (see
the existing `*_Completion` tests).
