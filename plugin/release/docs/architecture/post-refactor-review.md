# Release Plugin Post-Refactor Architecture Review

## Review status and evidence

This document records the architecture verified after completion of the nine-stage Release Plugin refactor. It describes production code in commit `440eecc` and later review-only documentation commits; it is not an extension of the completed refactor plan.

The review followed the command composition in `plugin/release/main.go`, every production Go file under `plugin/release`, all architecture guard tests, the public package surfaces, and repository-wide references to retained compatibility symbols. The completed V1 compatibility extraction sequence was verified in Git history:

```text
fa55952 test(release): characterize v1 compatibility contracts
ab72785 refactor(release): isolate v1 compatibility use cases
55d5962 refactor(release): isolate v1 compatibility adapters
440eecc docs(release): complete release plugin refactor
```

The authoritative historical ledger remains in [refactor-history.md](refactor-history.md). Detailed behavioral invariants and disk contracts remain in [current-state.md](current-state.md). Ranked work is sequenced independently in [architecture-evolution.md](architecture-evolution.md). The completed V1 compatibility policy support decision for retained V1 compatibility surfaces is recorded in [v1-compatibility-policy.md](v1-compatibility-policy.md).

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

`main.main` remains the stdin/stdout protocol and composition root. It decodes `plugin.Request`, applies process-wide metadata and verbose mode, resolves one `workspace.RepositoryRoot`, constructs the three V1 executors, routes one command through `handleRequestAt`, converts unexpected Go errors to fatal protocol output, and encodes one `plugin.Response`. Production routing no longer changes process cwd.

The verified command boundaries are:

| Command area | Presentation boundary | Application boundary | Response owner |
| --- | --- | --- | --- |
| `patch`, `minor`, `major` | `releaseCommandHandler` and `ParseReleaseCommandRequest` | `releaseStartOperation`, then the selected V1 or V2 application | `MapReleaseCommandOutcome` and `MapCommandFailure` |
| `plan` | `releasePlanCommandHandler` and `ParsePlanCommandRequest` | `releasePlanInspectionUseCase` | `MapReleasePlanInspection` and `MapCommandFailure` |
| `resume` | `resumeCommandHandler` and `ParseResumeCommandRequest` | `resumeReleaseUseCase` | `MapResumeCommandOutcome` and `MapCommandFailure` |
| `init`, `unit-add` | `HandleInit` / `HandleUnitAdd` plus command-specific parsers | `initializeV2RepositoryUseCase` / `addV2ReleaseUnitUseCase` | `response_mapper.go` in `pkg/init` |
| `migrate` | `migrationCommandHandler` and `parseMigrationCommandRequest` | `migrationUseCase` and `migrationPlanExecution` | `pkg/migrate/response_mapper.go` |
| `validate` | `validateCommandHandler` and `parseValidationQueryRequest` | `validationQueryUseCase` | `pkg/validate/response_mapper.go` |
| `history` | `historyCommandHandler` and `parseHistoryQueryRequest` | `historyQueryUseCase` | `pkg/history/response_mapper.go` |
| `contributors` | `contributorsCommandHandler` and `parseContributorsQueryRequest` | `contributorsQueryUseCase` | `pkg/contributors/response_mapper.go` |
| `plugin-index` | `pluginIndexCommandHandler` and `parsePluginIndexCommandRequest` | `generatePluginIndexUseCase` | `pkg/pluginindex/response_mapper.go` |
| `init-options` | `GetAvailableOptions` | none; this is a static presentation query | `GetAvailableOptions` |

Raw `plugin.Request` and flag-map access are confined to the presentation handlers and parser files. The static `init-options` command directly constructs a response because it has no application or infrastructure work.

explicit-root composition added explicit-root composition across the command surfaces used by production routing: `HandleInitAt`, `HandleUnitAddAt`, `HandleReleaseAt`, `HandleReleaseWithV1ExecutorsAt`, `HandleResumeAt`, `HandleValidateAt`, `HandleHistoryAt`, `HandleContributorsAt`, `HandleEvidenceAt`, `HandleEvidenceArchiveAt`, and `HandlePluginIndexAt`. Their existing non-`At` handlers remain compatibility facades that resolve the root from `plugin.Request.Context.WorkingDir` or current cwd. `migrate.HandleMigrateAt` exists for embedders, while production keeps `migrate.HandleMigrate` to preserve migration's legacy Git-root discovery and nested-V1 refusal behavior.

## Active V2 release ownership

`releaseStartOperation` loads the normalized repository and delegates exactly one release source decision to `selectReleaseApplicationPath`. `v2ReleaseCommandApplication` resolves the selected unit and builds a V2-only `ReleaseExecutionContext`.

V2 dry-run is owned by `planV2Release`. It validates requirements, builds the version plan, plans and validates materialization, derives `KnownReleaseFiles`, and builds the immutable dispatch summary. It resolves no token and invokes no mutation adapter.

Read-only release plan inspection is owned by `releasePlanInspectionUseCase`. It selects the canonical source once, resolves one unit, reuses `PlanV1Release` for the V1 subset and `planV2ReleaseFacts` for V2 facts, and maps typed planning data only at the command boundary. It receives no token resolver, remote client, journal writer, evidence writer, Git mutator, state persister, materialization executor, dispatcher, release runner, or recovery capability.

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

`GitHubActionsReleaseRunner` is the public production facade. It validates the request, owns default production composition through explicit fields, and invokes the use case. It does not own the mutation order. Since typed release progress reporting, request facts and active V2 operation progress are emitted through the typed `ReleaseProgress` port supplied by command composition rather than through package-global terminal logging in application code.

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

Their use cases return typed facts and failures, do not construct responses, and receive no mutation capability. Source-format checks in these queries select format-specific read semantics; they are not release-workflow selection. Since explicit-root composition, repository loading and Git reads receive the same explicit root; the old `pkg/git` query helpers remain cwd facades over root-aware `At` helpers.

## Plugin-index ownership

`generatePluginIndexUseCase` exposes the explicit `query -> build -> persist` sequence. `pluginIndexQueryUseCase` loads and validates config, state, and plugin manifests and orders entries by plugin name. `jsonPluginIndexOutputBuilder` creates complete stable bytes. `atomicPluginIndexOutputPersister` alone performs the requested single-file effect.

