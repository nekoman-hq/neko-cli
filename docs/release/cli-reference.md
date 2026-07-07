# Release CLI Reference

## V1

V1 uses `.release.neko.json` and supports the existing release commands.

```bash
neko release patch --dry-run
neko release patch
neko release minor
neko release major
neko release history
neko release contributors
neko release validate --show
```

`--unit default` is accepted for V1. Any other unit is rejected.

## V2

V2 uses:

```text
.neko/release.config.json
.neko/release.state.json
```

Supported now:

```bash
neko release migrate
neko release migrate --dry-run
neko release validate
neko release validate --unit api --show
neko release history --unit api
neko release contributors --unit web
neko release patch --unit api --dry-run
neko release minor --unit api --dry-run
neko release major --unit api --dry-run
```

`patch`, `minor`, and `major` in V2 are planning-only in this milestone. Dry-run output includes:

```text
unit
currentVersion
nextVersion
tag
executor
delivery
workingDirectory
```

Blocked until a later milestone:

```bash
neko release patch --unit api
neko release minor --unit api
neko release major --unit api
```

These commands return a clear error because V2 release execution, state persistence, executor context, and delivery execution are not implemented yet.

## Unit Flag

`--unit <unit-id>` is available on:

```text
patch
minor
major
history
contributors
validate
```

It is required for unit-bound commands when a V2 repository defines multiple units.

## Migrate

`migrate` has one flag:

```text
--dry-run
```

It migrates only `.release.neko.json` in the Git root to a V2 `default` unit. It does not migrate nested V1 files, infer multiple units, change tag prefixes, or enable real V2 releases.
