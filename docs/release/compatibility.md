# Release Compatibility

> **Audience:** Repository owners choosing between Release V1 compatibility and the V2 architecture.
>
> **Purpose:** Define source selection, supported behavior, migration, and unsupported combinations without duplicating command details.

V1 is supported compatibility, not the canonical architecture for new setup.
V2 is the canonical architecture for new repositories. Both resolve into shared
internal release concepts, but their configuration authority and execution
contracts remain distinct.

## Authority selection

| Repository files | Selected source | Result |
| --- | --- | --- |
| Root `.neko/release.config.json` and `.neko/release.state.json` | V2 | Valid after schema and repository checks |
| Nearest `.release.neko.json` with no root V2 config | V1 | Supported compatibility |
| Root V1 file together with root V2 config | none | Rejected conflict |
| V2 config without V2 state, or state without config | none | Rejected incomplete source |

A nested V1 file cannot override V2 at the Git root. Mixed V1 and V2 authority
is rejected; mixed V1 and V2 authority is rejected before release planning or
mutation.

## V1 compatibility

V1 uses `.release.neko.json` and is normalized as one virtual unit named
`default`. It supports the established `goreleaser`, `jreleaser`, and
`release-it` adapters and their legacy local release behavior.

V1 rules include:

- no `--unit` or `--unit default` selects the virtual unit;
- another unit ID is rejected;
- tag prefix is `v`;
- V1 loader, version, requirement, executor, Git, and rollback contracts remain compatibility-owned;
- V1 exported symbols remain governed by the [V1 compatibility policy](../../plugin/release/docs/architecture/v1-compatibility-policy.md).

V1 is not extended with V2 multi-unit configuration, V2 central state, or V2
GitHub Actions journal semantics.

## V2 canonical architecture

V2 uses:

```text
.neko/release.config.json
.neko/release.state.json
```

It supports one or more release units, unit-specific paths and working
directories, independent tag prefixes, explicit executors, optional Neko CLI
plugin metadata, and one consumer workflow per configured delivery target.

For executable V2 releases:

- `delivery` is `github-actions`;
- Neko CLI owns version materialization, state, release commit, unit tag, push order, journals, and dispatch;
- the consumer workflow owns build and publication;
- local executor execution is unsupported;
- an accepted dispatch is handoff evidence, not publication completion.

See [Configuration and State](configuration.md) and
[Release Lifecycle](lifecycle.md).

## Shared commands and source-specific behavior

`validate`, `units`, `plan`, release dry-run, `history`, `contributors`, and
Doctor operate through normalized repository facts while preserving the
selected source's constraints. V2-only commands reject V1 clearly:

- `unit-add`;
- `pipeline`;
- `ci-validate-context`;
- `github-workflow-init`;
- `plugin-index`;
- `resume`;
- `evidence` and `evidence archive`.

The complete availability, flag, output, and exit matrix is in the
[Release command reference](cli-reference.md).

## Migration

`neko release migrate` is the supported transition from a root V1 repository.
It plans or writes one V2 `default` unit and archives the V1 source as
`.release.neko.json.v1.bak`. It does not migrate nested V1 files or create a
consumer workflow. See [V1 to V2 Migration](migration-v1-to-v2.md).

## Unsupported combinations

- V1 and V2 cannot remain active at the Git root.
- V2 configuration without matching V2 state is invalid.
- V2 `delivery: local` is invalid for executable releases.
- Public V2 local non-dry-run execution is unavailable.
- GitHub Enterprise remotes are not dispatch targets.
- No standalone workflow dispatch or retry command exists.
- Workflow-run and publication-completion state are not stored as Release state.

Unsupported behavior fails before the affected mutation boundary. It is not
silently interpreted as another mode.

## Compatibility maintenance

V1 public and package contracts are not removed merely because V2 is canonical.
Changes follow the repository's deprecation, downstream-audit, replacement,
and removal gates. Implementation placement and package policy are described in
[Release package compatibility](../../plugin/release/docs/architecture/compatibility-notes.md).