Check mode stops after the query. Render mode stops after byte construction. Output mode passes complete bytes and the unchanged requested path to the persister. Command composition supplies the explicit repository root to the query use case. Public `Generate`, `Write`, and `WriteWithOptions` remain narrow compatibility surfaces over the canonical query and builder.

## Migration ownership

`migrationUseCase` resolves one repository root and one immutable plan. Pure policy classifies filesystem and journal evidence. `migrationPlanExecution` exposes a fixed order through focused journal, pair-persistence, target-verification, source-archive, and backup-verification capabilities.

Migration uses `config.V2ReleasePairPersister`, verifies exact target bytes and strict V2 validity before archiving V1, verifies the byte-identical backup after archive, and removes the migration journal last. Its worktree journal is intentionally separate from the Git-common-dir execution and dispatch journals.

## V1 compatibility ownership

Production `main` explicitly constructs `goreleaser.NewV1Executor`, `jreleaser.NewV1Executor`, and `releaseit.NewV1Executor` and passes them to `HandleReleaseWithV1Executors`. The fixed catalog selects one executor without mutable registry lookup.

The V1 application owns typed intent, preview/execution requests, pure planning, classified failures, and visible execution order. Focused capabilities own requirements, preflight, version evidence, config materialization, compensation evidence, named Git/GitHub compensation, process invocation, file inspection, environment/token access, and JReleaser time.

Active V1 compensation remains intentionally separate from V2 recovery. Its strict V1-only record at `<git-common-dir>/neko/release/v1-compensation/current.json` captures release identity, exact original config bytes and hashes, executor/Git evidence, fixed action fields, pending/confirmed outcomes, and timestamps. Named operations restore config, remove a GitHub Release and tags, revert and push or reset the release commit, and clean untracked release files. They persist intent before effects and verify before confirmation. Uncertain remote/executor evidence fails closed. The direct `V1ReleaseRollback` compatibility surface retains its characterized best-effort behavior but is not selected by the active application.

## Git ownership

Active V2 release and resume use one `GitReleaseCoordinator` instance per composed operation graph. Consumer-owned adapters expose only the preflight, stage, commit, tag, push, verification, or inspection methods needed by each operation. The former `GitReleaseCoordinator.Coordinate` convenience method was removed in retired-path cleanup.

Active V1 execution uses `SystemV1GitWriter`, root-aware named compensation adapters, the V1 compensation evidence store, and the V1 preflight repository adapter because its commit, push, and destructive-compensation contracts differ from V2. `V1ReleaseRollback` remains reachable only through direct compatibility calls. Read-only query adapters use `pkg/git` for their legacy and selected-path reads. Migration has a migration-owned Git-root resolver.

These owners are intentionally distinct; no active flow has two selectable Git implementations for the same intention.

## Journal ownership

`ReleaseExecutionJournalStore` exclusively owns execution-journal discovery, validation, pending-action writes, and confirmed-phase writes. `DispatchJournalStore` exclusively owns immutable dispatch-journal preparation, lookup, transitions, and terminal-state resolution.

`releaseJournalFiles` shares only identical mechanics: Git common-dir resolution, fixed directories, canonical JSON, private directory creation, and atomic `0600` replacement. It is not a generic journal repository.

Migration owns a separate compatible worktree journal because its source/target recovery evidence and lifecycle differ from release execution.

## Token and clock ownership

`EnvironmentGitHubActionsDispatchTokenResolver` is the production V2 `GITHUB_TOKEN` reader. It returns `GitHubActionsDispatchToken`; only dispatch adapters unwrap the value, and string formatting remains redacted.

V1 keeps its legacy token/environment behavior behind executor- and compensation-owned adapters. GoReleaser and release-it pass the legacy environment with redaction. JReleaser maps the legacy token to `JRELEASER_GITHUB_TOKEN` and uses an injected year clock. The bounded GitHub client receives a redacting typed token that is unwrapped only while constructing the request.

`ReleaseClock` supplies active release/resume response timestamps, V2 execution/dispatch persistence, and V1 compensation evidence timestamps. Query commands own their response clocks. Model-level zero-time fallbacks and public constructors with system defaults remain compatibility behavior for direct callers.

## Config, state, and materialization ownership

`pkg/config` owns V1/V2 disk models, strict loading and validation, normalized `ReleaseRepository`/`ReleaseUnit` read models, tag specifications, atomic single-file replacement, canonical V1 writing, and canonical V2 pair persistence.

For active V2 release, config is immutable architecture and state owns selected-unit versions. `VersionMaterializer` plans executor-specific file changes, `MaterializationTransaction` applies and restores declared changes before uncertainty, `StateTransaction` updates and restores V2 state, and `KnownReleaseFiles` defines the exact commit allowlist.

Plugin manifest materialization derives its path only from validated `ReleaseUnit` metadata. No active hard-coded plugin-unit registry remains.

## Remaining compatibility entry-point families

The following families remain outside canonical production composition or wrap it for compatibility. The completed V1 compatibility policy policy register records each qualified symbol, consumer, behavior, support decision, replacement, deprecation message, removal precondition, and intended removal gate.

- registry-backed release entry: `HandleRelease`, `Register`, `Get`, and the opt-in `pkg/release/tool` aggregator;
- legacy V1 command and tool surfaces: `Service`, `Preflight`, `Tool`, `ToolBase`, and executor `Init` / `Execute` / `Release` / `RevertRelease` methods;
- mixed-model builder: `BuildReleaseExecutionContext`; the old internal bridges `startLegacyRelease` and `newV1ReleaseCommandApplication` were removed by retired-path cleanup;
- inactive V2-local scaffolding: `ReleaseTransaction` and its preparation path; the former `GitReleaseCoordinator.Coordinate` convenience path was removed by retired-path cleanup;
- public query/migration wrappers: migration `ResolvePlan` / `Run` and plugin-index `Generate` / `Write` / `WriteWithOptions`;
- public constructors with system defaults for release/dispatch journals, dispatch, Git coordination, and the active runner.

## Progress and terminal reporting ownership

