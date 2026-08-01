# Release Plugin Refactor History

> **Sequence:** 001
> **Title:** Release Plugin Behavior-Preserving Refactor
> **Status:** completed
> **Created:** 2026-07-14
> **Completed or superseded:** 2026-07-15
> **Predecessor:** None
> **Successor:** [002 — Post-Refactor Architecture Review](002-post-refactor-architecture-review.md)
> **Current references:** [Current architecture](../architecture/current-state.md), [package ownership](../architecture/package-ownership.md), [maintainability policy](../architecture/maintainability-policy.md)
> **Original source:** `plugin/release/docs/architecture/refactor-plan.md`; later condensed as `plugin/release/docs/architecture/refactor-history.md`

> This is a historical record, not the current product or architecture source.
> It preserves the completed refactor boundary and its original limitations.

## Purpose

This document records the closed behavior-preserving Release Plugin refactor as durable architecture history. It avoids project-management labels and preserves the technical boundaries that were established.

Current runtime behavior is authoritative in [current-state.md](../architecture/current-state.md). Current dependency direction and remaining debt are maintained in [package-ownership.md](../architecture/package-ownership.md). Current architecture decisions are described in [architecture-decisions.md](../architecture/architecture-decisions.md).

## Git chronology and provenance

- Created on 2026-07-14 in `a47df6a` as
  `plugin/release/docs/architecture/refactor-plan.md`.
- Revised on 2026-07-14 as command presentation, release orchestration, resume,
  initialization, and adapter boundaries were characterized and extracted.
- Revised and completed on 2026-07-15 as query, migration, and V1 compatibility
  boundaries closed the defined scope; completion was recorded in `0e20a5c`.
- Condensed without reopening the work on 2026-07-17 in `f38a4b3` as
  `plugin/release/docs/architecture/refactor-history.md`.

## Archival outcome

- **Completed:** the defined behavior-preserving extraction completed, and the
  command, lifecycle, persistence, Git, dispatch, migration, and V1 boundaries
  below became explicit and tested.
- **Changed decisions:** later work rejected executable V2 local delivery and
  replaced several compatibility candidates, but did not change the completed
  refactor's behavior-preserving result.
- **Unfinished in this series:** none of the defined refactor scope remained;
  bounded recovery, compatibility, and developer-experience work was explicitly
  outside it.
- **Transferred:** the immediate audit moved to entry 002, and subsequent
  hardening and product-capability decisions moved to entry 003 and now to the
  current architecture owners.

## Global invariants preserved by the refactor

- V2 config owns unit architecture, and V2 state owns selected-unit versions.
- Version and tag calculation remained stable.
- Clean-worktree and exact known-file staging rules remained stable.
- Release commit messages, commit contents, lightweight tag target, and commit-before-tag push order remained stable.
- Execution journals persisted pending markers before unsafe mutations and confirmed phases only afterward.
- Dispatch journals were prepared before push and recorded request start before HTTP.
- Accepted, rejected, and unknown dispatch outcomes remained distinct.
- Dry-run and recovery stayed read-only where their contracts require that.
- Stable error codes, response schemas, renderer hints, and item ordering were preserved.
- Secret non-disclosure was preserved.
- V1 compatibility behavior stayed available through documented compatibility facades.

## Refactor boundary record

### Command contract characterization

The initial characterization pinned active release and resume command responses, error codes, renderer hints, ordered rows, unresolved-journal blocking, dispatch terminal outcomes, resume restrictions, dry-run immutability, and secret absence.

### Command presentation extraction

Release and resume handlers were reduced to presentation boundaries: parse untyped plugin flags into typed requests, invoke one application boundary, and map typed results or classified failures through response mappers with explicit clocks.

### GitHub Actions release use-case extraction

The active V2 GitHub Actions release path became one ordered use case with replaceable dependencies. The safety order remains visible: token resolution, materialization planning, Git and journal preflight, execution journal preparation, materialization, state write, targeted staging, commit, tag, dispatch journal preparation, commit push, tag push, workflow dispatch, and accepted handoff confirmation.

Every unsafe local or remote mutation is protected by a pending marker before the effect and a confirmed phase after success. Tests can inject failures around those boundaries.

### Resume policy extraction

Resume became a read-only assessment plus one explicit continuation operation or refusal. It no longer calculates a fresh version or identity. It reuses active release operations for supported continuation from confirmed local evidence and refuses ambiguous pushes, unsupported phases, terminal dispatch states, corrupt journals, and uncertain evidence.

