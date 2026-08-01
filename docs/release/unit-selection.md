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

- `--unit` is required for unit-bound `patch`, `minor`, `major`, `plan`,
  `pipeline`, `history`, `contributors`, and `resume` commands.
- Errors list the available unit IDs.

Other commands use different selection contracts. `ci-validate-context`
always requires `--unit`. `github-workflow-init` accepts either `--unit` or
`--path` when more than one workflow target is configured. Doctor can inspect
all units and, when filtered, retains checks for every unit sharing the selected
workflow. Evidence filters are optional and do not perform lifecycle unit
selection.

`validate` is special:

- Without `--unit`, it validates the complete repository config and state.
- With `--unit`, it still validates everything and names the selection in the
  human summary; `--show` focuses the unit table and details on that selection.
- Without `--show`, human output is a concise responsive validation summary
  table and contains no unit-detail table.

Examples:

```bash
neko release validate
neko release validate --unit api --show
neko release history --unit api
neko release contributors --unit web
neko release plan --change patch --unit api
neko release pipeline --unit api
neko release patch --unit api --dry-run
neko release resume --unit api --dry-run
```

V2 non-dry-run release execution is active only for `delivery: github-actions`. Existing V2 configs with `delivery: local` are rejected during validation.
