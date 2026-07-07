# Release Configuration

## Root Resolution

Release root detection follows this order:

1. Find the Git repository root when possible.
2. If the Git root contains `.neko/release.config.json`, V2 is canonical.
3. V2 requires both `.neko/release.config.json` and `.neko/release.state.json`.
4. A nested `.release.neko.json` cannot override V2 in the Git root.
5. If no V2 config exists in the Git root, V1 root detection falls back to the nearest `.release.neko.json`.
6. A V1 file and V2 config together in the Git root are a configuration conflict.

## V2 Config

`.neko/release.config.json`:

```json
{
  "schemaVersion": 2,
  "units": [
    {
      "id": "default",
      "displayName": "My Project",
      "paths": ["**"],
      "workingDirectory": ".",
      "tagPrefix": "v",
      "executor": {
        "type": "goreleaser",
        "delivery": "local"
      }
    }
  ]
}
```

Rules:

- `schemaVersion` must be exactly `2`.
- Unknown JSON fields are rejected.
- `units` must not be empty.
- Unit IDs must match `[a-z][a-z0-9-]*`.
- `displayName` is optional.
- `paths` must not be empty. Patterns must be relative and stay inside the repository.
- `workingDirectory` defaults internally to `.`. It must be relative, stay inside the repository, and exist when validating against a real root.
- `tagPrefix` must be non-empty, relative, safe, unique, and not overlap another unit's prefix.
- Supported executors are `jreleaser`, `release-it`, and `goreleaser`.
- `delivery` defaults internally to `local`.
- Supported delivery values are `local` and `github-actions`.

Allowed tag prefix examples:

```text
v
api/v
web/v
mobile/v
```

## Multi-Unit Example

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
        "delivery": "local"
      }
    },
    {
      "id": "web",
      "displayName": "Web",
      "paths": ["web/**"],
      "workingDirectory": "web",
      "tagPrefix": "web/v",
      "executor": {
        "type": "release-it",
        "delivery": "github-actions"
      }
    }
  ]
}
```

`github-actions` is valid configuration data and is resolved as a remote delivery contract, but no GitHub Actions dispatch is performed yet. `local` is the only delivery mode with internal release preparation and Git coordination.

For public V2 non-dry-run releases, all executors remain blocked until publish-only adapters exist. Internally, `jreleaser` materializes `jreleaser.yml` before state write and before the Neko-owned release commit boundary. `goreleaser` currently uses a no-op materializer. `release-it` remains valid configuration and dry-run data, but V2 execution is blocked because no publish-only boundary exists.

See [Local delivery](local-delivery.md), [Release executors](executors.md), [Version materialization](version-materialization.md), [Local release transaction](local-release-transaction.md), and [Git release coordination](git-coordination.md).

## Migration

Classic V1 repositories with `.release.neko.json` directly in the Git root can be migrated with:

```bash
neko release migrate
```

The migration creates a single V2 unit named `default`, keeps the tag prefix as `v`, and archives the original V1 file as `.release.neko.json.v1.bak`. Nested V1 files are not migrated automatically.

See [V1 to V2 migration](migration-v1-to-v2.md).