typed release progress reporting moved active V2 release progress behind a narrow synchronous `ReleaseProgress` capability. Application and focused operation code reports typed `ReleaseProgressEvent` values containing only safe display facts such as unit, version, tag, workflow, safe remote display URL, file paths, journal path, commit SHA, dispatch state, and typed dispatch inputs. The reporter has no error return and no access to Git, dispatch, journals, clocks, tokens, or response construction.

Terminal rendering is owned by `release_progress_terminal.go`, which wraps the established plugin stderr logger and preserves existing human text and verbose suppression through `log.Verbose`. A no-op reporter is used when no presentation reporter is supplied. Git command diagnostics remain separate from release progress through `gitReleaseDiagnostics`; the terminal diagnostics adapter is verbose-only and is composed explicitly with the production GitHub Actions runner.

Progress output remains separate from final command responses. JSON response bodies are still built only by command response mappers and encoded to stdout by `main`; progress uses stderr and is captured as execution logs by the existing plugin protocol. Unknown progress events render nothing and cannot change release policy or recovery classification.

## Architecture audit result

The verified architecture contains no active god function, replacement god object, generic workflow pipeline, generic state-machine engine, boolean V1/V2 workflow selector, broad dependency container, application-owned `plugin.Response`, raw flag access outside parsers, scattered active release source selection, or duplicated active release/journal infrastructure ownership.

typed release progress reporting resolved the prior active progress-presentation deviation. `releaseStartOperation`, `planV2Release`, `GitHubActionsReleaseRunner`, `githubActionsReleaseUseCase`, focused GitHub Actions release operations, Git preflight/coordination, and dispatch operations no longer import the package-global terminal logger directly. Response construction remains command-owned, and release decisions, journal writes, unsafe effect ordering, retry refusal, and recovery classification are unchanged.

The refactor ledger therefore remains closed at 9 / 9. Future safety, compatibility, developer-experience, and feature work belongs in the separate [architecture evolution record](architecture-evolution.md), not in another numbered refactor stage.

## Prioritized debt register

Priority means:

- **P0** — release safety or data-loss risk requiring immediate correction;
- **P1** — significant correctness or maintainability risk;
- **P2** — bounded architectural or compatibility debt;
- **P3** — optional cleanup.

No P0 issue was found. The active V2 GitHub Actions path preserves evidence around uncertain operations, blocks ambiguous retry, and has a complete pending-action boundary around its unsafe mutations. V1 compensation interruption safety resolved the active V1 P1 items below. V2 pair and migration crash recovery resolved the remaining pair and migration P1 crash-recovery risks by adding explicit durable pair evidence, deterministic next-process recovery, and migration refusal for owner-ambiguous evidence.

### D-01 — Destructive V1 compensation was best-effort and non-journaled

- **Status:** **Resolved by V1 compensation interruption safety** for the active V1 application. Direct compatibility calls to `V1ReleaseRollback` retain their characterized best-effort semantics.
- **Affected files or symbols:** `v1_compensation_evidence.go`; `v1_compensation_store.go`; `v1_compensation_policy.go`; `v1_compensation_operations.go`; `v1_compensation_adapters.go`; `v1_release_use_case.go`; executor `CompensationState` methods.
- **Current behavior:** The active application creates private, strict, hashed evidence before mutation and records pending intent before every fixed compensation effect. Exact config restoration and repeatable local Git operations can continue on the next invocation; remote, non-repeatable, corrupt, and uncertain states require manual recovery. No generic transition API or executable action list was introduced.
- **Remaining boundary:** An interruption inside the executor is not remotely inferred. release-it failures and GoReleaser/JReleaser push/publication ambiguity remain manual by design. evidence inspection and archival owns future inspection or archival tooling.
- **Completed capability:** **V1 compensation interruption safety — Make V1 compensation interruption-safe**.

### D-02 — Active V1 GitHub Release deletion used an unbounded global HTTP client

- **Status:** **Resolved by V1 compensation interruption safety** for the active V1 application.
- **Affected files or symbols:** `v1_github_release_client.go`; `systemV1GitHubReleaseRemover`; `v1GitHubToken`.
- **Current behavior:** The injected client receives an explicit repository root, derives a canonical GitHub target without changing cwd, uses a finite 15-second timeout, caps response reads at 64 KiB, never includes response bodies in diagnostics, and verifies deletion with a final not-found lookup. The typed token is unwrapped only at request construction and never reaches evidence, errors, logs, or responses.
- **Remaining boundary:** The old raw `pkg/git` helper remains an exported compatibility candidate for retired-path cleanup, but active V1 compensation cannot reach it.
- **Completed capability:** **V1 compensation interruption safety — Make V1 compensation interruption-safe**.

### D-03 — V2 config/state pair persistence is crash-recoverable but not cross-file atomic

- **Status:** **Resolved by V2 pair and migration crash recovery** for init, unit-add, and migration pair writes.
- **Affected files or symbols:** `pkg/config/v2_pair_persister.go`; `pkg/config/v2_pair_recovery.go`; `V2ReleasePairPersister.Persist`; init, unit-add, and migration pair writers.
- **Current behavior:** `V2ReleasePairPersister` creates durable schema-versioned evidence at `.neko/release.pair-recovery.json` before unsafe replacement. It records pending intent before each target rename, verifies exact bytes before confirmation, verifies the complete intended pair before completion, and closes evidence only after the pair is strict-valid. A later process closes already-complete intended pairs, restores exact prior bytes/modes/existence for supported partial application, or fails closed with evidence-preserving manual guidance.
- **Why it remains non-atomic:** Portable filesystems do not offer one atomic rename for two independent files. V2 pair and migration crash recovery deliberately implemented recovery evidence instead of claiming impossible cross-file atomicity.
- **Remaining boundary:** Corrupt, unsupported, owner-ambiguous, externally edited, or hash/mode-conflicting evidence requires manual recovery. A failed new-pair attempt may still leave an empty `.neko` directory.
- **User or developer impact:** Supported process or machine interruptions are deterministic on the next pair-writing command. Unsupported evidence no longer silently repairs or guesses.
- **Completed capability:** **V2 pair and migration crash recovery — Make pair and migration crash recovery explicit**.
- **Blocking new features:** No longer blocks ordinary multi-file config/state callers that use the shared pair persister and preserve its evidence contract.

