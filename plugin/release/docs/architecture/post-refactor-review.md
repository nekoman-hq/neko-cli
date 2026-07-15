# Release Plugin Post-Refactor Architecture Review

## Review status and evidence

This document records the architecture verified after completion of the nine-stage Release Plugin refactor. It describes production code in commit `440eecc` and later review-only documentation commits; it is not an extension of the completed refactor plan.

The review followed the command composition in `plugin/release/main.go`, every production Go file under `plugin/release`, all architecture guard tests, the public package surfaces, and repository-wide references to retained compatibility symbols. The completed Stage 9 sequence was verified in Git history:

```text
fa55952 test(release): characterize v1 compatibility contracts
ab72785 refactor(release): isolate v1 compatibility use cases
55d5962 refactor(release): isolate v1 compatibility adapters
440eecc docs(release): complete release plugin refactor
```

The authoritative historical ledger remains in [refactor-plan.md](refactor-plan.md). Detailed behavioral invariants and disk contracts remain in [current-state.md](current-state.md).

## Verified dependency direction

The production dependency direction is:

```text
command presentation
    ↓
application use cases
    ↓
consumer-owned capabilities
    ↓
concrete adapters
```

Canonical models and pure decisions are consumed across those layers; they do not depend upward on command presentation or concrete adapters. Composition occurs at command or public-facade boundaries. Active application results contain release facts or classified failures, not `plugin.Response` values.

Special allowed directions are narrower than the general flow:

- release, init, migration, queries, and plugin-index may consume canonical models and validation from `pkg/config`;
- migration may consume canonical V1 read models, loading, validation, and the canonical V2 pair persister, but it does not import V1 execution, executor, Git-mutation, or rollback internals;
- read-only queries may consume command-owned read adapters backed by `pkg/config` and `pkg/git`, but their use cases expose no mutation capability;
- V1 executor packages consume the V1 executor request and focused V1 adapter contracts from `pkg/release`; they do not select V1 versus V2;
- the explicit `pkg/release/tool` compatibility aggregator imports concrete V1 executors to populate the legacy registry. Production `main` does not import that aggregator.

No package move or idealized layer tree is implied by this direction. The boundaries are types, functions, and consumer-owned interfaces inside the existing packages.

## Production composition and command presentation

`main.main` remains the stdin/stdout protocol and composition root. It decodes `plugin.Request`, applies process-wide metadata and verbose mode, resolves and changes to the project root, constructs the three V1 executors, routes one command, converts unexpected Go errors to fatal protocol output, and encodes one `plugin.Response`.

The verified command boundaries are:

| Command area | Presentation boundary | Application boundary | Response owner |
| --- | --- | --- | --- |
| `patch`, `minor`, `major` | `releaseCommandHandler` and `ParseReleaseCommandRequest` | `releaseStartOperation`, then the selected V1 or V2 application | `MapReleaseCommandOutcome` and `MapCommandFailure` |
| `resume` | `resumeCommandHandler` and `ParseResumeCommandRequest` | `resumeReleaseUseCase` | `MapResumeCommandOutcome` and `MapCommandFailure` |
| `init`, `unit-add` | `HandleInit` / `HandleUnitAdd` plus command-specific parsers | `initializeV2RepositoryUseCase` / `addV2ReleaseUnitUseCase` | `response_mapper.go` in `pkg/init` |
| `migrate` | `migrationCommandHandler` and `parseMigrationCommandRequest` | `migrationUseCase` and `migrationPlanExecution` | `pkg/migrate/response_mapper.go` |
| `validate` | `validateCommandHandler` and `parseValidationQueryRequest` | `validationQueryUseCase` | `pkg/validate/response_mapper.go` |
| `history` | `historyCommandHandler` and `parseHistoryQueryRequest` | `historyQueryUseCase` | `pkg/history/response_mapper.go` |
| `contributors` | `contributorsCommandHandler` and `parseContributorsQueryRequest` | `contributorsQueryUseCase` | `pkg/contributors/response_mapper.go` |
| `plugin-index` | `pluginIndexCommandHandler` and `parsePluginIndexCommandRequest` | `generatePluginIndexUseCase` | `pkg/pluginindex/response_mapper.go` |
| `init-options` | `GetAvailableOptions` | none; this is a static presentation query | `GetAvailableOptions` |

Raw `plugin.Request` and flag-map access are confined to the presentation handlers and parser files. The static `init-options` command directly constructs a response because it has no application or infrastructure work.

## Active V2 release ownership

`releaseStartOperation` loads the normalized repository and delegates exactly one release source decision to `selectReleaseApplicationPath`. `v2ReleaseCommandApplication` resolves the selected unit and builds a V2-only `ReleaseExecutionContext`.

V2 dry-run is owned by `planV2Release`. It validates requirements, builds the version plan, plans and validates materialization, derives `KnownReleaseFiles`, and builds the immutable dispatch summary. It resolves no token and invokes no mutation adapter.

V2 GitHub Actions execution is owned by `githubActionsReleaseUseCase.Run`. Its consumer-owned capabilities make the safety order explicit:

1. resolve the typed dispatch token;
2. plan materialization and known files;
3. validate Git, workflow, and unresolved-journal preconditions;
4. prepare the execution journal;
5. apply materialization under a pending marker and confirm it;
6. update selected-unit state under a pending marker and confirm it;
7. stage the exact allowlist under a pending marker and confirm it;
8. create and verify the release commit under a pending marker and confirm it;
9. create and verify the unit tag under a pending marker and confirm it;
10. prepare the dispatch journal under a pending marker and confirm it;
11. push the commit under a pending marker and confirm it;
12. push the tag under a pending marker and confirm it;
13. persist `request-started`, dispatch, and classify the result;
14. confirm handoff only after accepted dispatch.

`GitHubActionsReleaseRunner` is the public production facade. It validates the request, owns default production composition through explicit fields, and invokes the use case. It does not own the mutation order.

V2 local non-dry-run remains blocked in `startV2Release`, and `ReleaseTransaction.Execute` independently refuses execution. No local executor is active in the V2 path.

## V2 resume ownership

