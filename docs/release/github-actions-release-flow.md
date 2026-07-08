# GitHub Actions Release Flow

Milestone 5C3B activates public V2 releases for `delivery: github-actions`.

```bash
neko release patch --unit api
neko release minor --unit web
neko release major --unit mobile
```

V2 local delivery remains blocked. V1 release behavior is unchanged.

## Transaction Order

The public V2 GitHub Actions release path runs in this order:

```text
V2 plan
V2 preflight
ReleaseExecutionJournal
version materialization
V2 state update
targeted staging
release commit
unit tag
prepared DispatchJournal
push release commit
push unit tag
GitHub Actions workflow dispatch
handoff-ready
```

The release commit contains exactly `.neko/release.state.json` plus materialized files marked `RequiredForReleaseCommit`. For Nekocli plugin units, the materialized file is the selected plugin manifest: `plugin/release/manifest.json` for `plugin-release`, or `plugin/ui/manifest.json` for `plugin-ui`. Journals live under the Git common directory and never enter the release commit.

## Token Preflight

Non-dry-run GitHub Actions delivery requires `GITHUB_TOKEN` with repository Actions write permission. The token is resolved before any materialization, state write, journal write, commit, tag, push, or dispatch. It is never persisted, logged, or rendered.

Dry-run does not require or resolve a token.

## Dispatch Outcomes

`accepted` marks the execution journal `handoff-ready`. GitHub Actions owns build and publish from the pushed tag.

For Nekocli's own units, `.github/workflows/release-neko-cli.yml`, `.github/workflows/release-plugin-release.yml`, and `.github/workflows/release-plugin-ui.yml` accept the canonical dispatch inputs `unit`, `version`, `tag`, and `release_sha`. Each workflow checks out `inputs.tag`, verifies that the unit and tag prefix match the configured unit, and verifies that both checked-out `HEAD` and the tag resolve to `release_sha`. It also validates `.neko/release.state.json`, the V2 unit config, and the selected plugin manifest for plugin units before publishing. Workflows do not calculate versions, commit, tag, push, or modify tracked repository files.

Publishing uses dedicated GoReleaser configs: `.goreleaser.cli.yaml`, `.goreleaser.plugin-release.yaml`, and `.goreleaser.plugin-ui.yaml`. Each config contains only that unit's build/archive/release definition. The root `.goreleaser.yaml` remains multi-artifact and is not used by production V2 publishing workflows.

`rejected` keeps the execution journal at `tag-pushed`, preserves the dispatch journal rejection, and does not roll back Git state.

`unknown` also keeps the execution journal at `tag-pushed`. Neko CLI does not automatically retry uncertain dispatch or push outcomes.

## Resume

```bash
neko release resume --unit api
neko release resume --unit api --dry-run
```

Resume continues only an existing unresolved V2 GitHub Actions execution journal. It never calculates a new version, chooses a new tag, creates a new release intent, or blindly retries uncertain push or dispatch outcomes.

`resume --dry-run` performs only read-only recovery assessment and does not require `GITHUB_TOKEN`.

Verbose and `--describe` output includes the selected unit, version, tag, workflow, release commit SHA, execution journal path, dispatch journal path, execution state, dispatch state, dispatch run URL when resolvable, and recovery guidance. Unknown dispatch or ambiguous push outcomes must not be retried blindly; inspect the journals and use `neko release resume --unit <unit> --dry-run` first.

## Boundaries

No local executor publishes V2 GitHub Actions releases. GoReleaser, JReleaser, or release-it run later inside the configured GitHub workflow.

No automatic `git reset --hard`, `git clean -fd`, local tag deletion, remote deletion, GitHub Release deletion, retry, or rollback occurs after commit, tag, push, or dispatch uncertainty.