### D-04 — Migration crash recovery is evidence-driven

- **Status:** **Resolved by V2 pair and migration crash recovery** for the supported V1-to-V2 migration crash windows.
- **Affected files or symbols:** `pkg/migrate/execution.go`; `pkg/migrate/policy.go`; `migrationPlanExecution.Execute`; `filesystemMigrationJournalOperations`; `filesystemMigrationSourceArchiver`; `release.migration.json`; `.neko/release.pair-recovery.json`.
- **Current behavior:** Migration journals intent, persists the V2 pair through the shared crash-recoverable persister, verifies exact target bytes and strict V2 validity, archives V1 only after target proof, verifies the byte-identical backup, and removes the migration journal last. If migration and pair-recovery evidence coexist, the next run classifies the interrupted pair through the persister before continuing source recovery. Pair-recovery evidence without a migration journal is refused as owner-ambiguous.
- **Why it remains conservative:** Migration still refuses arbitrary corruption, missing trustworthy source/backup evidence, unsupported journal versions, and externally edited files. It does not infer that an unproven filesystem effect completed.
- **Remaining boundary:** Manual recovery is required when the active V1 source and backup cannot be matched to migration evidence, when pair evidence conflicts, or when an operator deletes the migration journal but leaves pair evidence behind.
- **User or developer impact:** Supported interruption windows are recoverable or safely completable; unsupported states now produce explicit refusal instead of implicit best-effort behavior.
- **Completed capability:** **V2 pair and migration crash recovery — Make pair and migration crash recovery explicit**.
- **Blocking new features:** Multi-unit or nested migration must preserve this owner relationship before broadening migration format or schema behavior.

### D-05 — Compatibility tool registry is mutable and process-global

- **Affected files or symbols:** `pkg/release/registry.go`; package variable `tools`; `Register`; `Get`; `registeredV1ReleaseExecutorCatalog`; `directV1ReleaseExecutorCatalog`; `pkg/release/tool/register.go`.
- **Current behavior:** Importing `pkg/release/tool` runs three side-effect registrations. Re-registration silently overwrites by name. The map has no synchronization. Production `main` uses a fixed catalog and never imports the aggregator.
- **Why it remains:** External Go callers may still rely on the exported registry and blank-import registration behavior.
- **Risk:** Direct concurrent callers can race, tests can leak registration state, and import side effects obscure compatibility composition.
- **User or developer impact:** This primarily affects embedders and tests; the plugin executable's release path is unaffected.
- **Removal or improvement preconditions:** Audit downstream imports, publish a deprecation window, offer explicit executor composition to embedders, and retain characterized lookup/overwrite behavior until removal.
- **Recommended action:** **Replace** with explicit composition for all known consumers, then remove after compatibility policy permits.
- **Priority:** **P2**.
- **Proposed capability:** **V1 compatibility policy — Decide and deprecate V1 compatibility surfaces** completed the support/deprecation decision. retired-path cleanup retained the registry because `HandleRelease` and `Service` still intentionally preserve registry-backed compatibility behavior.
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
- **Proposed capability:** **V1 compatibility policy — Decide and deprecate V1 compatibility surfaces** completed the support/deprecation decision. retired-path cleanup retained the version-evidence seams because the public `VersionGuard` compatibility behavior and two-pass evidence semantics remain supported.
- **Blocking new features:** Not blocking unrelated features; blocks parallel V1 execution support.

### D-07 — Production command roots are explicit; cwd compatibility facades remain

- **Status:** **Resolved by explicit-root composition** for production command routing and canonical embedder entry points.
- **Affected files or symbols:** `pkg/workspace/root.go`; `main.handleRequestAt`; command `Handle*At` entry points; root-aware query and Git adapters; retained `ChangeToProjectRoot`; retained `ToolBase.InUnitRoot`; retained cwd-based V1 config facades.
- **Current behavior:** `main` resolves one `workspace.RepositoryRoot` and routes through explicit-root handlers without changing process cwd. Init, unit-add, release, resume, validation, history, contributors, evidence, and plugin-index command composition now receive a resolved root. Migration exposes `HandleMigrateAt`, while production keeps the legacy Git-root discovery facade to preserve nested-V1 behavior.
- **Remaining boundary:** Compatibility helpers still intentionally expose cwd semantics: `ChangeToProjectRoot`, cwd V1 config facades, selected `ToolBase` behavior, `Service` cwd fallback, and legacy `pkg/git` query facades. They are compatibility surfaces, not production composition dependencies.
- **Risk:** Passing an incorrect explicit root can still target the wrong repository; retained compatibility facades are still unsafe for parallel in-process use if callers choose them.
- **User or developer impact:** Embedders have canonical explicit-root command APIs, and tests prove two in-process repositories can be queried independently without cwd mutation. Existing CLI root discovery and relative response contracts remain intact.
- **Recommended action:** **Keep** the explicit-root APIs as the canonical embedder path. Do not remove cwd compatibility facades without a later compatibility-cleanup decision and downstream evidence.
- **Priority:** **P2** for retained compatibility facades only.
- **Completed capability:** **explicit-root composition — Make command roots explicit for embedders**.
- **Blocking new features:** No longer blocks ordinary in-process embedding that uses the explicit-root APIs. It still blocks claims of full concurrent safety for legacy compatibility facades.

### D-08 — Inactive V2-local transaction remains; coordinator convenience was retired by retired-path cleanup

