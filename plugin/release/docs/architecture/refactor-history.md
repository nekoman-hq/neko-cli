# Release Plugin Refactor History

## Purpose

This document records the closed behavior-preserving Release Plugin refactor as durable architecture history. It avoids project-management labels and preserves the technical boundaries that were established.

Current runtime behavior is authoritative in [current-state.md](current-state.md). Post-refactor dependency direction and remaining debt are maintained in [post-refactor-review.md](post-refactor-review.md). Future capability decisions are described in [architecture-evolution.md](architecture-evolution.md).

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

### Migration recovery extraction

Migration planning, policy, execution, journaling, target verification, source archival, and backup verification became explicit boundaries. Migration uses the shared V2 pair persister for target creation and refuses owner-ambiguous pair evidence.

The serialized migration journal values remain compatible, while unknown or corrupt evidence fails closed rather than being guessed.

### V1 compatibility subsystem extraction

V1 release behavior was isolated behind typed intent, pure planning, preview/execution use cases, focused requirements/preflight/materialization/Git/compensation adapters, fixed executor composition, and executor-owned process/token/environment/clock ports.

Active production bypasses mutable registry lookup. Registry-backed release entry points, service-style APIs, fatal preflight, tool interfaces, and concrete executor legacy methods remain compatibility surfaces governed by [v1-compatibility-policy.md](v1-compatibility-policy.md).

## Remaining historical limitations after the refactor

- V1 compensation is interruption-safe only for supported local actions and intentionally manual for uncertain remote or executor ambiguity.
- Pair and migration recovery is evidence-driven but refuses corrupt, externally edited, unsupported, or owner-ambiguous evidence.
- V2 local delivery was later evaluated and rejected for executable V2 releases.
- Compatibility registries, version-evidence globals, cwd facades, and broad public wrappers remain only as documented compatibility surfaces.
- Plugin-index output policy remains intentionally undecided until generated-output path semantics are designed.

## Validation baseline

The refactor used focused package tests while iterating and the repository validation set described in `plugin/release/RULES.md`: Release Plugin tests, full Go tests, lint, and diff checks where applicable.

Future changes must run validation proportional to their risk and must not treat this historical record as permission to change behavior.
