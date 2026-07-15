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