- **Affected files or symbols:** `pkg/release/release_transaction.go`; `ReleaseTransaction`; `prepareReleaseFilesForCoordinator`; `MutationTracker`. retired-path cleanup removed `GitReleaseCoordinator.Coordinate`.
- **Current behavior:** `ReleaseTransaction.Execute` always blocks non-dry-run V2 execution. Its private preparation and rollback-before-uncertainty code is exercised only by tests. Active V2 release/resume use named operation adapters and focused coordinator methods; there is no longer a one-call coordinator convenience method.
- **Why it remains:** `ReleaseTransaction` is exported/inactive V2-local scaffold, and V2 local delivery evaluation has not decided whether local delivery is a product goal. retired-path cleanup did have enough evidence to remove `GitReleaseCoordinator.Coordinate` because production did not call it and tests moved to focused coordinator methods.
- **Risk:** Readers can still mistake `ReleaseTransaction` preparation for active orchestration, and future work may accidentally extend the wrong scaffold.
- **User or developer impact:** No current command behavior is affected; maintenance and onboarding cost remain.
- **Removal or improvement preconditions:** Search downstream consumers, decide whether V2 local execution is a real product goal, preserve any public compatibility contract, and prove all active production call sites keep using named operations.
- **Recommended action:** **Defer** `ReleaseTransaction` until V2 local delivery evaluation. Keep retired-path cleanup's guard that prevents reintroducing `GitReleaseCoordinator.Coordinate`.
- **Priority:** **P2**.
- **Proposed capability:** **V2 local delivery evaluation — Evaluate V2 local delivery** for `ReleaseTransaction`; retired-path cleanup completed the coordinator convenience removal.
- **Blocking new features:** Blocks implementation of V2 local execution on the current scaffold. It does not block GitHub Actions features.

### D-09 — V2 local execution remains blocked

- **Affected files or symbols:** `startV2Release`; `V2_LOCAL_DELIVERY_BLOCKED`; `ReleaseTransaction.Execute`; executor capability and delivery models.
- **Current behavior:** V2 local non-dry-run returns an explicit block. GitHub Actions is the only active V2 publication owner.
- **Why it remains:** Executor ownership, inclusion of V2 state in the release commit, unsafe-operation journaling, and recovery are unresolved. release-it in particular owns commit/tag/push internally.
- **Risk:** Activating the current scaffold would bypass the proven V2 journal/recovery order or fail to guarantee committed state.
- **User or developer impact:** Users cannot execute V2 local releases; they must use GitHub Actions or remain on V1 compatibility.
- **Removal or improvement preconditions:** A separate product decision and design must prove ownership, state-in-commit, exact Git order, journaling, interruption recovery, and executor-specific feasibility.
- **Recommended action:** **Defer**. Keep the block until an explicit feature implementation is approved.
- **Priority:** **P2** as a product limitation, not an architecture regression.
- **Proposed capability:** **V2 local delivery evaluation — Evaluate V2 local delivery**.
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
- **Proposed capability:** none; preserve as a architecture invariant unless a dedicated recovery feature is authorized.
- **Blocking new features:** Does not block read-only recovery guidance; blocks blind retry features by design.

### D-11 — Journals have no repair tool or schema-migration mechanism

- **Status:** Partially resolved by evidence inspection and archival. There is still no repair or schema-migration command, by design, but operators now have evidence-safe inspection and a guarded completed-evidence archival path.
- **Affected files or symbols:** `ReleaseExecutionJournalStore`; `DispatchJournalStore`; migration journal loading; fixed schema versions and strict validation.
- **Current behavior:** Invalid, conflicting, or unsupported journals are rejected and preserved. `neko release evidence` reports redacted typed summaries and diagnostics across release execution, dispatch, migration, V1 compensation, and V2 pair-recovery evidence. `neko release evidence-archive` can only archive completed release-execution, completed V1 compensation, or completed V2 pair-recovery evidence after family, identity, digest, and explicit confirmation are rechecked.
- **Why it remains:** Guessing or rewriting recovery evidence would weaken safety. Existing schemas have not required migration yet.
- **Risk:** A corrupted journal can still block automatic continuation indefinitely; a future schema change still requires a separate migration design.
- **User or developer impact:** Manual recovery remains conservative, but operators no longer need to inspect raw JSON just to know which evidence exists, who owns it, whether it is completed, and whether archival is allowed.
- **Removal or improvement preconditions:** Define immutable backups, an auditable repair plan, explicit operator confirmation, schema compatibility rules, and tests that never infer unsafe completion.
- **Recommended action:** **Keep** the evidence inspection and archival inspection/archive boundary. **Defer** repair or schema migration until a concrete schema change or support burden exists.
- **Priority:** **P2**.
- **Proposed capability:** none for inspection/archive; future schema migration would require a new dedicated design.
- **Blocking new features:** Blocks journal schema changes and automated repair features; not other product work.

### D-12 — Public compatibility wrappers remain broad

- **Affected files or symbols:** the complete compatibility inventory below, including `HandleRelease`, `Service`, `Preflight`, `Tool`, `ToolBase`, executor legacy methods, mixed context construction, query/migration facades, and system-default constructors. retired-path cleanup removed `V2ExecutionUnavailableResponse`, `GitReleaseCoordinator.Coordinate`, private test bridges, and exact raw Git helpers whose gates were satisfied.
- **Current behavior:** Some retained wrappers directly delegate; others preserve fatal exit, cwd, registry, default-construction, or legacy method-shape behavior. Production composition bypasses most V1 wrappers.
- **Why it remains:** The Go packages are public and no downstream compatibility-removal decision was authorized.
- **Risk:** Unknown consumers constrain signature cleanup and can make inactive code appear canonical.
- **User or developer impact:** Existing embedders retain source compatibility; maintainers carry extra surfaces and tests.
- **Removal or improvement preconditions:** Downstream reference audit, deprecation policy, replacement examples, release notes, and an explicit support/removal decision.
- **Recommended action:** Follow the V1 compatibility policy policy register and retired-path cleanup completion record: keep, deprecate, defer, or remove only by family with fresh evidence and a documented replacement.
- **Priority:** **P2**.
- **Proposed capability:** **V1 compatibility policy — Decide and deprecate V1 compatibility surfaces** and **retired-path cleanup — Retire superseded and inactive release paths** are completed; later compatibility removals need a new gate.
- **Blocking new features:** Not generally blocking. New code must not depend on these wrappers.

### D-13 — Active V2 progress logging crosses the presentation boundary

