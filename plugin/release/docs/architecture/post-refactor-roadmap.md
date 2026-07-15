# Release Plugin Post-Refactor Roadmap

## Status and scope

The behavior-preserving Release Plugin refactor is closed at 9 / 9 stages. This roadmap starts after that ledger; none of the milestones below is Stage 10 or reopens a completed stage.

The architecture evidence, debt ranking, compatibility inventory, and removal candidates behind this roadmap are in [post-refactor-review.md](post-refactor-review.md). The detailed runtime and disk contracts remain in [current-state.md](current-state.md). Every implementation milestone remains subject to `plugin/release/RULES.md`.

Milestone namespaces distinguish the nature of future work:

| Namespace | Work class | Purpose |
| --- | --- | --- |
| `H` | Safety hardening | Reduce interruption, recovery, or evidence risk without broadening release behavior |
| `C` | Compatibility cleanup | Make explicit support decisions, deprecate surfaces, and remove only proven superseded paths |
| `DX` | Developer experience | Clarify presentation, composition, and filesystem policies |
| `F` | New features | Add separately authorized user-facing capabilities |

The recommended next milestone is **H1 — Make V1 compensation interruption-safe**. It addresses the highest-severity active risk: destructive V1 rollback can change local Git and GitHub state without durable intent or confirmation. H1 does not imply that all compatibility cleanup must precede product work. In particular, read-only F1 can proceed independently after its own characterization and can be developed in parallel if it does not touch H1 files or contracts.

## Sequencing and gates

```text
H1 ───────────────┐
                  ├─ safety baseline for new V1 mutation work
H2 ───────► H3 ───┘

C1 ───────► C2

DX1             DX2             DX3

F1  (independent of C1/C2; token-free and read-only)
F2  (decision milestone; implementation requires a later approved design)
```

- Start with H1 because its risk includes destructive remote compensation.
- H2 precedes changes that add multi-file config/state writes or broaden migration.
- H3 precedes any journal schema change or automated repair behavior.
- C1 must establish support and deprecation policy before C2 removes exported compatibility code.
- DX1 should precede substantial edits to active V2 orchestration files.
- F1 does not wait for compatibility cleanup. It may follow H1 or run independently when file ownership and validation remain isolated.
- F2 is evaluation only. A positive decision creates a new, separately reviewed implementation milestone; it does not activate the existing scaffold.

## Safety hardening

### H1 — Make V1 compensation interruption-safe

- **Objective:** Preserve durable intent and outcome evidence around every destructive V1 compensation action, and bound the legacy GitHub deletion client so an interrupted or uncertain rollback fails closed instead of guessing.
- **User value:** A failed V1 release has an auditable recovery record and cannot silently continue destructive cleanup after the process loses proof of what already happened.
- **Characterize first:** Pin the current rollback order, state-flag prerequisites, idempotent/not-found outcomes, partial Git/GitHub failures, token redaction, untracked-file deletion, and executor-specific failure mapping. Add interruption cases before changing production behavior.
- **Scope:** `V1ReleaseRollback`, its Git and GitHub adapters, bounded HTTP client construction, durable compensation evidence, supported continuation/refusal decisions, operator-facing recovery guidance, and exact compatibility response mapping.
- **Non-goals:** V1 redesign, V1 removal, broader compensation, remote-state inference, blind retry, V2 journal reuse, or a generic transaction/workflow engine.
- **Dependencies:** Existing Stage 9 V1 characterization; an explicit location, schema, permissions, and lifecycle decision for V1 evidence; agreement on supported manual versus automatic continuation.
- **Risks:** A new journal can itself become stale or corrupt; tightening a legacy timeout can expose previously hidden hangs as errors; resuming destructive work from insufficient evidence would be worse than the current behavior.
- **Acceptance criteria:** Every destructive action records intent before mutation and confirmation only after verified success; uncertain evidence blocks automatic continuation; network requests use an injected client with a finite timeout and bounded error reads; repository root is explicit; secrets never reach files, errors, logs, or responses; pre-existing V1 behavior remains characterized; no V2 ordering or schema changes; full failure-injection tests pass.
- **Expected commit boundaries:** (1) characterization only; (2) bounded/root-aware GitHub client; (3) durable evidence model and store; (4) rollback integration plus refusal/continuation policy; (5) recovery documentation and final architecture validation. Keep executor-specific behavior in independently revertible commits where practical.

