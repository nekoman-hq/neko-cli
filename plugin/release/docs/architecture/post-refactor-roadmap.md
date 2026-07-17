# Release Plugin Post-Refactor Roadmap

## Status and scope

The behavior-preserving Release Plugin refactor is closed at 9 / 9 stages. This roadmap starts after that ledger; none of the milestones below is Stage 10 or reopens a completed stage.

- Refactor: completed
- Post-refactor review: completed
- H1: completed
- H2: completed
- H3: completed
- C1: completed
- C2: completed
- DX1: completed
- Next roadmap milestone: **DX2 — Make command roots explicit for embedders**

The architecture evidence, debt ranking, compatibility inventory, and removal candidates behind this roadmap are in [post-refactor-review.md](post-refactor-review.md). The detailed runtime and disk contracts remain in [current-state.md](current-state.md). Every implementation milestone remains subject to `plugin/release/RULES.md`.

Milestone namespaces distinguish the nature of future work:

| Namespace | Work class | Purpose |
| --- | --- | --- |
| `H` | Safety hardening | Reduce interruption, recovery, or evidence risk without broadening release behavior |
| `C` | Compatibility cleanup | Make explicit support decisions, deprecate surfaces, and remove only proven superseded paths |
| `DX` | Developer experience | Clarify presentation, composition, and filesystem policies |
| `F` | New features | Add separately authorized user-facing capabilities |

H1 — Make V1 compensation interruption-safe, H2 — Make pair and migration crash recovery explicit, H3 — Add evidence-safe journal inspection and lifecycle support, C1 — Decide and deprecate V1 compatibility surfaces, C2 — Retire superseded and inactive release paths, and DX1 — Isolate release progress reporting are completed. The recommended next milestone is **DX2 — Make command roots explicit for embedders**. Compatibility cleanup still does not have to precede unrelated product work; read-only F1 may proceed independently when it does not touch H2/H3 files or contracts.

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

- H1 established the durable V1 compensation baseline; new V1 mutation work must preserve it.
- H2 established the required evidence protocol for changes that add multi-file config/state writes or broaden migration.
- H3 precedes any journal schema change or automated repair behavior.
- C1 established support and deprecation policy; C2 removed only candidates whose call-site evidence, replacement, and policy gates were satisfied.
- DX1 isolated active V2 progress reporting before substantial edits to active V2 orchestration files.
- DX2 should precede in-process embedder work that depends on multiple repository roots or parallel command execution.
- F1 does not wait for compatibility cleanup. It may follow H1 or run independently when file ownership and validation remain isolated.
- F2 is evaluation only. A positive decision creates a new, separately reviewed implementation milestone; it does not activate the existing scaffold.

## Safety hardening

### H1 — Make V1 compensation interruption-safe

- **Status:** **Completed.** Active V1 execution now fails closed around interrupted or uncertain compensation while preserving the public command response contract and all V2 ordering/schema boundaries.
- **Evidence ownership:** Schema version 1 is V1-only and lives at `<git-common-dir>/neko/release/v1-compensation/current.json` beneath a private `0700` directory with an atomically replaced `0600` file. Strict decoding rejects unknown, corrupt, unhashed, or unsupported evidence. The fixed record contains release identity, exact original config content/hash, executor/Git facts, release/config status, eight typed action fields, failure classification, and timestamps. It contains no token, headers, callbacks, response body, raw failure string, generic map, or executable action list.
- **Operation order:** restore exact V1 config; delete GitHub Release; delete local tag; delete remote tag; revert release commit then push the revert, or reset an unpushed release commit; clean untracked release files. Every required operation persists pending intent before its side effect, verifies success where observable, and persists confirmation afterward. A failure stops all later effects.
- **Automatic continuation:** A subsequent V1 release invocation continues supported repeatable local actions and does not replay confirmed effects. An absent local tag is verified as already complete. Completed evidence is retained and safely replaced only when a later attempt starts.
- **Manual recovery boundary:** Pending or uncertain GitHub deletion, remote tag deletion, or revert push; a pending/non-repeatable revert; corrupt/unsupported evidence; and interruption or ambiguity inside an executor block automatic continuation with the evidence path. No remote-state inference, blind retry, fallback mutation, repair command, V2 journal reuse, or generic transaction engine was added.
- **Executor policy:** GoReleaser may compensate only proven local effects; push/publication ambiguity is manual. JReleaser may compensate only before commit/push/publication ambiguity. release-it failure is always externally uncertain because the subprocess owns all Git and publication effects.
- **Network boundary:** The active GitHub client is injected, root-aware, uses a 15-second timeout and 64 KiB response cap, verifies GET/DELETE/not-found, and unwraps a redacting typed token only at request construction. It never changes cwd or uses `http.DefaultClient`.
- **Compatibility:** Direct legacy `V1ReleaseRollback` calls remain characterized best-effort delegates. The active V1 application uses only the new fixed named operations. Public flags, schemas, response codes, fatal mapping, and executor selection are unchanged.
- **Delivered commits:** H1.1 characterization `43f5997`; H1.2 evidence model/store/policy `4b40f6a`; H1.3 active interruption-safe integration `3fde644`; H1.4 documentation and roadmap closure uses commit message `docs(release): complete h1 compensation hardening`.