- **Status:** **Resolved by typed release progress reporting**.
- **Affected files or symbols:** `releaseStartOperation.Start`; `planV2Release`; `logV2DryRunPlan`; `GitHubActionsReleaseRunner.Run`; `githubActionsReleaseUseCase.Run`; `github_actions_release_operations.go`; package-global `log.Verbose`.
- **Current behavior:** Active V2 application and focused operations emit typed release progress events through `ReleaseProgress`. Terminal rendering and verbose suppression are isolated in `release_progress_terminal.go`; Git verbose diagnostics are isolated in `git_release_diagnostics_terminal.go`. Responses remain command-owned and typed, and reporting calls do not select workflow behavior.
- **Why it was resolved:** typed release progress reporting characterized existing output and secret behavior, introduced the typed reporter, moved active call sites to events, and added architecture guards preventing direct terminal logger imports in active V2 application/operation files.
- **Risk:** Reintroducing ad-hoc logging could again blur presentation and release-safety changes.
- **User or developer impact:** Release correctness and output text remain stable while tests can record typed progress without global stderr capture.
- **Removal or improvement preconditions:** Completed by typed release progress reporting.
- **Recommended action:** **Keep** progress reporting behind `ReleaseProgress`; do not introduce a generic event bus or telemetry pipeline.
- **Priority:** Historical **P2**, resolved.
- **Completed capability:** **typed release progress reporting — Isolate release progress reporting**.
- **Resolution evidence:** `release_progress_characterization_test.go`, `release_progress_terminal_test.go`, `release_progress_test.go`, and `command_architecture_test.go` cover output ordering, stderr/stdout separation, no-op reporting, verbose suppression, secret safety, no generic event infrastructure, infallible reporter behavior, and active-file logger import guards.
- **Blocking new features:** No longer blocking. Future active V2 orchestration edits must keep terminal rendering out of application and operation files.

### D-14 — Plugin-index output path policy is explicit

- **Affected files or symbols:** `pkg/pluginindex/output_persister.go`; `atomicPluginIndexOutputPersister.Persist`; `--output` contract.
- **Current behavior:** Resolved by generated-output path policy. Relative `--output` values are clean repository-root-relative paths resolved from the explicit root. Explicit absolute output remains supported for CI/temp artifacts. Repository-contained targets cannot overwrite release config/state/evidence, Git internals, or plugin manifest inputs from the generated index. Existing target directories and target symlinks are rejected, and repository-relative parent symlinks must resolve inside the repository.
- **Why it remains:** It no longer remains as undefined debt. Flexible absolute CI output is retained as a documented exception, while repository-relative output is confined.
- **Risk:** Remaining risk is bounded to explicitly supplied absolute output targets, which are intentionally supported for workflow temp artifacts.
- **User or developer impact:** Nested CLI invocation and explicit-root embedders now resolve relative output identically. Existing workflow `/tmp` and runner-temp outputs continue to work.
- **Removal or improvement preconditions:** Further restriction of absolute output would require fresh workflow and downstream compatibility evidence.
- **Recommended action:** **Keep** the focused output owner and guards; do not replace it with a generic path manager.
- **Priority:** Historical **P3**, resolved.
- **Completed capability:** **generated-output path policy — Clarify generated-output path policy**.
- **Blocking new features:** No longer blocks features that rely on repository-relative plugin-index output.

### D-15 — Failed new-pair creation can leave an empty `.neko` directory

- **Status:** Retained after V2 pair and migration crash recovery as a bounded cosmetic limitation.
- **Affected files or symbols:** `V2ReleasePairPersister.Persist`; `osV2PairPersistenceDisk.CreateDirectory`.
- **Current behavior:** The directory is created before snapshots and temporary files. A later returned failure cleans temporary files and restores targets but does not remove a newly created empty directory.
- **Why it remains:** V2 pair and migration crash recovery made pair file recovery explicit and intentionally avoided directory deletion that could remove user-created content. The directory is harmless, and tracking/restoring parent-directory existence would add another compatibility effect to pair persistence.
- **Risk:** Cosmetic filesystem residue can confuse scripts that treat directory presence as configuration presence without checking files.
- **User or developer impact:** No valid config/state is fabricated; manual cleanup may be desired.
- **Removal or improvement preconditions:** Characterize directory-mode/existence behavior and ensure cleanup never removes a directory containing unrelated files.
- **Recommended action:** **Defer** unless a real consumer needs stronger empty-directory cleanup semantics.
- **Priority:** **P3**.
- **Proposed capability:** none.
- **Blocking new features:** Not blocking.

### D-16 — Successive V2 release planning after completed-journal exclusion is less directly characterized

- **Status:** Resolved by evidence inspection and archival characterization.
- **Affected files or symbols:** `ReleaseExecutionJournalStore.FindUnresolved`; selected-unit state update; `releaseStartOperation`; planner tests.
- **Current behavior:** `handoff-ready` journals are excluded, so a later release plans from committed V2 state. evidence inspection and archival adds an end-to-end temporary-repository characterization that completes one GitHub Actions handoff, reloads V2 state, runs the next release, and proves the first execution/dispatch journals are not rewritten.
- **Why it remains:** Existing unit and runner tests prove the component facts; the refactor did not add a separate repeated-release scenario.
- **Risk:** A future journal-filter or state-loading change could reopen a completed execution or calculate from stale state.
- **User or developer impact:** Potential regression would affect repeated releases, not the first release.
- **Removal or improvement preconditions:** Keep the evidence inspection and archival characterization before changing journal exclusion or release planning.
- **Recommended action:** **Keep**.
- **Priority:** **P3**.
- **Proposed capability:** none.
- **Blocking new features:** Not blocking while the characterization remains in place.

### D-17 — Superseded raw Git helpers were removed by retired-path cleanup

- **Affected files or symbols:** `pkg/git/repository.go`; `LastCommit`, `TotalCommits`, `FilesCount`, `RepoSize`, `Head`, `CleanUntracked`, `DeleteLocalTag`, `DeleteRemoteTag`, `RevertCommit`, `CreateCommit`, `HardResetTo`, and `DeleteGithubRelease`.
- **Current behavior:** These exported cwd-based helpers are no longer compiled. retired-path cleanup kept active `git.Current`, query/preflight helpers, and unit/history/contributor readers while removing only the raw unused helpers.
- **Why it remains:** Resolved by retired-path cleanup.
- **Risk:** The former destructive helpers can no longer be mistaken for the canonical V1 rollback adapter inside the repository.
- **User or developer impact:** Active command behavior is unchanged. External consumers of those exported helper symbols must use caller-owned Git code or release-owned focused adapters.
- **Removal or improvement preconditions:** Completed in retired-path cleanup: repository and generated-entry reference audit, tests migrated, active helpers retained, and architecture guards added.
- **Recommended action:** **Keep** the retired-path cleanup guard and do not reintroduce cwd-based destructive helpers.
- **Priority:** **P3**.
- **Proposed capability:** **Completed in retired-path cleanup — Retire superseded and inactive release paths**.
- **Blocking new features:** Not blocking; new code must not call them.