### H2 — Make pair and migration crash recovery explicit

- **Objective:** Define and implement a crash-recovery protocol for V2 config/state pair replacement and align migration recovery with that protocol without claiming impossible cross-file atomicity.
- **User value:** After process or machine interruption, the next command can deterministically recover a complete valid pair or preserve evidence and explain a safe manual action.
- **Characterize first:** Exercise every boundary before and after temp-file sync, target replacement, restoration, migration target verification, source archive, backup verification, and journal removal. Preserve byte, mode, missing-file, directory, and symlink observations.
- **Scope:** `V2ReleasePairPersister`, init/unit-add/migration consumers, pair-generation or pair-journal evidence, migration crash classification, exact restoration, and safe cleanup of a newly created empty `.neko` directory if proven.
- **Non-goals:** A general filesystem transaction library, silent repair of externally edited files, migration format expansion, config/state schema redesign, or weakening strict loaders.
- **Dependencies:** A documented portable filesystem protocol; compatibility analysis for persisted evidence; recovery ownership that remains separate from release execution and dispatch journals.
- **Risks:** More evidence files can introduce conflicting states; filesystem durability differs across platforms; automatic cleanup could remove user-created content if directory ownership is not proven.
- **Acceptance criteria:** Every enumerated crash point resolves to a verified complete pair, exact restoration, or evidence-preserving refusal; migration never archives the only trustworthy V1 source before a valid V2 pair is proven; recovery is idempotent; modes and bytes remain stable; no unrelated directory is removed; Linux and macOS tests cover the protocol where CI permits.
- **Expected commit boundaries:** (1) crash-point characterization and protocol decision; (2) pair evidence/persistence mechanics; (3) init and unit-add adoption; (4) migration alignment and recovery; (5) cleanup policy plus documentation. Do not combine schema changes with use-case adoption.

### H3 — Add evidence-safe journal inspection and lifecycle support

- **Objective:** Provide read-only inspection first, then narrowly authorized lifecycle operations for release, dispatch, migration, and any H1/H2 evidence without inferring that an unsafe effect completed.
- **User value:** Operators can understand why recovery is blocked and can archive or migrate evidence through an auditable process instead of editing private JSON by hand.
- **Characterize first:** Pin corrupt, unsupported-version, conflicting, terminal, unresolved, and missing-evidence behavior for each journal owner, plus a complete second-release scenario after handoff-ready exclusion.
- **Scope:** Typed inspection facts, redacted human/structured output, immutable backups before any lifecycle mutation, explicit confirmation, completed-journal archival policy, repeated-release characterization, and schema migration only when a concrete new schema exists.
- **Non-goals:** Automatic remote reconciliation, clearing pending actions by assumption, blind retry, a generic journal repository, or changing terminal dispatch semantics.
- **Dependencies:** H2 for pair/migration evidence integration; H1 if V1 compensation evidence is included; an explicit retention and schema-support policy.
- **Risks:** A repair command can legitimize unsafe operator guesses; output may reveal sensitive request context; premature schema abstraction can merge intentionally distinct journal contracts.
- **Acceptance criteria:** Inspection is token-free and read-only; corrupt/unknown evidence is preserved; any mutation creates an exact private backup and requires explicit authorization; no operation converts uncertainty into completion; second-release tests prove completed journals are not reopened; schema versions remain owner-specific.
- **Expected commit boundaries:** (1) characterization including repeated release; (2) typed read-only inspection; (3) output/redaction mapping; (4) optional archival lifecycle; (5) schema migration only as a separate commit with a real format need.

