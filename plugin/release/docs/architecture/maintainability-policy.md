# Release Plugin Maintainability Policy

## Scope

This policy applies to new or materially changed Release Plugin production code.
Historical debt is not made a blanket failure, but a touched function or file
must not add unexplained complexity or mix separable responsibilities.

## File and function cohesion

Each production file owns one subject. A split or move must clarify ownership,
dependency direction, reuse, read-only versus mutation behavior, command versus
neutral policy, lifecycle versus presentation, or supported API versus internal
implementation.

Review a function at roughly 60 lines or cognitive complexity 20-25. New or
materially changed functions have an 80-line ceiling. Review nested control flow
at complexity 5. Do not split a cohesive function merely to satisfy a metric.

Documented exceptions are limited to:

- the authoritative straight-line release coordinator;
- cohesive safety transactions whose local ordering is part of their auditability;
- static presentation or diagnostic mappings where a split obscures the mapping.

The `.golangci.yml` exclusions name the existing instances. Adding an exclusion
requires a rationale in this document and an architecture review. Changed-code
review uses:

```bash
golangci-lint run --config .golangci.yml --new-from-rev <review-baseline> ./...
```

The complete repository lint remains required before commit.

## Current metric exceptions

| Area | Reason |
| --- | --- |
| Doctor consumer, credential, installation, publication, and workflow inspection | Cohesive diagnostic/static contract mappings |
| init options and release progress rendering | Static presentation mappings |
| V2 pair persistence and config/state validation | Ordered safety transactions with rollback/path checks |
| migration plan and recovery adapters | Ordered migration safety decisions |
| execution-journal updates, V1 evidence validation, and V1 rollback | Authoritative or compatibility safety transitions |
| JReleaser V1 initialization | Cohesive compatibility transaction whose order is characterized |
| dispatch response parsing and repository/output path decisions | Focused protocol/path-safety decision trees |

Tests are excluded from production flow metrics. Large test tables and AST
walkers are reviewed for fixture ownership and behavioral family instead.

## Root package review

`plugin/release` may only decode/encode the plugin protocol, resolve a root,
construct fresh V1 executors, and route commands. `pkg/release` may only own
supported release contracts, source selection, release planning/orchestration,
journals/recovery, and public compatibility facades.

Before adding a root production file, reviewers must answer:

1. Which documented root responsibility requires it?
2. Why can a focused internal capability or leaf fact package not own it?
3. Does it add an export, lifecycle policy, mutable global, or infrastructure edge?
4. Do import-direction and compatibility guards cover the new edge?

Never add a maximum-file-count test. Enforce responsibility and dependency
direction instead.

## Dependency and naming rules

- Shared tool facts do not import Doctor, HTTP, journals, presentation, or root
  release orchestration.
- Extracted command capabilities do not import `pkg/release`.
- Pipeline projection imports neither Doctor nor HTTP. Root Pipeline
  composition may adapt neutral Doctor facts but must not construct a second
  GitHub client or token resolver; explicit remote mode reuses Doctor's existing
  bounded GET-only boundary.
- Command handlers parse and map; they do not call other command handlers.
- Response mapping does not access Git, HTTP, journal stores, or recovery policy.
- Read-only capabilities do not receive mutation ports.
- New mutable package globals are prohibited.
- Verification fact IDs derive only from immutable neutral identity fields;
  evidence messages, timestamps, array positions, absolute paths, credentials,
  and presentation are prohibited inputs.
- Prefer subject-qualified names. Reserve `dispatch` for GitHub workflow dispatch.
- Do not introduce generic managers, engines, processors, registries, context
  bags, or pipeline abstractions for the release lifecycle.

## Compatibility wrappers

A retained wrapper must preserve a supported or explicitly deprecated surface,
delegate directly to one canonical owner, and contain no new policy. Removal
requires consumer evidence, a documented replacement or support-ending decision,
and focused characterization tests. See
[compatibility-notes.md](compatibility-notes.md) and
[v1-compatibility-policy.md](v1-compatibility-policy.md).

Every `*_compatibility.go` production file is an explicit quarantine. It may
contain only classified legacy, deprecated, alias, wrapper, or forwarding
declarations. Active planning, orchestration, policy, or adapters must have an
active subject-qualified owner. The architecture guard discovers every matching
production file below `plugin/release/pkg` across package boundaries and requires
an explicit compatibility classification on every top-level declaration; methods
inherit the classification of their receiver type. It does not rely on a list of
known filenames.

## Test and isolation policy

- Keep tool fixtures with the owning tool package and lifecycle tests with the
  authoritative lifecycle.
- Keep supported public API checks in external-package tests.
- Use `t.TempDir` or neutral synthetic paths; never hard-code developer paths.
- Restore cwd and environment changes in tests.
- Never read real credentials or call real external APIs.
- Loopback HTTP tests may only use listeners created by the tests themselves.
- Exercise two repository roots to prevent cwd, environment, registry, or
  executor leakage.
- Production composition constructs fresh V1 executor instances and does not use
  the legacy mutable registry.
