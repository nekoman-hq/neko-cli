# Release CLI Reference
## General

```bash
neko release init --executor goreleaser --delivery local
neko release init --unit plugin-release --kind plugin --plugin-name release --plugin-manifest plugin/release/manifest.json --plugin-asset-prefix plugin-release --plugin-binary-name plugin-release --executor goreleaser --delivery github-actions --workflow .github/workflows/release-plugin-release.yml --tag-prefix plugin-release/v
neko release unit-add --unit api --executor goreleaser --delivery github-actions --workflow .github/workflows/release-api.yml --tag-prefix api/v --paths "apps/api/**"
neko release init-options
```

`init` creates V2 `.neko/release.config.json` and `.neko/release.state.json` files for one release unit. Use `--kind release` or omit `--kind` for a normal unit. Use `--kind plugin` plus `--plugin-name`, `--plugin-manifest`, `--plugin-asset-prefix`, and `--plugin-binary-name` to initialize one plugin unit. Plugin unit ids must start with `plugin-`, plugin tag prefixes must be `<unit-id>/v`, and plugin asset prefixes must match the unit id. `init` no longer creates `.release.neko.json` or initializes executor-specific tool files. Existing V1 repositories should use `neko release migrate`. `github-actions` delivery requires `--workflow .github/workflows/<file>.yml|yaml`; workflow template generation and executor scaffolding are not implemented yet.

`unit-add` appends one unit to existing V2 config/state. It uses the same unit and plugin metadata flags as `init`, requires both `.neko/release.config.json` and `.neko/release.state.json`, preserves existing units, and never overwrites an existing unit. It does not generate workflow files, GoReleaser config files, plugin manifests, source directories, or any release artifacts.

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
neko release resume --unit api
neko release resume --unit api --dry-run
neko release plugin-index
neko release plugin-index --check
neko release plugin-index --output /tmp/plugin-index.json
```

See [Release V2 Examples](examples.md) for copy-ready init, unit-add, release, plugin-index, and plugin install/update flows.

V2 non-dry-run release commands are active only for `delivery: github-actions`. V2 local delivery remains blocked. Dry-run output includes:

```text
unit
currentVersion
nextVersion
tag
executor
delivery
workflow
dispatch
dispatchRef
dispatchInputs
journalIdentity
journalLocation
workingDirectory
unitRoot
stateChange
materializedFiles
knownReleaseFiles
plannedReleaseCommit
plannedTag
plannedPushOrder
toolOwnership
v2GitOwnership
stateCommitGuarantee
executorStart
```

Blocked:

- V2 local non-dry-run release commands return `V2 local release execution is not available yet.`
- Execution journals and dispatch journals are not written by dry-run.
- V2 local `release-it` remains blocked because no publish-only boundary exists.

For `delivery: github-actions`, V2 config must include `workflow: ".github/workflows/<file>.yml"` or `workflow: ".github/workflows/<file>.yaml"`. `neko release validate --show` displays the workflow only after repository-aware validation confirms that the file exists and stays inside `.github/workflows/`.

The execution journal records V2 release phases and recovery evidence under the Git common directory. The dispatch contract targets GitHub.com remotes only, uses `GITHUB_TOKEN` with repository Actions write permission, sends the existing unit tag as `ref`, and sends exactly four inputs: `unit`, `version`, `tag`, and `release_sha`. No public standalone dispatch or retry command exists.

`neko release plugin-index` generates the public plugin registry index from V2 plugin units, `.neko/release.state.json`, and plugin manifests. The generated `plugin-index.json` is not committed as source and is not published by this command. Plugin release workflows publish or replace it as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases. Runtime `neko plugin available`, `install`, and `update` use that asset as the source of truth and do not use `/releases/latest` or release-prefix fallback discovery for plugin discovery.

## Resume

`resume` continues only an existing unresolved V2 GitHub Actions execution journal. It never calculates a new version, chooses a new tag, or blindly retries uncertain push or dispatch outcomes.

```bash
neko release resume --unit api
neko release resume --unit api --dry-run
```

## Unit Flag

`--unit <unit-id>` is available on:

```text
patch
minor
major
history
contributors
validate
resume
```

It is required for unit-bound commands when a V2 repository defines multiple units.

## Migrate

`migrate` has one flag:

```text
--dry-run
```

It migrates only `.release.neko.json` in the Git root to a V2 `default` unit. It does not migrate nested V1 files, infer multiple units, change tag prefixes, or run a release.