### Canonical adapter consolidation

Plugin manifest materialization now uses validated release-unit metadata. Execution and dispatch journal stores share only common-dir and private atomic-write mechanics while retaining separate schemas and state rules. Active V2 release and resume share one coordinator boundary, one typed dispatch-token boundary, and one clock boundary.

### V2 initialization and pair persistence extraction

Init and unit-add became separate application intentions with typed requests, pure unit construction, pure file-presence policy, complete config/state pair validation, and shared rollback-backed pair persistence.

The pair writer provides bounded recovery for returned replacement failures and deterministic next-process recovery for supported interruption evidence. It does not claim impossible cross-file atomicity.

### Read-only query and plugin-index extraction

Validate, history, contributors, and plugin-index gained typed query boundaries, command-owned read capabilities, deterministic mappers, and explicit structured-versus-fatal error behavior.

Plugin-index output became `query -> build -> persist`: discovery and validation are read-only, JSON byte construction is pure, and output mode performs one atomic requested-path persistence effect.

### Pipeline runtime inspection

Pipeline Inspection was extended from the configured view with an immutable
runtime snapshot composed at the authoritative lifecycle boundary. Root
composition reads and validates local execution/dispatch journals, correlates
their exact identities, observes bounded local Git evidence, and reuses the
existing recovery, resume, and dispatch retry-safety decisions. The focused
internal package remains a data projector and presenter with no journal store,
Git client, mutation, network, token, transition, or continuation capability.

The additive schema preserves version `1` while exposing execution, dispatch,
local Git, recovery, manual-intervention, and remote-not-inspected boundaries.
It never infers progress from timestamps, treats conflicting evidence as
invalid structured output, and does not equate accepted dispatch handoff with
remote publication completion.

### Migration recovery extraction

Migration planning, policy, execution, journaling, target verification, source archival, and backup verification became explicit boundaries. Migration uses the shared V2 pair persister for target creation and refuses owner-ambiguous pair evidence.

The serialized migration journal values remain compatible, while unknown or corrupt evidence fails closed rather than being guessed.

### V1 compatibility subsystem extraction

V1 release behavior was isolated behind typed intent, pure planning, preview/execution use cases, focused requirements/preflight/materialization/Git/compensation adapters, fixed executor composition, and executor-owned process/token/environment/clock ports.

Active production bypasses mutable registry lookup. Registry-backed release entry points, service-style APIs, fatal preflight, tool interfaces, and concrete executor legacy methods remain compatibility surfaces governed by [v1-compatibility-policy.md](../architecture/v1-compatibility-policy.md).

## Remaining historical limitations after the refactor

- V1 compensation is interruption-safe only for supported local actions and intentionally manual for uncertain remote or executor ambiguity.
- Pair and migration recovery is evidence-driven but refuses corrupt, externally edited, unsupported, or owner-ambiguous evidence.
- V2 local delivery was later evaluated and rejected for executable V2 releases.
- Compatibility registries, version-evidence globals, cwd facades, and broad public wrappers remain only as documented compatibility surfaces.
- Plugin-index output policy remains intentionally undecided until generated-output path semantics are designed.

## July 2026 code-quality consolidation

A later code-quality sequence preserved the behavior above while tightening
ownership. Ten planned structural commits characterized architecture boundaries,
centralized shared tool
identity and configuration facts, extracted reusable GoReleaser facts, isolated
workflow facts and dispatch HTTP, moved Doctor and supporting command
capabilities into focused internal packages, made the active release operation
order visible through named files, split mixed files by subject, and finalized
documentation, lint, and architecture controls. Corrective follow-up Commit 11
fixed active/compatibility ownership and documentation boundaries; final
corrective follow-up Commit 12 completed the package-wide compatibility guard
and pre-commit restoration boundaries. The final sequence contains twelve
commits.

No workflow YAML, GoReleaser configuration, journal schema, command contract,
public release behavior, or active lifecycle order changed. The concise final
architecture is recorded in [package-ownership.md](../architecture/package-ownership.md).

## Validation baseline

The refactor used focused package tests while iterating and the repository validation set described in `plugin/release/RULES.md`: Release Plugin tests, full Go tests, lint, and diff checks where applicable.

Future changes must run validation proportional to their risk and must not treat this historical record as permission to change behavior.
