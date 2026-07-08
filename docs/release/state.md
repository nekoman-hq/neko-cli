# Release State

V2 state is stored in `.neko/release.state.json`.

```json
{
  "schemaVersion": 2,
  "units": {
    "default": {
      "version": "2.2.4"
    }
  }
}
```

Rules:

- `schemaVersion` must be exactly `2`.
- Unknown JSON fields are rejected.
- Every configured unit must have a state entry.
- State must not contain unknown units.
- Versions must be valid SemVer.
- Tags are not stored. Release tags are derived from `tagPrefix + version`.

Multi-unit state:

```json
{
  "schemaVersion": 2,
  "units": {
    "api": {
      "version": "1.4.2"
    },
    "web": {
      "version": "3.1.0"
    }
  }
}
```

Normal V2 loading never writes state. V2 dry-run commands are read-only and do not write state.

`neko release migrate` writes V2 state once when converting a root V1 repository. Public V2 GitHub Actions non-dry-run releases persist the selected unit's next version through the atomic writer, then reload and validate the real V2 repository before the Neko-owned release commit boundary. Public V2 local non-dry-run releases remain blocked.

Before internal V2 preparation writes state, it captures a byte-for-byte snapshot of `.neko/release.state.json`. If a local failure happens before commit/tag work starts, state and materialized version files are restored from snapshots.

The Git release coordinator commits exactly `.neko/release.state.json` plus required materialization files. It verifies that the committed state contains the selected unit's next version.