## Compatibility wrapper inventory

The inventory distinguishes the plugin executable's active composition from public Go compatibility. “No active consumer” means no production call site was found in this repository; exported symbols still carry unknown downstream-consumer risk.

| Symbol or family | Package | Active internal consumers | Compatibility purpose | Behavior or delegation | Removal risk | Recommendation |
| --- | --- | --- | --- | --- | --- | --- |
| `release.HandleRelease` | `pkg/release` | none; command and dry-run tests use it | Preserve the original patch/minor/major entry | Composes the registry-backed V1 catalog, then delegates to the canonical handler/start operation | High: exported command API and V1 behavior | Keep for V1 compatibility policy; production remains on `HandleReleaseWithV1Executors` |
| `release.Service`, `NewReleaseService`, `NewReleaseServiceWithContext`, `Service.Run`, `Service.GetNewVersion` | `pkg/release` | none; compatibility tests only | Preserve the former V1 service API | Delegates to isolated planning/application, but retains catalog selection, cwd fallback, logging, and fatal-exit mapping | High | Deprecated by V1 compatibility policy; removal requires retired-path cleanup/downstream audit |
| `release.Preflight` | `pkg/release` | none; compatibility tests only | Preserve fatal V1 preflight entry | Delegates checks to focused preflight then invokes legacy fatal output on failure | High because exit behavior is observable | Deferred by V1 compatibility policy because no exact public replacement exists |
| `release.Tool` | `pkg/release` | compatibility catalogs and registry only | Preserve the legacy executor interface | Interface only; its method shape drives compatibility adapters | High | Deferred by V1 compatibility policy; `V1Executor` replaces release execution but not legacy init/tool methods |
| `ToolBase.ValidateRequirements`, `ResolveFiles`, `InUnitRoot`, `RequireBinary`, `RevertGitRelease`, `DeleteGitHubRelease`, `CreateReleaseCommit`, `CreateGitTag`, `PushCommits`, `PushGitTag` | `pkg/release` | executor compatibility methods and tests; not active fixed-catalog `Run` flow | Preserve shared legacy tool methods | Direct delegates except cwd-changing `InUnitRoot`; constructs system adapters with legacy empty-root semantics | High for external tool implementations | Deferred by V1 compatibility policy; no exact public replacement for every helper |
| `release.Register`, `release.Get`, package variable `tools`, and `pkg/release/tool.init` | `pkg/release`, `pkg/release/tool` | registry-backed compatibility entry points only | Preserve registration and blank-import discovery | Contains mutable overwrite/lookup behavior and import side effects | High | Registry functions and package init deprecated by V1 compatibility policy; `tools` is a retired-path cleanup removal candidate |
| `GoReleaser.Init`, `Execute`, `Release`, `RevertRelease`, `ValidateRequirements`, `ResolveFiles` | `pkg/release/tool/goreleaser` | compatibility tests and potential external callers; active production uses `Run`/`Rollback` through `V1Executor` | Preserve concrete legacy tool API | `Execute` delegates to `Run`; `Release` delegates to shared release logic; `RevertRelease` delegates to canonical `Rollback` after V1 compatibility policy | High | `Execute`, `Release`, and `RevertRelease` deprecated by V1 compatibility policy; `Init` deferred |
| `JReleaser.Init`, `Execute`, `Release`, `RevertRelease`, `ValidateRequirements`, `ResolveFiles` | `pkg/release/tool/jreleaser` | same pattern as GoReleaser | Preserve concrete legacy tool API | Direct compatibility delegates around canonical executor logic; `RevertRelease` delegates to canonical `Rollback` after V1 compatibility policy | High | Same V1 compatibility policy result |
| `ReleaseIt.Init`, `Execute`, `Release`, `RevertRelease`, `ValidateRequirements`, `ResolveFiles` | `pkg/release/tool/releaseit` | same pattern as GoReleaser | Preserve concrete legacy tool API | Direct compatibility delegates around canonical executor logic; `RevertRelease` delegates to canonical `Rollback` after V1 compatibility policy | High | Same V1 compatibility policy result |
| `release.BuildReleaseExecutionContext` | `pkg/release` | tests only; production uses `BuildV2ReleaseExecutionContext` | Preserve a mixed V1/V2 normalized-repository builder | Contains source-specific unit-root selection, then delegates to common assembly | Medium to high | Deprecated by V1 compatibility policy; retain until external callers migrate |
| `release.startLegacyRelease`, `newV1ReleaseCommandApplication` | `pkg/release` | none; removed in retired-path cleanup | Former internal test/transition entry points from the old path | Tests now use canonical V1 application composition/public boundaries | Low internally | Removed by retired-path cleanup after tests moved to canonical entry points |
| `config.V1Exists`, `V1LoadConfig`, `V1SaveConfig` | `pkg/config` | compatibility tests; canonical code uses explicit-root/path variants | Preserve cwd-based V1 config API | Direct delegates to `V1ConfigExistsAt`, `V1LoadConfigAt`, and `V1SaveConfigAt` | High because exported and deprecated | Deprecated by V1 compatibility policy with explicit-root/path replacements |
| `release.VersionGuard`, `VersionGuardWithOptions`, `EnsureVersionIsValid` | `pkg/release` | compatibility tests; active V1 application uses its planning evidence adapter | Preserve legacy version-guard API and warnings | `VersionGuard` and `VersionGuardWithOptions` retain mutable evidence globals; `EnsureVersionIsValid` is pure | Medium to high | `VersionGuard` and `VersionGuardWithOptions` deprecated by V1 compatibility policy; pure `EnsureVersionIsValid` kept |
| `release.V2ExecutionUnavailableResponse` | `pkg/release` | none; removed in retired-path cleanup | Former exported response helper from command extraction | Use `MapCommandFailure` with a command-specific `CommandFailure` | Medium unknown-consumer risk before removal | Removed by retired-path cleanup after repository audit found no consumer |
| `release.ReleaseTransaction`, `NewReleaseTransaction`, `ReleaseTransaction.Execute` | `pkg/release` | tests only | Preserve inactive V2-local scaffold/public shape | Constructor builds preparation state; `Execute` always blocks | High because exported despite inactive behavior | Decide product direction in V2 local delivery evaluation first; deprecate/remove later only if local delivery is rejected |
| `GitReleaseCoordinator.Coordinate` | `pkg/release` | none; removed in retired-path cleanup | Former one-call Git sequence | Active release/resume use focused coordinator methods through named operations | Medium before removal | Removed by retired-path cleanup to prevent a competing orchestration path |
| `migrate.ResolvePlan`, `migrate.Run`, exported `migrate.Plan` | `pkg/migrate` | tests only; command uses `migrationUseCase` | Preserve programmatic migration preview/execution | Narrow facades over canonical root/plan/use-case paths | Medium | Keep unless a public API policy removes them; they do not violate direction |
| `pluginindex.Generate`, `Write`, `WriteWithOptions` | `pkg/pluginindex` | tests and possible programmatic callers; command uses injected query/builder | Preserve programmatic index generation/serialization | Direct delegates to canonical query and builder | Low to medium | Keep; these are cohesive public APIs, not urgent debt |
| `NewReleaseExecutionJournalStore`, `NewDispatchJournalStore`, `NewGitHubActionsDispatcher`, `NewGitHubActionsReleaseRunner`, `NewGitReleaseCoordinator` and their options | `pkg/release` | active production composition plus tests/direct callers | Provide system-default production adapters and substitution options | Construct explicit system defaults; active use cases receive their resulting capabilities | Low | Keep; review only if a constructor hides new infrastructure in application code |

