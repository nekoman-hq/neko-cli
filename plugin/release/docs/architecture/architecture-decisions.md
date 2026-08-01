# Release Plugin Architecture Decisions

## Purpose and authority

This document records current unresolved architecture boundaries and points to
the current owners of adopted decisions. It is not a roadmap or implementation
status tracker. Detailed behavior lives in [current-state.md](current-state.md),
package direction lives in [package-ownership.md](package-ownership.md), and
the closed evolution sequence is preserved in
[history entry 003](../history/003-post-refactor-architecture-evolution.md).

Every future implementation remains subject to `plugin/release/RULES.md` and
[maintainability-policy.md](maintainability-policy.md).

## Adopted boundaries

The current implementation owns these established decisions:

- V1 compensation and V2 pair/migration recovery use distinct typed evidence
  and fail closed across uncertain effects.
- Evidence inspection is read-only; completed-evidence archival is a separate
  guarded mutation.
- Production command composition uses one explicit repository root while cwd
  facades remain compatibility-only.
- Active V2 release progress is typed and presentation-neutral below its
  terminal adapter.
- V2 local delivery is unsupported; GitHub Actions is the executable V2
  delivery path.
- Workflow scaffolding is focused, create-only, and cannot update or merge an
  existing workflow.
- Generated-output path policy remains output-family-specific rather than a
  universal path manager.
- Plan, Units, default Doctor, default Pipeline, Validate, History,
  Contributors, Evidence query, and Context Validation remain local,
  read-only, offline, and token-free.
- Explicit Doctor and Pipeline remote verification reuse one bounded GET-only
  observation boundary and cannot mutate lifecycle state.

The authoritative details and current limitations for each boundary are in
[current-state.md](current-state.md). The active compatibility decisions are in
[v1-compatibility-policy.md](v1-compatibility-policy.md).

## Pending architecture decisions

Durable workflow-run and publication-completion state remain separate future
capabilities. Current Pipeline verification may expose exact Doctor-owned
workflow, Actions-setting, variable, release, tag, and asset facts, but it must
not infer runtime progress, remote freshness, safe retry, or publication
completion from those facts or from accepted dispatch handoff evidence.

Release V2 bootstrap planning remains product capability planning rather than
an architecture-refactor continuation. Its current ownership and genuinely
future capabilities are maintained in the
[Release V2 Bootstrap Product Boundary](../../../../docs/release/bootstrap-product-boundary.md).

No pending decision authorizes journal repair, schema migration, blind retry,
remote mutation, V2 local execution, or a generic pipeline/state-machine
framework.
