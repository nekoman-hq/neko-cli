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
neko release patch --unit api
neko release minor --unit api
neko release major --unit api
```

V2 non-dry-run local release is enabled for units using `delivery: local` with `goreleaser` or `jreleaser`. Dry-run output includes:

```text
unit
currentVersion
nextVersion
tag
executor
delivery
workingDirectory
unitRoot
stateChange
materializedFiles
ownership
stateCommitGuarantee
```

Blocked:

- `delivery: github-actions` returns `github-actions delivery is configured but not implemented yet`.
- V2 local `release-it` is blocked because the root `.neko/release.state.json` cannot currently be guaranteed in release-it's own release commit.

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
