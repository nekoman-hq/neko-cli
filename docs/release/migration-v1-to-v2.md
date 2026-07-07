# V1 To V2 Migration

`neko release migrate` converts a classic V1 single-unit repository into the V2 file layout.

## Supported Source

The automatic migration is intentionally conservative. It supports only this shape:

```text
<git-root>/
├── .release.neko.json
└── ...
```

Nested V1 configs are not migrated automatically:

```text
<git-root>/
├── api/
│   └── .release.neko.json
└── ...
```

For nested or multi-project repositories, create an explicit V2 multi-unit configuration instead. The migration never guesses unit paths or tag prefixes.

## Command

```bash
neko release migrate
neko release migrate --dry-run
```

`--dry-run` is fully read-only. It does not create `.neko`, write config or state, create a journal, create a backup, remove V1, contact remotes, or start executors. It reports the planned paths, unit, version, executor, delivery, action list, and the exact V2 JSON content.

## File Transformation

Input:

```json
{
  "project-name": "example",
  "project-owner": "example-owner",
  "project-type": "backend",
  "release-system": "jreleaser",
  "version": "1.2.3"
}
```

Output:

```text
.neko/
├── release.config.json
└── release.state.json

.release.neko.json.v1.bak
```

`.neko/release.config.json`:

```json
{
  "schemaVersion": 2,
  "units": [
    {
      "id": "default",
      "displayName": "example",
      "paths": ["**"],
      "workingDirectory": ".",
      "tagPrefix": "v",
      "executor": {
        "type": "jreleaser",
        "delivery": "local"
      }
    }
  ]
}
```

`.neko/release.state.json`:

```json
{
  "schemaVersion": 2,
  "units": {
    "default": {
      "version": "1.2.3"
    }
  }
}
```

The migrated unit is always `default`. `paths` is always `["**"]`, `workingDirectory` is `.`, and `tagPrefix` is `v` so existing `vX.Y.Z` tags remain compatible. `project-owner` and `project-type` stay in the archived V1 backup and are not copied into V2 runtime state.

## Backup

The active V1 file is archived at the end:

```text
.release.neko.json.v1.bak
```

The backup is byte-identical to the original V1 file. The active `.release.neko.json` is removed from the Git root after success because V1 and V2 in the same root are a deliberate conflict.

If a backup already exists and differs from the active V1 file, migration aborts.

## Journal And Recovery

During a real migration, Neko writes an internal journal:

```text
.neko/release.migration.json
```

The journal stores source, config, state, and backup hashes plus the current stage. If a process stops after the journal, config write, state write, or V1 archive step, running `neko release migrate` again validates the hashes and safely finishes only missing or identical steps.

The journal is removed only after final V2 validation succeeds and the V1 file has been archived.

If the journal is invalid or existing files do not match the recorded hashes, migration aborts with a recovery error.

## Idempotency And Conflicts

Already migrated repositories are successful no-ops when V2 config and state exist and active V1 is gone.

Migration fails safely for:

- no release config;
- only V2 config or only V2 state;
- active V1 plus V2 files without a migration journal;
- differing backup content;
- invalid V1 version or executor;
- nested V1 without root V1.

## Not Migrated

This milestone does not implement:

- multi-unit detection;
- path splitting;
- `api/v...` or other unit-specific tag-prefix conversion;
- GitHub Actions delivery;
- real V2 release execution;
- V2 state updates from `patch`, `minor`, or `major`.