`resumeReleaseUseCase` owns discovery, local evidence assessment, dry-run return, pure recovery resolution, context reconstruction, and selection of one named continuation. It does not calculate a new version or identity.

`resolveResumeRecovery` maps execution-journal evidence to one supported operation or refusal. The named operations resume from `commit-created`, `tag-created`, or `tag-pushed`, or return an already completed handoff. They reuse the active release tag, dispatch preparation, commit push, tag push, workflow dispatch, and handoff capabilities.

Remote completion is not inferred. Ambiguous push markers, unproven pushes, corrupt evidence, and terminal or uncertain dispatch journals remain blocked. Accepted dispatch reuse does not resolve a token or send another request.

## Init and unit-add ownership

`initializeV2RepositoryUseCase` and `addV2ReleaseUnitUseCase` are separate intentions. Pure policy selects whether creation or append is allowed; pure constructors build normal or plugin units and complete config/state pairs. Both use cases validate a complete pair before passing it to `config.V2ReleasePairPersister`.

`V2ReleasePairPersister` is the sole config/state pair writer used by init, unit-add, and migration. It prepares both target-local temporary files before replacing either target and attempts exact restoration of both prior snapshots after a returned replacement failure. The contract is rollback-backed pair persistence, not cross-file crash atomicity.

## Read-only query ownership

Validate, history, and contributors retain distinct command intentions and consumer-owned read capabilities:

- validation loads the canonical repository; V2 remains token-free, while V1 deliberately invokes the legacy requirements contract;
- history uses repository-wide legacy tag/count behavior for V1 and exact `TagSpec` plus selected paths for V2;
- contributors uses repository-wide shortlog for V1 and selected-unit path filtering for V2.

Their use cases return typed facts and failures, do not construct responses, and receive no mutation capability. Source-format checks in these queries select format-specific read semantics; they are not release-workflow selection.

## Plugin-index ownership

`generatePluginIndexUseCase` exposes the explicit `query -> build -> persist` sequence. `pluginIndexQueryUseCase` loads and validates config, state, and plugin manifests and orders entries by plugin name. `jsonPluginIndexOutputBuilder` creates complete stable bytes. `atomicPluginIndexOutputPersister` alone performs the requested single-file effect.

Check mode stops after the query. Render mode stops after byte construction. Output mode passes complete bytes and the unchanged requested path to the persister. Public `Generate`, `Write`, and `WriteWithOptions` remain narrow compatibility surfaces over the canonical query and builder.

## Migration ownership

`migrationUseCase` resolves one repository root and one immutable plan. Pure policy classifies filesystem and journal evidence. `migrationPlanExecution` exposes a fixed order through focused journal, pair-persistence, target-verification, source-archive, and backup-verification capabilities.

Migration uses `config.V2ReleasePairPersister`, verifies exact target bytes and strict V2 validity before archiving V1, verifies the byte-identical backup after archive, and removes the migration journal last. Its worktree journal is intentionally separate from the Git-common-dir execution and dispatch journals.

## V1 compatibility ownership

Production `main` explicitly constructs `goreleaser.NewV1Executor`, `jreleaser.NewV1Executor`, and `releaseit.NewV1Executor` and passes them to `HandleReleaseWithV1Executors`. The fixed catalog selects one executor without mutable registry lookup.

The V1 application owns typed intent, preview/execution requests, pure planning, classified failures, and visible execution order. Focused capabilities own requirements, preflight, version evidence, config materialization, Git writes, rollback, process invocation, file inspection, environment/token access, and JReleaser time.

V1 rollback remains intentionally separate from V2 recovery. It may delete a GitHub Release and tags, create and push a revert or fallback commit, hard-reset an unpushed release commit, and clean untracked files according to recorded evidence. It is best-effort compensation without a durable journal.

## Git ownership

Active V2 release and resume use one `GitReleaseCoordinator` instance per composed operation graph. Consumer-owned adapters expose only the preflight, stage, commit, tag, push, verification, or inspection methods needed by each operation. `GitReleaseCoordinator.Coordinate` is not selected by production.

V1 uses `SystemV1GitWriter`, `V1ReleaseRollback`, and the V1 preflight repository adapter because its commit, push, and destructive-compensation contracts differ from V2. Read-only query adapters use `pkg/git` for their legacy and selected-path reads. Migration has a migration-owned Git-root resolver.

These owners are intentionally distinct; no active flow has two selectable Git implementations for the same intention.

## Journal ownership

`ReleaseExecutionJournalStore` exclusively owns execution-journal discovery, validation, pending-action writes, and confirmed-phase writes. `DispatchJournalStore` exclusively owns immutable dispatch-journal preparation, lookup, transitions, and terminal-state resolution.

`releaseJournalFiles` shares only identical mechanics: Git common-dir resolution, fixed directories, canonical JSON, private directory creation, and atomic `0600` replacement. It is not a generic journal repository.

Migration owns a separate compatible worktree journal because its source/target recovery evidence and lifecycle differ from release execution.

## Token and clock ownership

`EnvironmentGitHubActionsDispatchTokenResolver` is the production V2 `GITHUB_TOKEN` reader. It returns `GitHubActionsDispatchToken`; only dispatch adapters unwrap the value, and string formatting remains redacted.

V1 keeps its legacy token/environment behavior behind executor- and rollback-owned adapters. GoReleaser and release-it pass the legacy environment with redaction. JReleaser maps the legacy token to `JRELEASER_GITHUB_TOKEN` and uses an injected year clock.

`ReleaseClock` supplies active release/resume response timestamps and V2 execution/dispatch persistence. Query commands own their response clocks. Model-level zero-time fallbacks and public constructors with system defaults remain compatibility behavior for direct callers. V1 execution persists no timestamp.

## Config, state, and materialization ownership

`pkg/config` owns V1/V2 disk models, strict loading and validation, normalized `ReleaseRepository`/`ReleaseUnit` read models, tag specifications, atomic single-file replacement, canonical V1 writing, and canonical V2 pair persistence.

For active V2 release, config is immutable architecture and state owns selected-unit versions. `VersionMaterializer` plans executor-specific file changes, `MaterializationTransaction` applies and restores declared changes before uncertainty, `StateTransaction` updates and restores V2 state, and `KnownReleaseFiles` defines the exact commit allowlist.

