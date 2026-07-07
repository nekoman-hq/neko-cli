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
- Tags are not stored. A future V2 release path derives tags from `tagPrefix + version`.

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

Normal V2 loading never writes state. The repository includes an atomic JSON writer for future migration and state-update commands, but those commands are not part of this milestone.
