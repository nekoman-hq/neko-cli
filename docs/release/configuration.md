# Release Configuration and State

> **Audience:** Repository owners configuring Release V2 and operators selecting release units.
>
> **Purpose:** Define the authoritative V2 files, validation rules, unit selection, tag ownership, and plugin-unit metadata.

Configuration answers what may be released. State answers which version each
unit has reached. Neither file records tags, workflow runs, or publication
completion.

## Authority and root resolution

Release source detection uses the Git repository root when Git is available:

1. `.neko/release.config.json` at the Git root selects V2.
2. V2 requires both `.neko/release.config.json` and `.neko/release.state.json`.
3. A nested `.release.neko.json` cannot override root V2 files.
4. Without root V2 configuration, the nearest `.release.neko.json` selects V1.
5. Root V1 and V2 files together are rejected as mixed authority.

Normal loading and validation are read-only. `init`, `unit-add`, `migrate`, and
an executable release are the commands that write configuration or state under
their documented contracts.

## V2 configuration

`.neko/release.config.json` uses schema version 2:

```json
{
  "schemaVersion": 2,
  "units": [
    {
      "id": "api",
      "displayName": "API",
      "paths": ["api/**"],
      "workingDirectory": "api",
      "tagPrefix": "api/v",
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release-api.yml"
      }
    }
  ]
}
```

Unknown JSON fields are rejected. The configuration rules are:

- `schemaVersion` is exactly `2`.
- `units` is non-empty.
- Unit IDs match `[a-z][a-z0-9-]*` and are unique.
- `displayName` is optional.
- `paths` is non-empty; every glob is relative and remains inside the repository.
- `workingDirectory` defaults to `.`; it is relative, remains inside the repository, and exists during repository-aware validation.
- `tagPrefix` is non-empty, safe, unique, and non-overlapping.
- `executor.type` is `goreleaser`, `jreleaser`, or `release-it`.
- V2 `executor.delivery` is `github-actions`.
- `executor.workflow` is a repository-relative `.yml` or `.yaml` file directly below `.github/workflows/`; repository-aware validation requires it to exist.

`local` is not valid executable V2 delivery. It remains part of the V1
compatibility behavior described in [Compatibility](compatibility.md).

### Field ownership

| Field | Responsibility |
| --- | --- |
| `units[].id` | Stable selection key used by `--unit` |
| `units[].displayName` | Optional human label |
| `units[].paths` | Repository path globs owned by the unit |
| `units[].workingDirectory` | Root for executor requirement checks |
| `units[].tagPrefix` | Namespace used to derive the unit tag |
| `units[].executor.type` | Release tool selected by the consumer workflow |
| `units[].executor.delivery` | Handoff mode; V2 accepts `github-actions` |
| `units[].executor.workflow` | Consumer-owned workflow dispatched by Neko CLI |
| `units[].kind` and `units[].plugin` | Optional Neko CLI plugin registry metadata |

## State

`.neko/release.state.json` is the version source of truth:

```json
{
  "schemaVersion": 2,
  "units": {
    "api": { "version": "1.4.2" }
  }
}
```

State rules are:

- `schemaVersion` is exactly `2`.
- Unknown JSON fields are rejected.
- Every configured unit has exactly one state entry.
- State contains no unknown units.
- Every version is valid SemVer.
- Tags are derived from `tagPrefix + version`; tags are not stored in state.

Dry-run release commands, `validate`, `units`, `plan`, `pipeline`, `evidence`,
and Doctor do not write state. An executable V2 GitHub Actions release writes
the selected unit's next version atomically, reloads and validates the
repository, then includes state in the release commit.

Before the commit boundary, state and materialization transactions retain
byte-for-byte snapshots. A local failure in that interval restores their known
files. Once commit, tag, or push coordination starts, automatic restoration no
longer applies; see [Journals and Recovery](journals-and-recovery.md).

## Release units

A release unit is the V2 ownership boundary for paths, working directory, tag
namespace, executor, workflow, and version. V1 is normalized internally as one
virtual unit named `default`.

Selection rules:

- V1 accepts no `--unit` or `--unit default`; another ID is rejected.
- A one-unit V2 repository selects its only unit when `--unit` is omitted.
- A multi-unit V2 repository requires `--unit` for `patch`, `minor`, `major`, `plan`, `pipeline`, `history`, `contributors`, and `resume`.
- `ci-validate-context` always requires `--unit`.
- `github-workflow-init` accepts `--unit`; when multiple workflows are configured, `--path` can identify the workflow target.
- Doctor may inspect every unit. A unit filter retains shared-workflow checks required by other units.
- Evidence filters are inspection filters, not lifecycle unit selection.
- `validate` always validates the complete repository; `--unit` and `--show` only focus its human presentation.

The exact command and flag matrix is owned by the
[Release command reference](cli-reference.md).

## Normal and plugin units

The default `--kind release` represents services, applications, libraries,
SDKs, and CLIs. Normal units omit `kind` and plugin metadata in stored JSON.

`--kind plugin` is reserved for Neko CLI plugins and requires:

```text
plugin.name
plugin.manifest
plugin.assetPrefix
plugin.binaryName
```

Plugin unit IDs start with `plugin-`, their tag prefix is `<unit-id>/v`, and
their asset prefix equals the unit ID. The manifest path is repository-relative
and remains inside the repository.

`neko release plugin-index` reads plugin units, state, and manifests to produce
deterministic Plugin Index schema-v1 bytes. The index is output, not committed
repository source. Normal units are excluded.

## Tag ownership

Examples of valid prefixes are `v`, `api/v`, and `web/v`. A unit tag is the
prefix followed by its SemVer version. Exact parsing requires:

- the exact configured prefix;
- a valid release version after the prefix;
- no slash or unrelated suffix in the version portion.

Overlapping prefixes such as `api/` and `api/v` are rejected. Local tag lookup
does not fetch. Git globbing is only a prefilter; the selected unit's `TagSpec`
performs the authoritative parse.

## Creation and migration

`neko release init` creates one V2 configuration and state pair.
`neko release unit-add` appends one validated unit while preserving existing
units. They do not create V1 configuration, workflows, manifests, source trees,
or executor configuration.

`neko release migrate` converts a root V1 configuration into one V2 unit named
`default`, retains tag prefix `v`, and archives the source as
`.release.neko.json.v1.bak`. Nested V1 files are not migrated automatically.
See [V1 to V2 Migration](migration-v1-to-v2.md).

## Related documentation

- [Release lifecycle](lifecycle.md)
- [GitHub Actions delivery](github-actions-delivery.md)
- [Release examples](examples.md)
- [Release compatibility](compatibility.md)