Plugin manifest materialization derives its path only from validated `ReleaseUnit` metadata. No active hard-coded plugin-unit registry remains.

## Remaining compatibility entry-point families

The following families remain outside canonical production composition or wrap it for compatibility. The debt register added by the next review milestone records each qualified symbol, consumer, behavior, and recommendation.

- registry-backed release entry: `HandleRelease`, `Register`, `Get`, and the opt-in `pkg/release/tool` aggregator;
- legacy V1 command and tool surfaces: `Service`, `Preflight`, `Tool`, `ToolBase`, and executor `Init` / `Execute` / `Release` / `RevertRelease` methods;
- mixed-model builder and old internal bridges: `BuildReleaseExecutionContext`, `startLegacyRelease`, and `newV1ReleaseCommandApplication`;
- inactive V2-local scaffolding: `ReleaseTransaction`, its preparation path, and `GitReleaseCoordinator.Coordinate`;
- public query/migration wrappers: migration `ResolvePlan` / `Run` and plugin-index `Generate` / `Write` / `WriteWithOptions`;
- public constructors with system defaults for release/dispatch journals, dispatch, Git coordination, and the active runner.

## Architecture audit result

The verified architecture contains no active god function, replacement god object, generic workflow pipeline, generic state-machine engine, boolean V1/V2 workflow selector, broad dependency container, application-owned `plugin.Response`, raw flag access outside parsers, scattered active release source selection, or duplicated active release/journal infrastructure ownership.

One active deviation from the strict presentation rule remains: `releaseStartOperation.Start`, `planV2Release`, `GitHubActionsReleaseRunner.Run`, `githubActionsReleaseUseCase.Run`, and focused V2 release operation adapters call the package-global terminal logger directly. The release decisions and response models remain typed, and the calls do not alter safety ordering, but progress presentation is not supplied as an explicit boundary. This is a bounded architecture violation, not an intentional compatibility requirement. It does not invalidate the historical fact that all nine planned stages were completed; it does require an explicit post-refactor recommendation rather than a claim of zero remaining architectural debt.

The refactor ledger therefore remains closed at 9 / 9. Future safety, compatibility, developer-experience, and feature work belongs in the separate post-refactor roadmap, not in a Stage 10.

## Prioritized debt register

Priority means:

- **P0** — release safety or data-loss risk requiring immediate correction;
- **P1** — significant correctness or maintainability risk;
- **P2** — bounded architectural or compatibility debt;
- **P3** — optional cleanup.

No P0 issue was found. The active V2 GitHub Actions path preserves evidence around uncertain operations, blocks ambiguous retry, and has a complete pending-action boundary around its unsafe mutations. The P1 items below are real risks but are either confined to V1 execution or require a process/machine interruption during pair persistence or migration.

### D-01 — Destructive V1 compensation is best-effort and non-journaled

- **Affected files or symbols:** `pkg/release/v1_git_adapters.go`; `V1ReleaseRollback.Rollback`; `GitReleaseState`; executor `Rollback` / `RevertRelease` methods.
- **Current behavior:** After executor failure, V1 may delete a GitHub Release, delete local and remote tags, create and push a revert or fallback commit, hard-reset an unpushed commit, and clean untracked files. In-memory flags guard the order, but no durable intent or confirmation is written.
- **Why it remains:** These destructive semantics are characterized V1 compatibility behavior. Stage 9 isolated them without authorizing a behavior or recovery-format change.
- **Risk:** A crash, interruption, stale in-memory flag, or partial compensation can leave remote and local state divergent. Later cleanup may lack enough evidence to determine which actions completed.
- **User or developer impact:** A failed V1 release can require manual Git/GitHub recovery and can remove untracked work as part of the legacy contract.
- **Removal or improvement preconditions:** Decide the supported lifetime of V1; characterize interruption evidence and all remote/local partial states; define a compatible durable record or explicitly narrow destructive compensation; document manual recovery and migration guidance.
- **Recommended action:** **Harden**. Do not broaden compensation. Prefer durable evidence and explicit refusal when completion is uncertain.
- **Priority:** **P1**.
- **Proposed milestone:** **H1 — Make V1 compensation interruption-safe**.
- **Blocking new features:** Blocks new V1 release execution or rollback features. It does not block read-only work or unrelated V2 features.

### D-02 — Legacy GitHub Release deletion has an unbounded global HTTP client

- **Affected files or symbols:** `pkg/git/repository.go`; `DeleteGithubRelease`; `http.DefaultClient`; `legacyV1GitHubReleaseClient`; `systemV1GitHubReleaseRemover`.
- **Current behavior:** The release-owned rollback adapter is replaceable, but its production compatibility client changes the process cwd, discovers the GitHub remote there, and performs GET/DELETE requests through `http.DefaultClient`. Error bodies are read without a configured limit before the outer adapter redacts the token.
- **Why it remains:** Stage 9 preserved the legacy GitHub API behavior beneath a testable release-owned boundary and did not change network semantics.
- **Risk:** Rollback can hang without a client timeout, consume an unexpectedly large error body, or be influenced by global HTTP-client mutation. Process cwd remains an implicit input to remote selection.
- **User or developer impact:** V1 recovery may stall or report an oversized third-party error while other compensation is stopped.
- **Removal or improvement preconditions:** Pin status/body compatibility, inject a bounded client and explicit repository target, retain token redaction and idempotent 404 handling, and prove no real network use in tests.
- **Recommended action:** **Harden** within the V1 rollback boundary.
- **Priority:** **P1**.
- **Proposed milestone:** **H1 — Make V1 compensation interruption-safe**.
- **Blocking new features:** Blocks expansion of V1 remote compensation. It does not block V2 dispatch or read-only features.

### D-03 — V2 config/state pair persistence is not crash-atomic

