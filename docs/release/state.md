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

Normal V2 loading never writes state. V2 local release transactions write state only after preflight and version materialization succeed, and only for the selected unit.

`neko release migrate` writes V2 state once when converting a root V1 repository. V2 local `patch`, `minor`, and `major` persist the selected unit's next version through the atomic writer, then reload and validate the real V2 repository.

Before a V2 local release writes state, it captures a byte-for-byte snapshot of `.neko/release.state.json`. If a local failure happens before commit/tag work starts, state and materialized version files are restored from snapshots.