### H2 — Make pair and migration crash recovery explicit

- **Status:** **Completed.** Init, unit-add, and migration now use one explicit crash-recoverable V2 pair writer. Migration recovery classifies pair evidence together with its migration journal and refuses owner-ambiguous evidence.
- **Evidence ownership:** Schema version 1 lives at `.neko/release.pair-recovery.json`. The record stores the target paths, exact prior and intended config/state bytes, hashes, modes, existence, per-target replacement status, restoration status, and completion flag. Strict decoding rejects unknown schema, invalid status values, invalid hashes, invalid modes, impossible state order, and completed evidence without confirmed target replacement.
- **Operation order:** create `.neko`; resolve unresolved pair evidence; capture prior snapshots; persist pair-recovery evidence; create, write, and sync both target-local temp files; record config replacement pending; replace and verify config; confirm config replacement; record state replacement pending; replace and verify state; confirm state replacement; strict-validate the intended complete pair; mark evidence complete; remove evidence.
- **Automatic continuation:** A later pair-writing command closes evidence when the intended pair is already complete and strict-valid. It restores exact prior bytes, modes, and absence for supported partial application when every observed target matches either prior or intended evidence. It then proceeds with the newly requested pair write.
- **Migration alignment:** A migration journal owns migration recovery. If migration and pair-recovery evidence coexist, migration delegates pair repair to the shared persister before continuing target/source verification. Pair-recovery evidence without a migration journal is refused as owner-ambiguous.
- **Manual recovery boundary:** Corrupt, unsupported, externally edited, hash/mode-conflicting, owner-ambiguous, or source/backup-untrustworthy evidence fails closed with preserved files. No generic transaction engine, generation pointer, silent repair, remote inference, or strict-loader weakening was added.
- **Compatibility:** Existing V2 config/state file schemas and modes are unchanged. The new evidence file is private recovery state for interrupted pair writes. A failed new-pair attempt may still leave an empty `.neko` directory; H2 deliberately avoided deleting a directory that might contain user-created content.
- **Delivered commits:** H2.1 characterization `1c1f388`; H2.2 pair evidence and persister recovery `69540e0`; H2.3 migration recovery alignment `2a09633`; H2.4 documentation and roadmap closure uses commit message `docs(release): complete h2 crash recovery hardening`.

### H3 — Add evidence-safe journal inspection and lifecycle support

- **Status:** **Completed.** `neko release evidence` provides read-only, redacted inspection for release-execution journals, dispatch journals, migration journals, V1 compensation evidence, and V2 pair-recovery evidence. `neko release evidence-archive` provides the only lifecycle operation, `archive-completed`, for completed release-execution, completed V1 compensation, and completed V2 pair-recovery evidence.
- **Objective:** Provide read-only inspection first, then narrowly authorized lifecycle operations for release, dispatch, migration, and any H1/H2 evidence without inferring that an unsafe effect completed.
- **User value:** Operators can understand why recovery is blocked and can archive or migrate evidence through an auditable process instead of editing private JSON by hand.
- **Characterize first:** Pin corrupt, unsupported-version, conflicting, terminal, unresolved, and missing-evidence behavior for each journal owner, plus a complete second-release scenario after handoff-ready exclusion.
- **Scope:** Typed inspection facts, redacted human/structured output, immutable backups before any lifecycle mutation, explicit confirmation, completed-journal archival policy, repeated-release characterization, and schema migration only when a concrete new schema exists.
- **Non-goals:** Automatic remote reconciliation, clearing pending actions by assumption, blind retry, a generic journal repository, or changing terminal dispatch semantics.
- **Dependencies:** completed H2 pair/migration evidence integration; completed H1 V1 compensation evidence if included; an explicit retention and schema-support policy.
- **Risks:** A repair command can legitimize unsafe operator guesses; output may reveal sensitive request context; premature schema abstraction can merge intentionally distinct journal contracts.
- **Acceptance criteria:** Inspection is token-free and read-only; corrupt/unknown evidence is preserved; any mutation creates an exact private backup and requires explicit authorization; no operation converts uncertainty into completion; second-release tests prove completed journals are not reopened; schema versions remain owner-specific.
- **Delivered commits:** H3.1 characterization `62052fc`; H3.2 read-only inspection `2241ab9`; H3.3 guarded archival `3ec8df5`; H3.3 lint cleanup `3fb309c`; H3.4 documentation and closure uses commit message `docs(release): complete h3 journal lifecycle hardening`.
- **Lifecycle policy:** Inspection is read-only and deterministic; diagnostics are emitted for corrupt, unsupported, conflicting, and invalid evidence without raw file content. Archival re-observes family, identity, and digest, requires explicit confirmation, writes an exact private archive copy, verifies it, and only then removes the completed source. No force, repair, retry, remote inference, dispatch archival, or migration archival was added.