- **Affected files or symbols:** `pkg/config/v2_pair_persister.go`; `V2ReleasePairPersister.Persist`; `restoreV2File`; init, unit-add, and migration pair writers.
- **Current behavior:** Both temporary files are fully prepared before config then state replacement. Returned replacement failures attempt exact restoration of both snapshots. A process, kernel, machine, or filesystem failure between successful renames can expose a mixed config/state pair.
- **Why it remains:** Portable filesystems do not offer one atomic rename for two independent files. The completed refactor deliberately claimed bounded rollback rather than false cross-file atomicity.
- **Risk:** A crash window can leave a pair that strict loading rejects, and an interrupted create can leave one target without the other.
- **User or developer impact:** The next command can fail validation and require manual restoration even though no Go error was returned to the interrupted process.
- **Removal or improvement preconditions:** Select and document a crash-recovery protocol, such as a pair journal or generation/manifest pointer; preserve exact bytes, modes, schemas, and existing restoration behavior; add process-interruption recovery tests.
- **Recommended action:** **Harden**, without claiming impossible cross-file atomicity.
- **Priority:** **P1**.
- **Proposed milestone:** **H2 — Make pair and migration crash recovery explicit**.
- **Blocking new features:** Blocks features that add more multi-file config/state mutations. It does not block read-only features.

### D-04 — Migration retains effect/journal crash windows

- **Affected files or symbols:** `pkg/migrate/execution.go`; `migrationPlanExecution.Execute`; `filesystemMigrationJournalOperations`; `filesystemMigrationSourceArchiver`; `release.migration.json`.
- **Current behavior:** Migration journals intent, persists and verifies the V2 pair, archives V1, verifies the backup, and removes the journal. A crash can occur after pair replacement or source rename but before journal confirmation. Recovery classifies compatible hashes and evidence, but it does not universally repair arbitrary corruption.
- **Why it remains:** Stage 8 isolated and characterized deterministic returned-error recovery without introducing a generic transaction engine or changing the journal schema.
- **Risk:** Interrupted pair replacement can expose mixed targets; interruption around source archival can leave the journal behind its real effect. Hash evidence handles supported states but not missing or externally modified evidence.
- **User or developer impact:** A migration can require manual recovery; the V1 source or backup may be the only trustworthy copy.
- **Removal or improvement preconditions:** Align with the pair crash-recovery protocol, enumerate crash points, retain byte-identical source evidence, and define refusal/manual-recovery behavior for every unsupported state before any schema change.
- **Recommended action:** **Harden** with evidence-driven recovery, not automatic guessing.
- **Priority:** **P1**.
- **Proposed milestone:** **H2 — Make pair and migration crash recovery explicit**.
- **Blocking new features:** Blocks expansion to multi-unit or nested migration and any migration schema change. It does not block unrelated release features.

### D-05 — Compatibility tool registry is mutable and process-global

- **Affected files or symbols:** `pkg/release/registry.go`; package variable `tools`; `Register`; `Get`; `registeredV1ReleaseExecutorCatalog`; `directV1ReleaseExecutorCatalog`; `pkg/release/tool/register.go`.
- **Current behavior:** Importing `pkg/release/tool` runs three side-effect registrations. Re-registration silently overwrites by name. The map has no synchronization. Production `main` uses a fixed catalog and never imports the aggregator.
- **Why it remains:** External Go callers may still rely on the exported registry and blank-import registration behavior.
- **Risk:** Direct concurrent callers can race, tests can leak registration state, and import side effects obscure compatibility composition.
- **User or developer impact:** This primarily affects embedders and tests; the plugin executable's release path is unaffected.
- **Removal or improvement preconditions:** Audit downstream imports, publish a deprecation window, offer explicit executor composition to embedders, and retain characterized lookup/overwrite behavior until removal.
- **Recommended action:** **Replace** with explicit composition for all known consumers, then remove after compatibility policy permits.
- **Priority:** **P2**.
- **Proposed milestone:** **C1 — Decide and deprecate V1 compatibility surfaces**.
- **Blocking new features:** Not blocking product features; blocks adding new registry-based executors.

### D-06 — V1 version-evidence seams are mutable package globals

- **Affected files or symbols:** `pkg/release/version_guard.go`; `refreshVersionTags`; `latestVersionTag`; `legacyV1VersionEvidence`.
- **Current behavior:** Active V1 planning calls an adapter whose production implementation reaches two replaceable package variables. Tests replace those functions directly.
- **Why it remains:** The variables preserve the legacy version-guard seam and exact local-versus-refreshed evidence behavior.
- **Risk:** Parallel in-process calls or tests can race and observe the wrong evidence source; the dependency remains less explicit than the rest of the V1 application.
- **User or developer impact:** The single-request plugin process is low risk, but embedding or parallel tests are fragile.
- **Removal or improvement preconditions:** Retain two-pass evidence semantics, inject the evidence source at compatibility composition, and keep public `VersionGuard` behavior characterized.
- **Recommended action:** **Replace** the globals with explicit compatibility composition.
- **Priority:** **P2**.
- **Proposed milestone:** **C1 — Decide and deprecate V1 compatibility surfaces**.
- **Blocking new features:** Not blocking unrelated features; blocks parallel V1 execution support.

### D-07 — Working-directory ownership remains process-global

- **Affected files or symbols:** `pkg/workspace/root.go`; `ChangeToProjectRoot`; `ToolBase.InUnitRoot`; `inV1Repository`; relative-path command composition.
- **Current behavior:** `main` changes cwd once for every request. Compatibility helpers temporarily change cwd and restore it. Many adapters therefore receive `.` or an empty root as a meaningful legacy value.
- **Why it remains:** The plugin is a single-request executable, and V1 compatibility historically depends on current-directory behavior.
- **Risk:** In-process parallel use is unsafe, restoration errors are ignored, and implicit cwd makes ownership harder to inspect.
- **User or developer impact:** Normal CLI execution is bounded; embedders and parallel tests cannot safely invoke commands concurrently.
- **Removal or improvement preconditions:** Inventory every relative-path contract, pass explicit roots through public composition, and preserve nested-start-directory resolution semantics.
- **Recommended action:** **Defer** for the executable, then **Replace** only when an embedding or parallel-execution requirement justifies the compatibility change.
- **Priority:** **P2**.
- **Proposed milestone:** **DX2 — Make command roots explicit for embedders**.
- **Blocking new features:** Blocks safe in-process concurrency. It does not block single-request CLI features.