## Compatibility cleanup

### C1 — Decide and deprecate V1 compatibility surfaces

- **Objective:** Turn unknown compatibility risk into an explicit support matrix and migrate known consumers away from mutable registries, version-evidence globals, mixed builders, and legacy executor method ownership.
- **User value:** Embedders receive a documented stable path and deprecation window instead of surprise removal, while maintainers gain one clearly canonical composition model.
- **Characterize first:** Inventory repository, module, generated entry-point, and known downstream use of every symbol in the review's compatibility table; pin fatal-exit, cwd, registry overwrite, and executor delegation behavior.
- **Scope:** `HandleRelease`, `Service`, `Preflight`, `Tool`, `ToolBase`, `Register`/`Get`, executor legacy methods, `BuildReleaseExecutionContext`, V1 config facades, and mutable version-evidence seams.
- **Non-goals:** Immediate V1 removal, production behavior changes, removal based only on IDE reachability, or forcing feature work to wait for cleanup.
- **Dependencies:** A public API/versioning policy, a downstream communication channel, replacement examples, and deprecation duration.
- **Risks:** Unseen Go consumers may compile against exported symbols; fatal and cwd behavior may be observable; partial deprecation can leave two apparent canonical paths.
- **Acceptance criteria:** Every retained surface has an owner and support status; every deprecated surface points to a tested replacement; production stays on fixed explicit composition; globals have no new consumers; direct `Rollback` owns canonical executor rollback behavior before `RevertRelease` is considered removable; no public symbol is deleted in C1.
- **Expected commit boundaries:** (1) consumer/contract evidence; (2) support matrix and deprecation docs; (3) explicit registry replacement for known callers; (4) version-evidence injection; (5) executor delegation inversion and migration examples.

### C2 — Retire superseded and inactive release paths

- **Objective:** Remove only code whose consumer audit and deprecation window prove it is superseded, test-only, or an unselected future scaffold.
- **User value:** The supported release path becomes easier to understand and safer to extend, without breaking acknowledged consumers.
- **Characterize first:** Re-run repository and downstream reference searches; move tests from private bridges to canonical boundaries; prove production uses named V2 operations and fixed V1 executors.
- **Scope:** Eligible internal V1 bridges, inactive `ReleaseTransaction` preparation, `GitReleaseCoordinator.Coordinate`, `V2ExecutionUnavailableResponse`, `buildV2InitConfigFromFlags`, registry internals after C1, and the exact unused raw `pkg/git` helpers listed in the review.
- **Non-goals:** Removing active `git.Current` or query/preflight helpers, deleting public APIs without policy, implementing V2 local execution, or combining cleanup with behavior changes.
- **Dependencies:** C1 completion for exported V1 surfaces; F2 decision for local-execution scaffolding; deprecation window and downstream evidence.
- **Risks:** Reflection, blank imports, or external packages may make a symbol reachable outside repository search; deleting test scaffolds can reduce failure coverage if tests are not moved first.
- **Acceptance criteria:** Each deletion has call-site evidence and a replacement or explicit unsupported decision; architecture guards prevent reintroduction of competing orchestration; all behavior tests remain; release binaries and public docs contain no stale reference; removals are independently revertible by family.
- **Expected commit boundaries:** (1) test migration; (2) private bridge removal; (3) inactive V2-local path decision/removal; (4) raw Git helper removal; (5) exported compatibility removals by separately announced family.

## Developer experience

### DX1 — Isolate release progress reporting

