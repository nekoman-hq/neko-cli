# Release Compatibility

## V1 Compatibility

`.release.neko.json` remains supported as the legacy compatibility format and keeps its existing fields:

```text
project-name
project-owner
project-type
release-system
version
```

V1 is normalized internally as a virtual single unit:

```text
id: default
paths: ["**"]
workingDirectory: "."
tagPrefix: "v"
delivery: local
executor.type: value from release-system
version: value from version
```

This internal model does not change the V1 file on disk.

## V2 Compatibility Boundary

When `.neko/release.config.json` exists in the Git root, it is authoritative for that repository. Nested V1 files are ignored for root selection. A V1 file in the Git root next to V2 config is rejected as a conflict so the CLI does not accidentally mix global V1 release behavior with unit-aware V2 data.

## Validate

`neko release validate` supports both formats:

- V1: existing validation and public JSON behavior remains compatible. Human
  `--show` presents the normalized `default` unit and legacy project details
  without a V2 state path.
- V2: config and state are strictly decoded and validated together. Default
  human output is a concise summary; `--show` adds a responsive unit table and
  complete structured unit details with one path per line. Plugin-specific
  fields appear only for plugin units. `--unit` focuses displayed details while
  still validating the complete repository.

Both formats use the `Release Configuration Validation` human title. The
presentation declarations do not alter public `--output json`, its established
`data.items` ordering, raw JSON behavior, error codes, or legacy exit behavior.

## V2 Commands

V2 now supports unit-specific read-only commands:

```bash
neko release history --unit api
neko release contributors --unit web
neko release patch --unit api --dry-run
```

Dry-run planning does not write state, create tags, commit, push, publish, run executors, or fetch remotes.

Dry-run planning now also builds the schema-neutral execution context. That context resolves the absolute repository root, selected unit root, tag spec, executor capabilities, and delivery contract without mutating files.

`github-actions` is a valid V2 delivery mode only when `workflow` uses canonical `.github/workflows/<file>.yml|yaml` form. Real repository validation requires the workflow file to exist and remain inside `.github/workflows/`.

V2 GitHub Actions non-dry-run public release commands are active. They prepare known release files, build a durable release execution journal, coordinate Neko-owned materialization, state update, commit, unit tag, push, build the GitHub Actions dispatch request and journal identity, resolve GitHub.com repository targets, and dispatch the workflow. V2 local delivery is unsupported and rejected by validation.

Execution and dispatch journals never use V1 `project-owner` or `project-name`. Dispatch derives owner and repository only from the selected V2 Git remote and currently supports GitHub.com remotes only.

## Migration

`neko release migrate` converts only root V1 single-unit repositories to V2. It writes `.neko/release.config.json`, `.neko/release.state.json`, and archives `.release.neko.json` to `.release.neko.json.v1.bak`.

`neko release migrate --dry-run` is read-only and shows the planned V2 JSON content.

Nested V1 configs are rejected because the CLI cannot safely infer whether they represent the whole repository or one future unit.

## Not Yet Available

The following remain future work:

- Public V2 local executor execution.
- Public standalone dispatch and retry commands.

## Dry-Run And Rollback Safety

V1 dry-run release commands are read-only and do not fetch remotes, write config, update executor files, run executors, commit, tag, push, publish, or rollback.

Rollback only runs after a mutating release step has been recorded. This prevents planning and guard errors from reaching destructive Git rollback operations.

V2 recovery is more conservative: before commit/tag work starts, materialized files and `.neko/release.state.json` may be restored from snapshots and known release files may be unstaged. After commit/tag/remote work starts, V2 does not call `git reset --hard`, `git clean -fd`, remote tag deletion, or GitHub release deletion.