### D-08 — Inactive V2-local transaction and coordinator convenience paths remain

- **Affected files or symbols:** `pkg/release/release_transaction.go`; `ReleaseTransaction`; `prepareReleaseFilesForCoordinator`; `MutationTracker`; `GitReleaseCoordinator.Coordinate`.
- **Current behavior:** `ReleaseTransaction.Execute` always blocks non-dry-run V2 execution. Its private preparation and rollback-before-uncertainty code is exercised only by tests. `GitReleaseCoordinator.Coordinate` is functional and tested but production uses named operation adapters instead.
- **Why it remains:** The refactor prohibited deleting public or potentially useful compatibility/scaffold code without a separate proof and support decision.
- **Risk:** Readers can mistake these paths for active orchestration, and future work may accidentally extend the wrong path or create a second release sequence.
- **User or developer impact:** No current command behavior is affected; maintenance and onboarding cost remain.
- **Removal or improvement preconditions:** Search downstream consumers, decide whether V2 local execution is a real product goal, preserve any public compatibility contract, and prove all active production call sites use named operations.
- **Recommended action:** **Remove** internal test-only preparation if local execution is rejected; otherwise keep it isolated as a deliberately redesigned future feature. Deprecate public convenience paths before removal.
- **Priority:** **P2**.
- **Proposed milestone:** **C2 — Retire superseded and inactive release paths**.
- **Blocking new features:** Blocks implementation of V2 local execution on the current scaffold. It does not block GitHub Actions features.

### D-09 — V2 local execution remains blocked

- **Affected files or symbols:** `startV2Release`; `V2_LOCAL_DELIVERY_BLOCKED`; `ReleaseTransaction.Execute`; executor capability and delivery models.
- **Current behavior:** V2 local non-dry-run returns an explicit block. GitHub Actions is the only active V2 publication owner.
- **Why it remains:** Executor ownership, inclusion of V2 state in the release commit, unsafe-operation journaling, and recovery are unresolved. release-it in particular owns commit/tag/push internally.
- **Risk:** Activating the current scaffold would bypass the proven V2 journal/recovery order or fail to guarantee committed state.
- **User or developer impact:** Users cannot execute V2 local releases; they must use GitHub Actions or remain on V1 compatibility.
- **Removal or improvement preconditions:** A separate product decision and design must prove ownership, state-in-commit, exact Git order, journaling, interruption recovery, and executor-specific feasibility.
- **Recommended action:** **Defer**. Keep the block until an explicit feature milestone is approved.
- **Priority:** **P2** as a product limitation, not an architecture regression.
- **Proposed milestone:** **F2 — Evaluate V2 local delivery**.
- **Blocking new features:** Blocks only V2 local-delivery features.

### D-10 — Remote-state inference and automatic uncertain-operation retry are intentionally absent

- **Affected files or symbols:** `resolveResumeRecovery`; `resolveResumeDispatch`; `AssessReleaseExecutionRecovery`; dispatch terminal-state policy.
- **Current behavior:** Resume trusts durable local intent and verified local evidence. Ambiguous pushes and request-started/unknown dispatch are refused; no remote probe converts uncertainty into permission to retry.
- **Why it remains:** Absence is the safety policy, not missing extraction. Remote observations may themselves be incomplete and do not prove whether retry is idempotent.
- **Risk:** Operators must perform manual inspection, but automatic duplicate push/dispatch is avoided.
- **User or developer impact:** Recovery is conservative and sometimes manual.
- **Removal or improvement preconditions:** Only a separately designed, provider-specific proof model with immutable request identity and explicit operator authorization could change this policy.
- **Recommended action:** **Keep**.
- **Priority:** **P2** as an intentional compatibility/safety boundary.
- **Proposed milestone:** none; preserve as a roadmap invariant unless a dedicated recovery feature is authorized.
- **Blocking new features:** Does not block read-only recovery guidance; blocks blind retry features by design.

### D-11 — Journals have no repair tool or schema-migration mechanism

- **Affected files or symbols:** `ReleaseExecutionJournalStore`; `DispatchJournalStore`; migration journal loading; fixed schema versions and strict validation.
- **Current behavior:** Invalid, conflicting, or unsupported journals are rejected and preserved. There is no command that edits evidence, migrates schemas, or clears pending actions.
- **Why it remains:** Guessing or rewriting recovery evidence would weaken safety. Existing schemas have not required migration yet.
- **Risk:** A corrupted journal can block automatic continuation indefinitely; a future schema change lacks a migration path.
- **User or developer impact:** Manual inspection is required, but evidence is not silently destroyed.
- **Removal or improvement preconditions:** Define immutable backups, an auditable repair plan, explicit operator confirmation, schema compatibility rules, and tests that never infer unsafe completion.
- **Recommended action:** **Defer** until a concrete schema change or support burden exists; then **Harden** with a separate inspection-first tool.
- **Priority:** **P2**.
- **Proposed milestone:** **H3 — Add evidence-safe journal inspection and lifecycle support**.
- **Blocking new features:** Blocks journal schema changes and automated repair features; not other product work.

### D-12 — Public compatibility wrappers remain broad

- **Affected files or symbols:** the complete compatibility inventory below, including `HandleRelease`, `Service`, `Preflight`, `Tool`, `ToolBase`, executor legacy methods, mixed context construction, query/migration facades, and system-default constructors.
- **Current behavior:** Some wrappers directly delegate; others preserve fatal exit, cwd, registry, default-construction, or legacy method-shape behavior. Production composition bypasses most V1 wrappers.
- **Why it remains:** The Go packages are public and no downstream compatibility-removal decision was authorized.
- **Risk:** Unknown consumers constrain signature cleanup and can make inactive code appear canonical.
- **User or developer impact:** Existing embedders retain source compatibility; maintainers carry extra surfaces and tests.
- **Removal or improvement preconditions:** Downstream reference audit, deprecation policy, replacement examples, release notes, and an explicit support/removal decision.
- **Recommended action:** **Keep** bounded wrappers now; **Remove** or **Replace** only through compatibility milestones with evidence.
- **Priority:** **P2**.
- **Proposed milestone:** **C1 — Decide and deprecate V1 compatibility surfaces**, followed by **C2** for proven removals.
- **Blocking new features:** Not generally blocking. New code must not depend on these wrappers.