- **Objective:** Replace active V2 package-global terminal logging with a narrow reporter supplied by composition while keeping application results and unsafe operation order unchanged.
- **User value:** Output remains stable, tests become isolated, and presentation changes no longer require editing safety-oriented application code.
- **Characterize first:** Capture verbose and non-verbose progress order, dry-run summaries, failure output, response coexistence, and token/secret absence.
- **Scope:** V2 start/planning, runner/use-case progress, focused V2 operations, reporter event vocabulary, and command composition.
- **Non-goals:** A generic event bus, telemetry framework, workflow pipeline, response construction in application code, or V1 logging redesign.
- **Dependencies:** A small stable set of presentation facts; coordination with any concurrent active-runner changes.
- **Risks:** Over-modeling events can become a second orchestration API; output snapshots can hide ordering changes; an injected no-op must not suppress required protocol responses.
- **Acceptance criteria:** Active application/operation files do not import the terminal logger; composition owns formatting; existing output order and redaction pass characterization; use-case dependencies stay narrow; release decisions, journal writes, and unsafe operation order are byte/behavior unchanged.
- **Expected commit boundaries:** (1) output characterization; (2) reporter contract and composition; (3) planning/start cutover; (4) runner/operation cutover; (5) architecture guard and docs.

### DX2 — Make command roots explicit for embedders

- **Objective:** Offer explicit-root composition so in-process callers do not depend on process-global cwd, while preserving the single-request CLI's existing root discovery.
- **User value:** Embedders and parallel tests can safely run against separate repositories without changing each other's working directory.
- **Characterize first:** Pin nested-start-directory discovery, V1 cwd facades, temporary restoration behavior, relative paths, and main's one-request lifecycle.
- **Scope:** New explicit-root entry points, adapter root propagation, embedder documentation, and gradual migration of known internal cwd consumers.
- **Non-goals:** Removing CLI root discovery immediately, changing relative-path user contracts, or promising concurrent safety before every global seam is removed.
- **Dependencies:** C1 support decisions for cwd-based public facades; a real embedding or concurrency requirement before broad replacement.
- **Risks:** Passing the wrong root can alter Git/config ownership; mixed explicit and implicit paths can be harder to reason about than cwd alone.
- **Acceptance criteria:** New canonical embedder APIs take a resolved root; two in-process test repositories do not interfere; main behavior remains unchanged; legacy cwd facades are documented and, if policy permits, deprecated rather than silently changed.
- **Expected commit boundaries:** (1) root-contract characterization; (2) explicit-root composition; (3) adapter migrations by command area; (4) concurrency tests and docs; (5) optional deprecation under C1.

### DX3 — Clarify generated-output path policy

- **Objective:** Decide whether plugin-index output is intentionally arbitrary or repository-confined and make symlink/overwrite behavior explicit.
- **User value:** CI retains deliberate flexibility while users can predict exactly which paths the command may create or replace.
- **Characterize first:** Pin absolute/relative paths, parent creation, existing mode, symlinks, check/render modes, and paths outside the repository.
- **Scope:** Policy documentation, parser validation, output-persister confinement or explicit opt-in, and actionable error mapping.
- **Non-goals:** Changing JSON schema/order, weakening atomic single-file replacement, or applying one path policy to unrelated commands.
- **Dependencies:** Product/security decision on arbitrary output and backward-compatibility plan for existing CI.
- **Risks:** Confinement can break legitimate artifact paths; symlink rejection differs across filesystems; permissive defaults retain overwrite risk.
- **Acceptance criteria:** One policy is documented and enforced; symlink behavior is tested; unchanged requested paths remain visible in typed intent; check/render stay write-free; existing mode/atomicity contracts remain unless explicitly versioned.
- **Expected commit boundaries:** (1) path characterization; (2) policy decision/docs; (3) parser/persister enforcement; (4) compatibility messaging and architecture update.

## New features

### F1 — Add release plan inspection

