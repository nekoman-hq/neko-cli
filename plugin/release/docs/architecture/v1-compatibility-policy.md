# V1 Compatibility Policy

> **Audience:** Maintainers changing exported Release V1 symbols, adapters, facades, or compatibility files.
>
> **Purpose:** Define supported, deprecated, and compatibility-only V1 surfaces together with their replacement and removal gates.

V1 remains supported compatibility. V2 is canonical for new repositories, but
that status does not by itself authorize removal or behavior change in V1.
Public Go symbols may have downstream consumers that cannot be discovered from
this repository.

## Classification vocabulary

| Classification | Meaning |
| --- | --- |
| Supported | Used by production or intentionally maintained as public V1 behavior. |
| Deprecated | A precise supported replacement exists and the Go declaration carries the matching `Deprecated:` comment. |
| Compatibility-only | Retained because behavior or downstream use lacks an exact replacement/audit; production must not add dependencies on it. |

“No repository consumer” never proves “no external consumer.” Removal requires
all gates in this policy and separate authorization for the compatibility
change.

## Production composition

The plugin executable constructs fresh GoReleaser, JReleaser, and release-it
`V1Executor` values and calls `HandleReleaseWithV1ExecutorsAt`. Production does
not use the mutable registry or blank-import registration.

The registry-backed `HandleRelease` path remains supported because its exported
semantics may be consumed externally. It is confined to compatibility
composition and must not become the preferred example.

## Supported surfaces

| Symbol or family | Reason |
| --- | --- |
| `HandleRelease` | Original public registry-backed command entry; external semantics are unknown. |
| `HandleReleaseWithV1Executors*` | Canonical production V1 entry with caller-owned fixed executors. |
| `PlanV1Release` | Explicit V1 planning boundary. |
| `Preflight` | Preserves observable fatal JSON/exit behavior; no identical public non-fatal replacement exists. |
| `Tool`, `ToolBase` | Legacy init/file/requirement surface is broader than `V1Executor`. |
| Executor `Init` methods | Legacy tool initialization has no identical public replacement. |
| Executor `Run` and `Rollback` methods | Required by active fixed `V1Executor` composition. |
| `config.V1ConfigExistsAt`, `V1LoadConfigAt`, `V1SaveConfigAt` | Explicit-root/path V1 config operations used by compatibility behavior. |
| `EnsureVersionIsValid` | Pure V1 comparison helper without hidden evidence refresh. |
| Migration and Plugin Index programmatic APIs | Cohesive APIs, not V1 lifecycle compatibility debt. |
| Production journal, dispatch, and Git coordinator constructors | Intentional system-default composition seams. |

Supported does not mean preferred for new V2 code. New production dependencies
must use the narrow canonical owner described in
[Package Ownership](package-ownership.md).

## Deprecated surfaces

| Symbol or family | Replacement |
| --- | --- |
| `Service`, `NewReleaseService*`, `Service.Run` | `HandleReleaseWithV1Executors` with explicit executors |
| `Service.GetNewVersion` | `PlanV1Release` with explicit latest-tag evidence |
| `Register`, `Get`, and `pkg/release/tool` registration import | Caller-owned executor selection and `HandleReleaseWithV1Executors` |
| Executor `Execute` and `Release` methods | `Run` with `V1ExecutorRequest` |
| Executor `RevertRelease` methods | `Rollback` |
| `BuildReleaseExecutionContext` | `BuildV2ReleaseExecutionContext` for V2 or `PlanV1Release` for V1 |
| `config.V1Exists`, `V1LoadConfig`, `V1SaveConfig` | The matching explicit-root/path V1 config function |
| `VersionGuard`, `VersionGuardWithOptions` | `PlanV1Release` with caller-owned tag evidence |
| `ReleaseTransaction`, constructor, and `Execute` | No executable local replacement; V2 uses GitHub Actions delivery |

The source comments are part of the compatibility contract. A deprecated
wrapper delegates directly to the replacement and contains no new policy,
parsing, token resolution, Git/filesystem mutation, rollback decision, or
workflow selection.

## Compatibility-only implementation

The mutable registry map exists solely behind `Register`, `Get`,
registry-backed `HandleRelease`, and `Service`. Production composition remains
off that path. Version evidence globals are confined to V1 version-guard and
registry adapters. Cwd helpers remain direct delegates to explicit-root/path
functions.

`*_compatibility.go` files are quarantine boundaries. Every top-level
declaration is classified as legacy, deprecated, alias, wrapper, or forwarder;
methods inherit the classification of their receiver. Those files contain no
active plan, policy, orchestration, state transition, process adapter, or
infrastructure owner.

## Behavior that remains stable

- V1 command names, flags, defaults, and single virtual `default` unit.
- GoReleaser, JReleaser, and release-it selection and requirement checks.
- Executor invocation, environment mapping, Git/GitHub effects, and rollback/compensation evidence.
- Fatal V1 preflight/output behavior.
- Registry overwrite/error behavior on the compatibility path.
- Cwd facade delegation and canonical config bytes/modes.
- Two-pass version evidence behavior and pure comparison behavior.
- Response schema, item order, errors, logs, renderer declarations, and exits.

V1 compatibility does not import V2 central state, V2 workflow dispatch,
execution journals, or dispatch journals into the legacy lifecycle.

## Removal gates

Removal of a compatibility family requires all of the following:

1. Search production, tests, docs, examples, generators, registration side effects, and reflection/string references.
2. Record available downstream/import evidence and explicitly retain unknown external-consumer risk.
3. Provide and document an exact supported replacement, or approve an explicit support-ending decision.
4. Move repository consumers and tests to the canonical boundary first.
5. Complete an appropriate deprecation window for exported surfaces.
6. Preserve focused characterization of the old behavior through the authorized removal commit.
7. Confirm production has no dependency on the family.
8. Add or retain a guard that prevents accidental reintroduction of the superseded implementation.

No scheduled cleanup, V2 feature, file move, or lack of internal references
waives these gates.

## Related controls

- [Compatibility Notes](compatibility-notes.md)
- [Current Architecture](current-state.md)
- [Maintainability Policy](maintainability-policy.md)
- [Public Release Compatibility](../../../../docs/release/compatibility.md)