### D-13 — Active V2 progress logging crosses the presentation boundary

- **Affected files or symbols:** `releaseStartOperation.Start`; `planV2Release`; `logV2DryRunPlan`; `GitHubActionsReleaseRunner.Run`; `githubActionsReleaseUseCase.Run`; `github_actions_release_operations.go`; package-global `log.Verbose`.
- **Current behavior:** Active V2 application and focused operations emit formatted terminal progress directly through `pkg/log`. Responses remain command-owned and typed, and log calls do not select workflow behavior.
- **Why it remains:** The refactor isolated response mapping and unsafe effects but retained existing progress output to preserve behavior and snapshots.
- **Risk:** Application tests require global log capture, presentation changes touch safety-oriented files, and the code violates the strict `RULES.md` direction even though business results remain separated.
- **User or developer impact:** No release correctness change is known; maintenance and test isolation suffer.
- **Removal or improvement preconditions:** Characterize required verbose/non-verbose output and secret absence, define a focused progress-event/reporting boundary, and ensure operation order remains visible without a generic event pipeline.
- **Recommended action:** **Replace** direct application logging with an explicit narrowly scoped reporter supplied by composition.
- **Priority:** **P2**, classified as the one active architecture violation found by this review.
- **Proposed milestone:** **DX1 — Isolate release progress reporting**.
- **Blocking new features:** Not blocking unrelated features. It should precede substantial edits to the active V2 orchestration files.

### D-14 — Plugin-index output accepts an arbitrary requested path without symlink confinement

- **Affected files or symbols:** `pkg/pluginindex/output_persister.go`; `atomicPluginIndexOutputPersister.Persist`; `--output` contract.
- **Current behavior:** The unchanged user-supplied path is created or atomically overwritten; existing mode is preserved. No repository confinement or symlink policy is applied.
- **Why it remains:** Arbitrary requested-path output is characterized behavior, and Stage 7 explicitly avoided a new path policy.
- **Risk:** A mistaken or adversarial path can overwrite a file outside the repository with the invoking user's permissions.
- **User or developer impact:** Flexible CI output is retained, but callers must validate their own path.
- **Removal or improvement preconditions:** Decide whether arbitrary output is a supported feature, characterize symlink behavior, and introduce an explicit opt-in or confinement policy without silently breaking CI.
- **Recommended action:** **Defer** until product policy is chosen; then **Harden** the boundary if repository confinement is desired.
- **Priority:** **P3**.
- **Proposed milestone:** **DX3 — Clarify generated-output path policy**.
- **Blocking new features:** Blocks features that assume plugin-index output is repository-confined; otherwise not blocking.

### D-15 — Failed new-pair creation can leave an empty `.neko` directory

- **Affected files or symbols:** `V2ReleasePairPersister.Persist`; `osV2PairPersistenceDisk.CreateDirectory`.
- **Current behavior:** The directory is created before snapshots and temporary files. A later returned failure cleans temporary files and restores targets but does not remove a newly created empty directory.
- **Why it remains:** The directory is harmless, and tracking/restoring parent-directory existence would add another compatibility effect to pair persistence.
- **Risk:** Cosmetic filesystem residue can confuse scripts that treat directory presence as configuration presence without checking files.
- **User or developer impact:** No valid config/state is fabricated; manual cleanup may be desired.
- **Removal or improvement preconditions:** Characterize directory-mode/existence behavior and ensure cleanup never removes a directory containing unrelated files.
- **Recommended action:** **Defer** or **Harden** alongside H2 if a safe empty-directory cleanup contract is proven.
- **Priority:** **P3**.
- **Proposed milestone:** **H2 — Make pair and migration crash recovery explicit**.
- **Blocking new features:** Not blocking.

### D-16 — Successive V2 release planning after completed-journal exclusion is less directly characterized

- **Affected files or symbols:** `ReleaseExecutionJournalStore.FindUnresolved`; selected-unit state update; `releaseStartOperation`; planner tests.
- **Current behavior:** `handoff-ready` journals are excluded, so a later release plans from committed V2 state. The primary release/recovery matrix is stronger than one end-to-end test of a second release after handoff.
- **Why it remains:** Existing unit and runner tests prove the component facts; the refactor did not add a separate repeated-release scenario.
- **Risk:** A future journal-filter or state-loading change could reopen a completed execution or calculate from stale state.
- **User or developer impact:** Potential regression would affect repeated releases, not the first release.
- **Removal or improvement preconditions:** Add an end-to-end temporary-repository scenario that completes one handoff, reloads state, and plans the next version without redispatching the prior journal.
- **Recommended action:** **Harden** with focused characterization before changing journal exclusion or release planning.
- **Priority:** **P3**.
- **Proposed milestone:** **H3 — Add evidence-safe journal inspection and lifecycle support**.
- **Blocking new features:** Blocks changes to completed-journal lifecycle or repeated-release planning; otherwise not blocking.

### D-17 — Superseded raw Git helpers have no internal production consumers

- **Affected files or symbols:** `pkg/git/repository.go`; `LastCommit`, `TotalCommits`, `FilesCount`, `RepoSize`, `Head`, `CleanUntracked`, `DeleteLocalTag`, `DeleteRemoteTag`, `RevertCommit`, `CreateCommit`, and `HardResetTo`.
- **Current behavior:** These exported cwd-based helpers remain compiled. Repository-wide search found no production consumer for them; active V1 rollback uses `systemV1RollbackGit`, while `cmd/version.go` still consumes `git.Current` and query paths consume other `pkg/git` reads.
- **Why it remains:** Exported symbols may have downstream consumers, and Stage 9 did not authorize compatibility deletion.
- **Risk:** The destructive helpers can be mistaken for the canonical V1 rollback adapter and bypass repository-root, environment-redaction, and test seams.
- **User or developer impact:** No current command uses them, but external embedders may.
- **Removal or improvement preconditions:** Audit downstream imports, deprecate exact symbols, retain `Current` and actively used query/preflight functions, and prove no generated/plugin entry point references them.
- **Recommended action:** **Remove** only after the compatibility window; until then mark them explicitly non-canonical.
- **Priority:** **P3**.
- **Proposed milestone:** **C2 — Retire superseded and inactive release paths**.
- **Blocking new features:** Not blocking; new code must not call them.