## Compatibility cleanup

### C1 — Decide and deprecate V1 compatibility surfaces

- **Status:** **Completed.** C1 added repository characterization for the retained compatibility surfaces, recorded the support/removal decisions in [v1-compatibility-policy.md](v1-compatibility-policy.md), deprecated only surfaces with tested replacements, and kept/deferred surfaces without exact replacement unmarked.
- **Objective:** Turn unknown compatibility risk into an explicit support matrix and migrate known consumers away from mutable registries, version-evidence globals, mixed builders, and legacy executor method ownership.
- **User value:** Embedders receive a documented stable path and deprecation window instead of surprise removal, while maintainers gain one clearly canonical composition model.
- **Characterize first:** Inventory repository, module, generated entry-point, and known downstream use of every symbol in the review's compatibility table; pin fatal-exit, cwd, registry overwrite, and executor delegation behavior.
- **Scope:** `HandleRelease`, `Service`, `Preflight`, `Tool`, `ToolBase`, `Register`/`Get`, executor legacy methods, `BuildReleaseExecutionContext`, V1 config facades, and mutable version-evidence seams.
- **Non-goals:** Immediate V1 removal, production behavior changes, removal based only on IDE reachability, or forcing feature work to wait for cleanup.
- **Dependencies:** A public API/versioning policy, a downstream communication channel, replacement examples, and deprecation duration.
- **Risks:** Unseen Go consumers may compile against exported symbols; fatal and cwd behavior may be observable; partial deprecation can leave two apparent canonical paths.
- **Acceptance criteria:** Every retained surface has an owner and support status; every deprecated surface points to a tested replacement; production stays on fixed explicit composition; globals have no new consumers; direct `Rollback` owns canonical executor rollback behavior before `RevertRelease` is considered removable; no public symbol is deleted in C1.
- **Completed commit boundaries:** C1.1 `test(release): characterize v1 compatibility surfaces`; C1.2 `docs(release): decide v1 compatibility policy`; C1.3 `refactor(release): deprecate v1 compatibility surfaces`; C1.4 documentation closure uses commit message `docs(release): complete c1 compatibility deprecation`.
- **Completion record:** Production remains on `HandleReleaseWithV1Executors` and fixed concrete V1 executors. `Service`, registry functions, registry package init, mixed execution-context builder, mutable version guards, cwd V1 config facades, and selected executor legacy methods now have precise deprecation comments. `Preflight`, `Tool`, `ToolBase`, legacy executor `Init`, explicit V1 config operations, pure `EnsureVersionIsValid`, system constructors, migration/plugin-index programmatic APIs, and undecided V2-local scaffolding were kept or deferred because no exact replacement/removal decision exists. `Rollback` owns concrete executor rollback behavior; `RevertRelease` is a deprecated direct delegate. No public symbol was removed in C1.

### C2 — Retire superseded and inactive release paths