- **Objective:** Add a typed, token-free, read-only way to inspect the selected release source/unit, current and next version, tag, planned materialization, exact known files, and local journal blockers without starting execution.
- **User value:** Users and CI can review a release plan before mutation and diagnose local blockers with stable structured output.
- **Characterize first:** Pin existing dry-run calculations and outputs for V1/V2, unit selection, version/tag rules, materialization validation, unresolved-journal checks, and structured/fatal response boundaries.
- **Scope:** A dedicated query/use-case boundary, typed facts, human and structured response mapping, manifest/docs changes if a new command is selected, and reuse of pure planning capabilities.
- **Non-goals:** Token resolution, file writes, Git mutation, workflow dispatch, remote probing, retry, or changing existing dry-run behavior incidentally.
- **Dependencies:** Completed refactor planning seams; no dependency on C1 or C2. H1 remains the recommended next overall milestone, but F1 may proceed independently when work does not overlap.
- **Risks:** Reusing a runner facade could accidentally construct mutating adapters; duplicated planning could drift; exposing filesystem details may create a new public contract.
- **Acceptance criteria:** Tests prove zero writes/process mutations/network/token reads; calculations match canonical release planning; selected paths are validated; results are typed until presentation; command and manifest contracts agree; V1 compatibility is explicit rather than inferred.
- **Expected commit boundaries:** (1) behavior/contract characterization; (2) typed query and pure planning reuse; (3) response mapping and command/manifest integration; (4) docs and architecture guards.

### F2 — Evaluate V2 local delivery

- **Objective:** Decide whether V2 local publication should exist and, if so, produce a reviewed executor-by-executor safety design before any implementation.
- **User value:** Users receive either a credible path to local V2 delivery or a clear explanation that GitHub Actions remains the supported owner.
- **Characterize first:** Pin the current explicit block, executor capabilities, release-it ownership of Git actions, known-file/state requirements, and all V2 journal/recovery invariants.
- **Scope:** Product use cases, executor feasibility, ownership of materialization/state/Git, state-in-commit proof, journal model, failure matrix, and migration/compatibility impact.
- **Non-goals:** Activating `ReleaseTransaction`, using `GitReleaseCoordinator.Coordinate` as-is, implementing code, weakening GitHub Actions safety, or treating an executor subprocess as safely recoverable without proof.
- **Dependencies:** Product approval, executor-specific experiments, and architecture review. A positive result creates a later implementation milestone; a negative result unlocks relevant C2 cleanup.
- **Risks:** Some executors own commit/tag/push internally and cannot satisfy exact known-file or interruption guarantees; two active publication owners can drift.
- **Acceptance criteria:** The decision is recorded per executor; any positive design proves exact state inclusion, commit/tag/push ownership, pending evidence, resume/refusal semantics, and token boundaries; no production path is activated by this milestone.
- **Expected commit boundaries:** (1) characterization; (2) executor feasibility evidence; (3) architecture decision record; (4) roadmap update selecting a new feature milestone or C2 removal.

## Intentionally deferred and preserved

The following are not backlog omissions. They remain deliberate boundaries until separately authorized evidence changes the decision:

- no blind retry of ambiguous push or uncertain workflow dispatch;
- no remote-state inference that converts an observation into proof of safe retry;
- no generic workflow pipeline, universal state-machine engine, dependency bag, or shared journal repository;
- no V1 removal before C1 establishes consumers, policy, replacements, and a deprecation window;
- no V2 local execution before F2 and a subsequent implementation milestone prove executor ownership and recovery;
- no journal schema migration or repair command before H3 has a concrete format/support need;
- no process-wide cwd rewrite before an embedding/concurrency requirement justifies DX2;
- no plugin-index confinement change before DX3 resolves the product policy.

## Roadmap completion reporting

Roadmap progress is reported independently from the historical refactor ledger. A future status should say, for example, “Refactor stages: 9 / 9 completed; roadmap milestone H1: completed.” It must not report H1 as Stage 10.

Each milestone begins with characterization, retains independently revertible commit boundaries, runs the full uncached Release Plugin and repository validation required by `plugin/release/RULES.md`, and updates this roadmap plus the architecture review when evidence changes a priority or recommendation.