## Compatibility wrapper inventory

The inventory distinguishes the plugin executable's active composition from public Go compatibility. “No active consumer” means no production call site was found in this repository; exported symbols still carry unknown downstream-consumer risk.

| Symbol or family | Package | Active internal consumers | Compatibility purpose | Behavior or delegation | Removal risk | Recommendation |
| --- | --- | --- | --- | --- | --- | --- |
| `release.HandleRelease` | `pkg/release` | none; command and dry-run tests use it | Preserve the original patch/minor/major entry | Composes the registry-backed V1 catalog, then delegates to the canonical handler/start operation | High: exported command API and V1 behavior | Keep for C1; production remains on `HandleReleaseWithV1Executors` |
| `release.Service`, `NewReleaseService`, `NewReleaseServiceWithContext`, `Service.Run`, `Service.GetNewVersion` | `pkg/release` | none; compatibility tests only | Preserve the former V1 service API | Delegates to isolated planning/application, but retains catalog selection, cwd fallback, logging, and fatal-exit mapping | High | Keep until downstream audit; deprecate in C1 |
| `release.Preflight` | `pkg/release` | none; compatibility tests only | Preserve fatal V1 preflight entry | Delegates checks to focused preflight then invokes legacy fatal output on failure | High because exit behavior is observable | Keep until C1 support decision |
| `release.Tool` | `pkg/release` | compatibility catalogs and registry only | Preserve the legacy executor interface | Interface only; its method shape drives compatibility adapters | High | Keep while registry/service are supported; do not use in new code |
| `ToolBase.ValidateRequirements`, `ResolveFiles`, `InUnitRoot`, `RequireBinary`, `RevertGitRelease`, `DeleteGitHubRelease`, `CreateReleaseCommit`, `CreateGitTag`, `PushCommits`, `PushGitTag` | `pkg/release` | executor compatibility methods and tests; not active fixed-catalog `Run` flow | Preserve shared legacy tool methods | Direct delegates except cwd-changing `InUnitRoot`; constructs system adapters with legacy empty-root semantics | High for external tool implementations | Keep bounded; deprecate with `Tool` in C1 |
| `release.Register`, `release.Get`, package variable `tools`, and `pkg/release/tool.init` | `pkg/release`, `pkg/release/tool` | registry-backed compatibility entry points only | Preserve registration and blank-import discovery | Contains mutable overwrite/lookup behavior and import side effects | High | Replace known consumers with explicit composition; remove only after C1 |
| `GoReleaser.Init`, `Execute`, `Release`, `RevertRelease`, `ValidateRequirements`, `ResolveFiles` | `pkg/release/tool/goreleaser` | `Rollback` delegates to `RevertRelease`; other methods serve `Tool` compatibility | Preserve concrete legacy tool API | `Execute` delegates to `Run`; `Release` delegates to shared release logic; `RevertRelease` owns current rollback mapping | High | In C1 invert rollback so canonical `Rollback` owns behavior, then keep direct wrappers until deprecation ends |
| `JReleaser.Init`, `Execute`, `Release`, `RevertRelease`, `ValidateRequirements`, `ResolveFiles` | `pkg/release/tool/jreleaser` | same pattern as GoReleaser | Preserve concrete legacy tool API | Direct compatibility delegates around the canonical executor logic; `RevertRelease` currently owns rollback mapping | High | Same C1 recommendation |
| `ReleaseIt.Init`, `Execute`, `Release`, `RevertRelease`, `ValidateRequirements`, `ResolveFiles` | `pkg/release/tool/releaseit` | same pattern as GoReleaser | Preserve concrete legacy tool API | Direct compatibility delegates around the canonical executor logic; `RevertRelease` currently owns rollback mapping | High | Same C1 recommendation |
| `release.BuildReleaseExecutionContext` | `pkg/release` | tests only; production uses `BuildV2ReleaseExecutionContext` | Preserve a mixed V1/V2 normalized-repository builder | Contains source-specific unit-root selection, then delegates to common assembly | Medium to high | Deprecate in C1; retain until external callers migrate |
| `release.startLegacyRelease`, `newV1ReleaseCommandApplication` | `pkg/release` | compatibility tests only | Preserve internal test/transition entry points from the old path | Delegate to the isolated V1 application | Low internally | Safe removal candidates in C2 after tests move to canonical entry points |
| `config.V1Exists`, `V1LoadConfig`, `V1SaveConfig` | `pkg/config` | compatibility tests; canonical code uses explicit-root/path variants | Preserve cwd-based V1 config API | Direct delegates to `V1ConfigExistsAt`, `V1LoadConfigAt`, and `V1SaveConfigAt` | High because exported and deprecated | Keep through C1; use explicit variants in new code |
| `release.VersionGuard`, `VersionGuardWithOptions`, `EnsureVersionIsValid` | `pkg/release` | compatibility tests; active V1 application uses its planning evidence adapter | Preserve legacy version-guard API and warnings | Contains compatibility behavior over mutable evidence globals | Medium to high | Characterize and deprecate in C1; do not remove pure version validation without consumer audit |
| `release.V2ExecutionUnavailableResponse` | `pkg/release` | none found | Preserve an exported response helper from command extraction | Directly maps a classified failure through the canonical response mapper | Medium unknown-consumer risk | Deprecate and remove in C2 if downstream audit is clean |
| `release.ReleaseTransaction`, `NewReleaseTransaction`, `ReleaseTransaction.Execute` | `pkg/release` | tests only | Preserve inactive V2-local scaffold/public shape | Constructor builds preparation state; `Execute` always blocks | High because exported despite inactive behavior | Decide product direction first; deprecate/remove in C2 if local delivery is rejected |
| `GitReleaseCoordinator.Coordinate` | `pkg/release` | tests only; active release/resume use focused methods through adapters | Preserve the earlier one-call Git sequence | Contains a complete stage/commit/tag/push convenience sequence | Medium to high | Deprecate in C2 to prevent a competing orchestration path |
| `migrate.ResolvePlan`, `migrate.Run`, exported `migrate.Plan` | `pkg/migrate` | tests only; command uses `migrationUseCase` | Preserve programmatic migration preview/execution | Narrow facades over canonical root/plan/use-case paths | Medium | Keep unless a public API policy removes them; they do not violate direction |
| `pluginindex.Generate`, `Write`, `WriteWithOptions` | `pkg/pluginindex` | tests and possible programmatic callers; command uses injected query/builder | Preserve programmatic index generation/serialization | Direct delegates to canonical query and builder | Low to medium | Keep; these are cohesive public APIs, not urgent debt |
| `NewReleaseExecutionJournalStore`, `NewDispatchJournalStore`, `NewGitHubActionsDispatcher`, `NewGitHubActionsReleaseRunner`, `NewGitReleaseCoordinator` and their options | `pkg/release` | active production composition plus tests/direct callers | Provide system-default production adapters and substitution options | Construct explicit system defaults; active use cases receive their resulting capabilities | Low | Keep; review only if a constructor hides new infrastructure in application code |