- **Status:** **Completed.** C2 removed only paths with repository call-site evidence and an explicit replacement or unsupported decision, while retaining compatibility surfaces whose C1/F2 preconditions were not met.
- **Objective:** Remove only code whose consumer audit and deprecation window prove it is superseded, test-only, or an unselected future scaffold.
- **User value:** The supported release path becomes easier to understand and safer to extend, without breaking acknowledged consumers.
- **Characterize first:** Re-run repository and downstream reference searches; move tests from private bridges to canonical boundaries; prove production uses named V2 operations and fixed V1 executors.
- **Scope:** Eligible internal V1 bridges, `GitReleaseCoordinator.Coordinate`, `V2ExecutionUnavailableResponse`, `buildV2InitConfigFromFlags`, and the exact unused raw `pkg/git` helpers listed in the review. `ReleaseTransaction` preparation and registry internals were audited but retained because their removal preconditions were not met.
- **Non-goals:** Removing active `git.Current` or query/preflight helpers, deleting public APIs without policy, implementing V2 local execution, or combining cleanup with behavior changes.
- **Dependencies:** completed C1 support decisions for exported V1 surfaces; F2 decision for local-execution scaffolding; deprecation window and downstream evidence.
- **Risks:** Reflection, blank imports, or external packages may make a symbol reachable outside repository search; deleting test scaffolds can reduce failure coverage if tests are not moved first.
- **Acceptance criteria:** Each deletion has call-site evidence and a replacement or explicit unsupported decision; architecture guards prevent reintroduction of competing orchestration; all behavior tests remain; release binaries and public docs contain no stale reference; removals are independently revertible by family.
- **Delivered commit boundaries:** C2.1 test migration `76252dd`; C2.2 private bridge removal `f3f0113`; C2.3 obsolete exported/internal helper removal `ddd687b`; C2.4 documentation closure uses commit message `docs(release): complete c2 path retirement`.
- **Removal record:** C2 removed `release.startLegacyRelease`, `release.newV1ReleaseCommandApplication`, `init.buildV2InitConfigFromFlags`, `release.V2ExecutionUnavailableResponse`, `(*release.GitReleaseCoordinator).Coordinate`, and raw `plugin/release/pkg/git` helpers `LastCommit`, `TotalCommits`, `FilesCount`, `RepoSize`, `DeleteGithubRelease`, `Head`, `CleanUntracked`, `DeleteLocalTag`, `DeleteRemoteTag`, `RevertCommit`, `CreateCommit`, and `HardResetTo`.
- **Retained record:** `ReleaseTransaction` and its preparation helpers remain blocked inactive scaffold pending F2. `Register`, `Get`, `tools`, and `pkg/release/tool` remain compatibility registry support while `HandleRelease` and `Service` are retained. `Service`, `Preflight`, `Tool`, `ToolBase`, executor legacy methods, `BuildReleaseExecutionContext`, cwd V1 config facades, `VersionGuard` helpers, migration/plugin-index APIs, and production constructors remain governed by the C1 policy register.

## Developer experience

### DX1 — Isolate release progress reporting

- **Status:** **Completed.** Active V2 start/planning, runner/use-case progress, focused release operations, Git progress, and dispatch progress now report typed `ReleaseProgressEvent` values through a synchronous `ReleaseProgress` port. Terminal rendering lives in `release_progress_terminal.go`; Git verbose diagnostics use a separate `gitReleaseDiagnostics` port and terminal adapter.
- **Objective:** Replace active V2 package-global terminal logging with a narrow reporter supplied by composition while keeping application results and unsafe operation order unchanged.
- **User value:** Output remains stable, tests become isolated, and presentation changes no longer require editing safety-oriented application code.
- **Characterize first:** Capture verbose and non-verbose progress order, dry-run summaries, failure output, response coexistence, and token/secret absence.
- **Scope:** V2 start/planning, runner/use-case progress, focused V2 operations, reporter event vocabulary, and command composition.
- **Non-goals:** A generic event bus, telemetry framework, workflow pipeline, response construction in application code, or V1 logging redesign.
- **Dependencies:** A small stable set of presentation facts; coordination with any concurrent active-runner changes.
- **Risks:** Over-modeling events can become a second orchestration API; output snapshots can hide ordering changes; an injected no-op must not suppress required protocol responses.
- **Acceptance criteria:** Active application/operation files do not import the terminal logger; composition owns formatting; existing output order and redaction pass characterization; use-case dependencies stay narrow; release decisions, journal writes, and unsafe operation order are byte/behavior unchanged.
- **Delivered commit boundaries:** DX1.1 `583702f` characterized output, stderr separation, verbose suppression, and secret absence; DX1.2 `aa8c478` introduced the typed progress port; DX1.3 `059e0b4` moved terminal rendering and Git diagnostics behind explicit adapters; DX1.4 documentation closure uses commit message `docs(release): complete dx1 progress reporting`.
- **Progress contract:** The reporter is synchronous and infallible: it returns no error, never selects policy, and cannot modify journals, Git, network, command results, or response schemas. No-op reporting is explicit through `releaseProgressOrNoop`.
- **Output compatibility:** Human progress still goes through the established plugin stderr stream, verbose progress still respects `log.Verbose` inside the terminal adapter, JSON stdout remains reserved for `plugin.Response`, and unknown progress events render nothing.
- **Secret safety:** Progress events contain no token/header/environment/body fields or arbitrary maps. Call sites pass sanitized remote display values and typed dispatch inputs; terminal tests prove unused secret-bearing fields are not rendered.

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
- **Dependencies:** Completed refactor planning seams; no dependency on C1 or C2. H3 is the recommended next overall milestone, but F1 may proceed independently when work does not overlap.
- **Risks:** Reusing a runner facade could accidentally construct mutating adapters; duplicated planning could drift; exposing filesystem details may create a new public contract.
- **Acceptance criteria:** Tests prove zero writes/process mutations/network/token reads; calculations match canonical release planning; selected paths are validated; results are typed until presentation; command and manifest contracts agree; V1 compatibility is explicit rather than inferred.
- **Expected commit boundaries:** (1) behavior/contract characterization; (2) typed query and pure planning reuse; (3) response mapping and command/manifest integration; (4) docs and architecture guards.

