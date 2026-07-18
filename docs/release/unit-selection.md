# Unit Selection

V2 uses release units as the canonical release boundary. V1 is still supported as a legacy compatibility mode and is normalized internally as one virtual unit:

```text
default
```

## Rules

V1:

- No `--unit`: selects `default`.
- `--unit default`: allowed.
- Any other unit: rejected.

V2 with one unit:

- No `--unit`: selects the only unit.
- Matching `--unit`: allowed.
- Unknown unit: rejected.

V2 with multiple units:

- `--unit` is required for `patch`, `minor`, `major`, `history`, `contributors`, and `resume`.
- Errors list the available unit IDs.

`validate` is special:

- Without `--unit`, it validates the complete repository config and state.
- With `--unit`, it still validates everything, but `--show` focuses the selected unit.

Examples:

```bash
neko release validate
neko release validate --unit api --show
neko release history --unit api
neko release contributors --unit web
neko release patch --unit api --dry-run
neko release resume --unit api --dry-run
```

V2 non-dry-run release execution is active only for `delivery: github-actions`. Existing V2 configs with `delivery: local` are rejected during validation.