## Dead, inactive, and redundant production code inventory

No code is labeled dead solely from an IDE result. Classification uses production call sites, tests, package initialization, repository-wide imports, and exported-symbol risk.

| Code | Reachability evidence | Classification | Recommendation |
| --- | --- | --- | --- |
| `release.startLegacyRelease`, `newV1ReleaseCommandApplication` | no remaining definitions or call sites | **Removed in retired-path cleanup** | Tests moved to canonical boundaries; retired-path cleanup guard prevents reintroduction |
| `registeredV1ReleaseExecutorCatalog`, `registeredV1ReleaseExecutor`, `directV1ReleaseExecutorCatalog`, `directV1ReleaseExecutor` | reached only from `HandleRelease` / `Service`, not production `main` | **Internal compatibility** | Keep while those public facades remain; remove with them |
| `pkg/release/tool` package initializer | reached only when deliberately imported; production does not import it | **Public compatibility** | Keep as explicit opt-in until registry deprecation completes |
| `ReleaseTransaction.Execute` | production-callable but always blocked; tests cover the refusal | **Future feature scaffold** and **Unknown consumer risk** | Do not extend; decide V2 local delivery evaluation versus retired-path cleanup |
| `ReleaseTransaction.prepareReleaseFilesForCoordinator`, callback hooks, `ensureGitClean`, and `unstageKnownFiles` | private and referenced only by transaction tests | **Test support**, **Future feature scaffold**, and **Safe removal candidate** | Remove in retired-path cleanup if V2 local delivery evaluation rejects the scaffold; otherwise redesign before activation |
| `GitReleaseCoordinator.Coordinate` | no remaining definition or call site | **Removed in retired-path cleanup** | Active release/resume use focused coordinator methods through named operations; retired-path cleanup guard prevents reintroduction |
| `V2ExecutionUnavailableResponse` | no remaining definition or call site | **Removed in retired-path cleanup** | Use `MapCommandFailure` with a command-specific `CommandFailure`; retired-path cleanup guard prevents reintroduction |
| `buildV2InitConfigFromFlags` | no remaining definition or call site | **Removed in retired-path cleanup** | Tests cover typed parser/constructor behavior directly; retired-path cleanup guard prevents reintroduction |
| `pkg/git` raw metrics helpers `LastCommit`, `TotalCommits`, `FilesCount`, `RepoSize` | no remaining definitions or call sites | **Removed in retired-path cleanup** | Use command-owned query readers or caller-owned Git code; retired-path cleanup guard prevents reintroduction |
| `pkg/git` raw destructive helpers `Head`, `CleanUntracked`, `DeleteLocalTag`, `DeleteRemoteTag`, `RevertCommit`, `CreateCommit`, `HardResetTo`, `DeleteGithubRelease` | no remaining definitions or call sites; active rollback uses focused root-aware adapters | **Removed in retired-path cleanup** | Use root-aware release adapters or caller-owned Git code; retired-path cleanup guard prevents reintroduction |
| `BuildReleaseExecutionContext` mixed builder | tests and potential external callers only; production selected V2 path uses the V2-only builder | **Public compatibility** | Keep through V1 compatibility policy; remove only after caller migration |
| V2 executor capability and delivery facts describing local execution | read by planning and the blocked transaction but do not activate execution | **Future feature scaffold** | Keep only while V2 local delivery evaluation remains a plausible product decision |

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

- no unresolved P1 issue remains in the active release safety model after V1 compensation interruption safety and V2 pair and migration crash recovery; manual recovery boundaries are explicit compatibility and safety policy;
- P2 mutable compatibility globals, retained cwd compatibility facades, broad public compatibility surfaces, inactive paths, and future schema repair/migration policy;
- P3 arbitrary plugin-index output policy and empty-directory residue.

### Architecture violation

typed release progress reporting removed the last active presentation-boundary deviation known at this review level. Remaining debt is tracked by the ranked items above and by the next capability records rather than by reopening the completed refactor ledger.

This does not change the historical ledger: all nine planned refactor stages were completed. It means “completed” is a closed architecture record, not a claim that no future architecture maintenance exists.

The architecture decision backlog, acceptance criteria, and commit-boundary guidance are defined in [architecture-evolution.md](architecture-evolution.md). V1 compensation interruption safety, V2 pair and migration crash recovery, evidence inspection and archival, V1 compatibility policy, retired-path cleanup, typed release progress reporting, explicit-root composition, generated-output path policy, and release plan inspection are completed. The recommended next architecture decision is **V2 local delivery evaluation**.