### F2 — Evaluate V2 local delivery

- **Objective:** Decide whether V2 local publication should exist and, if so, produce a reviewed executor-by-executor safety design before any implementation.
- **User value:** Users receive either a credible path to local V2 delivery or a clear explanation that GitHub Actions remains the supported owner.
- **Characterize first:** Pin the current explicit block, executor capabilities, release-it ownership of Git actions, known-file/state requirements, and all V2 journal/recovery invariants.
- **Scope:** Product use cases, executor feasibility, ownership of materialization/state/Git, state-in-commit proof, journal model, failure matrix, and migration/compatibility impact.
- **Non-goals:** Activating `ReleaseTransaction`, reintroducing the removed `GitReleaseCoordinator.Coordinate` convenience path, implementing code, weakening GitHub Actions safety, or treating an executor subprocess as safely recoverable without proof.
- **Dependencies:** Product approval, executor-specific experiments, and architecture review. A positive result creates a later implementation milestone; a negative result unlocks a later cleanup milestone for retained V2-local scaffold.
- **Risks:** Some executors own commit/tag/push internally and cannot satisfy exact known-file or interruption guarantees; two active publication owners can drift.
- **Acceptance criteria:** The decision is recorded per executor; any positive design proves exact state inclusion, commit/tag/push ownership, pending evidence, resume/refusal semantics, and token boundaries; no production path is activated by this milestone.
- **Expected commit boundaries:** (1) characterization; (2) executor feasibility evidence; (3) architecture decision record; (4) roadmap update selecting a new feature milestone or C2 removal.

## Intentionally deferred and preserved

The following are not backlog omissions. They remain deliberate boundaries until separately authorized evidence changes the decision:

- no blind retry of ambiguous push or uncertain workflow dispatch;
- no remote-state inference that converts an observation into proof of safe retry;
- no generic workflow pipeline, universal state-machine engine, dependency bag, or shared journal repository;
- no V1 removal beyond C2's completed removal record without fresh consumers, policy, replacements, and deprecation evidence;
- no V2 local execution before F2 and a subsequent implementation milestone prove executor ownership and recovery;
- no journal schema migration or repair command without a concrete format/support need and an inspection-first design;
- no process-wide cwd rewrite before an embedding/concurrency requirement justifies DX2;
- no plugin-index confinement change before DX3 resolves the product policy.

## Roadmap completion reporting

Roadmap progress is reported independently from the historical refactor ledger: “Refactor stages: 9 / 9 completed; roadmap milestones H1, H2, H3, C1, C2, and DX1: completed; next milestone: DX2 — Make command roots explicit for embedders.” H1, H2, H3, C1, C2, DX1, and later roadmap milestones must not be reported as Stage 10.

Each milestone begins with characterization, retains independently revertible commit boundaries, runs the full uncached Release Plugin and repository validation required by `plugin/release/RULES.md`, and updates this roadmap plus the architecture review when evidence changes a priority or recommendation.