## Dead, inactive, and redundant production code inventory

No code is labeled dead solely from an IDE result. Classification uses production call sites, tests, package initialization, repository-wide imports, and exported-symbol risk.

| Code | Reachability evidence | Classification | Recommendation |
| --- | --- | --- | --- |
| `release.startLegacyRelease`, `newV1ReleaseCommandApplication` | referenced only by V1 compatibility tests | **Test support** and **Safe removal candidate** | Move tests to public/canonical boundaries, then remove in C2 |
| `registeredV1ReleaseExecutorCatalog`, `registeredV1ReleaseExecutor`, `directV1ReleaseExecutorCatalog`, `directV1ReleaseExecutor` | reached only from `HandleRelease` / `Service`, not production `main` | **Internal compatibility** | Keep while those public facades remain; remove with them |
| `pkg/release/tool` package initializer | reached only when deliberately imported; production does not import it | **Public compatibility** | Keep as explicit opt-in until registry deprecation completes |
| `ReleaseTransaction.Execute` | production-callable but always blocked; tests cover the refusal | **Future feature scaffold** and **Unknown consumer risk** | Do not extend; decide F2 versus C2 |
| `ReleaseTransaction.prepareReleaseFilesForCoordinator`, callback hooks, `ensureGitClean`, and `unstageKnownFiles` | private and referenced only by transaction tests | **Test support**, **Future feature scaffold**, and **Safe removal candidate** | Remove in C2 if F2 rejects the scaffold; otherwise redesign before activation |
| `GitReleaseCoordinator.Coordinate` | no production call site; coordinator's focused methods are active | **Public compatibility** and **Unknown consumer risk** | Deprecate before removal; never call from new production code |
| `V2ExecutionUnavailableResponse` | no repository call site | **Public compatibility** and **Unknown consumer risk** | Candidate for C2 after downstream audit |
| `buildV2InitConfigFromFlags` | private and referenced only by `pkg/init/handler_test.go` | **Test support** and **Safe removal candidate** | Move the test to typed parser/constructor coverage, then remove |
| `pkg/git` raw metrics helpers `LastCommit`, `TotalCommits`, `FilesCount`, `RepoSize` | no repository production consumers | **Unknown consumer risk** and **Safe removal candidate** | Deprecate and remove in C2 after import audit |
| `pkg/git` raw destructive helpers `Head`, `CleanUntracked`, `DeleteLocalTag`, `DeleteRemoteTag`, `RevertCommit`, `CreateCommit`, `HardResetTo` | no repository production consumers; active rollback uses focused root-aware adapters | **Superseded implementation**, **Unknown consumer risk**, and **Safe removal candidate** | Mark non-canonical now; deprecate/remove in C2 |
| `BuildReleaseExecutionContext` mixed builder | tests and potential external callers only; production selected V2 path uses the V2-only builder | **Public compatibility** | Keep through C1; remove only after caller migration |
| V2 executor capability and delivery facts describing local execution | read by planning and the blocked transaction but do not activate execution | **Future feature scaffold** | Keep only while F2 remains a plausible product decision |

## Clean-code audit classification

### No issue

- command handlers keep raw request/flag access at parser boundaries and map typed results at presentation boundaries;
- release source selection occurs once before distinct V1/V2 applications;
- active V2 release and resume retain readable named safety operations without a generic step pipeline or workflow engine;
- typed journal states validate transitions without executing a universal state machine;
- config/state pair persistence, release journals, dispatch journals, migration, and plugin-index have distinct ownership matching their contracts;
- direct Git, environment, token, network, and clock access is located in named adapters or documented compatibility fallbacks;
- no broad service/manager, boolean workflow selector, or generic dependency container was found in active composition.

### Intentional compatibility boundary

- fatal `Preflight` and `Service.Run` behavior, cwd-based V1 config/tool facades, registry-backed `HandleRelease`, and executor legacy method shapes;
- V1 validation continuing to require the legacy token/executor requirements;
- model-level zero-time fallbacks and system-default public constructors;
- no remote-state inference, no automatic uncertain-operation retry, and strict preservation of corrupt/conflicting journal evidence;
- V2 local execution remaining blocked pending an explicit product and safety design.

### Bounded technical debt

- P1 V1 rollback/network safety and config/state/migration crash windows;
- P2 mutable compatibility globals, process cwd, broad public compatibility surfaces, inactive paths, and absent journal lifecycle tooling;
- P3 arbitrary plugin-index output policy, empty-directory residue, repeated-release characterization gap, and superseded raw Git helpers.

### Architecture violation

- active V2 application and operation code directly formats terminal progress through the package-global logger. This is bounded to reporting, does not own `plugin.Response`, and does not change release decisions or order, but it conflicts with the strict presentation dependency direction and is assigned to DX1.

The violation does not change the historical ledger: all nine planned refactor stages were completed. It means “completed” is a closed milestone record, not a claim that no future architecture maintenance exists.
