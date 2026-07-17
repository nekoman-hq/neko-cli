# Release Plugin Architecture Evolution

## Purpose and scope

This document records durable Release Plugin architecture decisions after the completed behavior-preserving refactor. It is not a project-management roadmap and does not track task status.

Detailed runtime behavior and disk contracts live in [current-state.md](current-state.md). Dependency direction, debt ranking, and compatibility inventory live in [post-refactor-review.md](post-refactor-review.md). The closed refactor history lives in [refactor-history.md](refactor-history.md). V1 compatibility support decisions live in [v1-compatibility-policy.md](v1-compatibility-policy.md).

Every future implementation remains subject to `plugin/release/RULES.md`.

## Completed architecture records

### V1 compensation interruption safety

Active V1 release execution writes strict private evidence before local mutation and persists pending intent before each supported compensation effect. Supported repeatable local work can continue on the next invocation; uncertain remote work, unsupported evidence, corrupt evidence, pending non-repeatable operations, and executor ambiguity fail closed with manual-recovery guidance.

The active V1 GitHub Release client is injected, root-aware, bounded by timeout and response size, verifies deletion through an observable not-found result, and unwraps its token only while constructing the HTTP request.

### V2 pair and migration crash recovery

Init, unit-add, and migration share the V2 config/state pair persister. The persister creates durable pair-recovery evidence before replacing either file, records pending and confirmed target replacement, verifies exact bytes, validates the complete intended pair, and either closes already-complete evidence or restores exact prior bytes, modes, and absence when supported.

Migration owns a separate worktree journal. It persists the complete V2 target pair before archiving the V1 source, verifies exact target bytes and strict V2 validity before the archive, verifies the byte-identical backup after archive, and refuses owner-ambiguous pair evidence.

These guarantees are crash-recoverable, not cross-file atomic. Corrupt, externally edited, unsupported, hash-conflicting, mode-conflicting, or owner-ambiguous evidence requires manual recovery.

### Evidence inspection and archival

`neko release evidence` is read-only and token-free. It reports redacted typed summaries and diagnostics for release execution journals, dispatch journals, migration journals, V1 compensation evidence, and V2 pair-recovery evidence.

`neko release evidence-archive` supports only completed release-execution, completed V1 compensation, and completed V2 pair-recovery evidence. It requires family, identity, current digest, and explicit confirmation; then it re-observes the evidence, writes and verifies an exact private archive, and removes the completed source. Dispatch and migration evidence remain inspect-only.

No repair command, remote reconciliation, unsafe completion inference, or schema migration exists without a separate evidence-specific design.

### V1 compatibility policy and retired release paths

The V1 compatibility policy documents which exported surfaces are kept, deprecated, deferred, removed, or still dependent on downstream evidence. Production composition uses explicit V1 executor values and bypasses mutable registry lookup.

Retired internal bridges, inactive response helpers, one-call coordinator convenience paths, and unused raw Git helpers are guarded against reintroduction. Retained compatibility surfaces remain direct delegates or deliberately bounded facades; new production code must not depend on mutable registries, version-evidence globals, inactive V2-local scaffolding, or raw destructive Git helpers.

### Typed release progress reporting

Active V2 progress reporting is behind the `ReleaseProgress` port. Application and operation code emits safe typed `ReleaseProgressEvent` values. Terminal rendering lives in adapter files, Git diagnostics are separate, and command responses remain owned by response mappers.

The reporter is synchronous and infallible. It cannot choose release policy, retry behavior, journal state, Git or network effects, command success, or response schema. Unknown events render nothing.

### Explicit repository-root composition

The plugin executable resolves one `workspace.RepositoryRoot` at the command boundary and routes production commands through explicit-root handlers. Production command routing no longer mutates process cwd.

Explicit-root entry points exist for init, unit-add, release, resume, validate, history, contributors, evidence, evidence archive, migration, and plugin-index. Existing cwd-based facades remain compatibility surfaces. Production migration keeps its legacy Git-root discovery facade to preserve nested-V1 behavior, while embedders can call the explicit-root migration handler.

Isolation tests prove two in-process repositories can be validated and indexed without changing process cwd or leaking root-specific data.

### Generated-output path policy

Generated-output path policy is explicit per output family rather than shared through a universal path manager.

`plugin-index --output` owns the generated integration artifact path. Relative paths are clean repository-root-relative paths resolved from the explicit `workspace.RepositoryRoot`, independent of process cwd, CLI nesting, or embedder mode. Explicit absolute paths remain supported for CI and temporary artifact targets, including the plugin release workflows' runner-temp output. Repository-contained targets cannot overwrite release config/state, recovery or migration evidence, legacy release config/backup, Git internals, or the plugin manifest inputs present in the generated index. Existing target directories and target symlinks are rejected before writing. Existing repository-relative parent symlinks are allowed only when their physical target remains inside the resolved repository root. Missing parents are created only by the persister, with mode `0755`; new files use `0644`, existing file mode is preserved, and replacement remains target-local atomic.

User-declared materialized release files are owned by V2 release planning and materialization. Plugin manifest paths come only from validated plugin unit metadata, and JReleaser materialization owns only `jreleaser.yml` below the unit root. V2 `workingDirectory` must be lexically and physically inside the resolved repository root, materialized target symlinks are rejected, and `KnownReleaseFiles` keeps staging identity repository-relative to the same root. `MaterializationTransaction` continues to snapshot and restore exact bytes/modes before commit uncertainty.

Operational state remains separately owned by its existing components: config/state/pair recovery by config persistence, execution and dispatch journals below the Git common dir, V1 compensation evidence, migration journals/backups, and evidence archives. These files are not folded into generated-output handling merely because they are files.

Check, render, and planning modes remain write-free. The plugin-index JSON schema/order and command response schema are unchanged.

## Pending architecture decisions

### Release plan inspection

A future release-plan inspection command could provide token-free, read-only facts about selected source/unit, current and next version, tag, planned materialization, known release files, and local blockers without starting execution.

Such a command must reuse canonical planning facts, avoid token reads and mutation, keep output response mapping at the command boundary, and preserve existing dry-run behavior unless an explicit compatibility change is approved.

### V2 local delivery evaluation

V2 local delivery remains blocked. GitHub Actions is the active V2 publication owner. Local executor activation requires a product and safety decision per executor, including state-in-commit proof, exact Git ownership, journal boundaries, recovery/refusal semantics, token boundaries, and release-it feasibility.

A positive decision would require a new implementation design. A negative decision would unlock cleanup of retained inactive V2-local scaffold.

## Preserved invariants

- No blind retry for ambiguous push or uncertain workflow dispatch.
- No remote-state inference that converts observation into proof of safe retry.
- No generic workflow pipeline, universal state-machine engine, dependency bag, or shared journal repository.
- No V1 removal without fresh consumer evidence, replacement paths, deprecation policy, and a separately approved removal.
- No V2 local execution without executor-specific ownership and recovery proof.
- No journal repair or schema migration without an inspection-first evidence design.
- No claim of full concurrent safety for retained cwd compatibility facades.
- No repository-relative generated output may infer its base from process cwd.
- No generated integration artifact may overwrite protected release state, evidence, Git internals, or plugin manifest inputs.

## Implementation guidance

Future work should use descriptive capability names in code, tests, documentation, and commits. Characterization remains the first step for behavior-sensitive changes, but permanent artifacts must describe the behavior or architectural boundary rather than internal project-management labels.
