# Release Compatibility

## V1 Compatibility

`.release.neko.json` remains supported and keeps its existing fields:

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

- V1: existing validation behavior remains compatible.
- V2: config and state are strictly decoded, validated together, and `--show` displays schema type, units, versions, working directories, tag prefixes, executor, delivery, and paths.

## Not Yet Available

The following remain future work:

- V1-to-V2 migration.
- `--unit`.
- V2 release execution.
- Unit-specific tags, history, contributors, and version bumps.
- GitHub Actions delivery execution.

## Dry-Run And Rollback Safety

V1 dry-run release commands are read-only and do not fetch remotes, write config, update executor files, run executors, commit, tag, push, publish, or rollback.

Rollback only runs after a mutating release step has been recorded. This prevents planning and guard errors from reaching destructive Git rollback operations.
