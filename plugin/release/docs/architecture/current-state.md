# Release Plugin Current Architecture

## Purpose and audit basis

This document describes the Release Plugin as it exists in the current checkout. It is not a target package design and does not assume that an earlier refactor exists.

The verified final dependency view, compatibility inventory, and debt classification are maintained in [post-refactor-review.md](post-refactor-review.md). Future safety, compatibility, developer-experience, and feature milestones are maintained in [post-refactor-roadmap.md](post-refactor-roadmap.md). This document remains the detailed behavioral and data-contract reference.

The C1 support decision and C2 removal record for retained V1 compatibility surfaces are recorded in [v1-compatibility-policy.md](v1-compatibility-policy.md). That register is the authoritative source for Keep, Deprecate, Defer, Removed, and future-removal decisions.

The audit follows the current command routes in `plugin/release/main.go`, every production package under `plugin/release/pkg`, the tests under `plugin/release`, the plugin manifest, the repository V2 release files, and the release workflows. Existing repository-wide release documentation was used only as supporting context where the source and tests confirmed it.

The active production scope is `plugin/release`. Shared contracts inspected for integration context include `pkg/plugin/types.go`, `pkg/errors/plugin_errors.go`, and `pkg/config/env.go`.

## Runtime topology

The plugin is a stdin/stdout JSON executable:

1. `main.main` decodes one `plugin.Request` from stdin.
2. It sets global plugin metadata and verbose logging.
3. `workspace.ChangeToProjectRoot` changes the process working directory.
4. A command switch invokes one handler.
5. The handler normally returns a `plugin.Response`; an unexpected Go error is converted by `main` to fatal `EXECUTION_ERROR` output.
6. `main` JSON-encodes the response to stdout.

The public command contract is duplicated between `manifest.json` and the switch in `main.go`. `manifest_test.go` characterizes their agreement and also checks the repository command documentation.

## Package and responsibility map

| Area | Current responsibility | Important symbols | Notes |
| --- | --- | --- | --- |
| `main` | Plugin protocol entry, workspace change, command routing, fatal error fallback | `main` | Uses a command switch rather than a command registry. |
| `pkg/workspace` | Select V2 Git root or legacy nearest-V1 root, then change process cwd | `ResolveProjectRoot`, `ChangeToProjectRoot` | Process-global `os.Chdir` is an implicit dependency of every handler. |
| `pkg/config` | V1/V2 disk models, strict loading, validation, normalization, unit and tag selection, atomic file writes, and canonical crash-recoverable V2 pair persistence | `ReleaseRepository`, `ReleaseUnit`, `LoadReleaseRepository`, `ValidateV2`, `ResolveReleaseUnit`, `TagSpec`, `AtomicWriteFile`, `V2ReleasePairPersister` | `ReleaseRepository` is the shared normalized model. Init and migration reuse one V2 config/state writer and one pair-recovery evidence protocol. V1 remains a compatibility source. |
| `pkg/init` | Typed init/unit-add command boundaries, focused initialization use cases, pure unit/pair construction, and explicit file policy | `HandleInit`, `HandleUnitAdd`, `initializeV2RepositoryUseCase`, `addV2ReleaseUnitUseCase` | Handlers parse, invoke one use case, and map a typed result/failure; validated pairs are passed to the shared config persister. |
| `pkg/migrate` | Typed command presentation, source discovery, pure target planning/recovery policy, ordered failure-aware execution, journaling, and root V1-to-V2 migration | `HandleMigrate`, `migrationUseCase`, `migrationPlan`, `migrationPlanExecution`, `ResolvePlan`, `Run` | Uses focused filesystem operations and a worktree migration journal distinct from release journals. |
| `pkg/validate` | Typed validation request/result boundary, focused V1/V2 validation query, and response mapping | `HandleValidate`, `validationQueryUseCase`, `mapValidationQueryResponse` | V1 validation retains its requirements adapter and `GITHUB_TOKEN` dependency; V2 config validation is token-independent and read-only. |
| `pkg/history` | Typed history query, format-specific read-only Git capabilities, and response mapping | `HandleHistory`, `historyQueryUseCase`, `historyGitReader` | V1 deliberately retains non-erroring tag/count queries; V2 uses exact `TagSpec` matches and structured Git failures. |
| `pkg/contributors` | Typed contributor query, repository/unit selection, focused shortlog capabilities, and response mapping | `HandleContributors`, `contributorsQueryUseCase`, `contributorsGitReader` | V1 repository-wide and V2 path-filtered reads share one command-owned read port without mutation capabilities. |
| `pkg/pluginindex` | Typed command modes, deterministic discovery/validation/order, pure JSON output building, and atomic requested-path persistence | `HandlePluginIndex`, `pluginIndexQueryUseCase`, `jsonPluginIndexOutputBuilder`, `atomicPluginIndexOutputPersister` | Check/render/persist retain their established outputs; all command failures remain Go errors that become top-level `EXECUTION_ERROR`. |
| `pkg/git` | Legacy Git queries and the underlying compatibility operations used by focused V1 adapters | `IsClean`, `LatestTag`, `UnitTagsInHistory`, `Current`, `Contributors`, `ContributorsForPaths` | Direct process details remain below release-owned V1 ports; active V1 application code does not import retired raw C2 helpers. |
| `pkg/release` planning | Version bump, execution context, delivery/capability descriptions, materialization plan | `PlanUnitVersionBump`, `BuildReleaseExecutionContext`, `ResolveDelivery`, `ResolveExecutorCapabilities`, `ResolveVersionMaterializer` | Useful typed models exist, but some capability data describes inactive V2 local behavior. |
| `pkg/release` V1 | Typed V1 intent/planning/preview/execution/failures, focused requirements/preflight/materialization/Git/rollback adapters, and explicit executor selection | `V1ReleaseIntent`, `PlanV1Release`, `v1ReleasePreviewUseCase`, `v1ReleaseExecutionUseCase`, `V1Executor` | Production uses a fixed executor catalog. `Service`, `Preflight`, `Tool`, `ToolBase`, and `Register/Get` are bounded compatibility facades. |
| `pkg/release` V2 GitHub Actions | Typed command boundary, active release use case, named journaled operations, and production facade | `releaseCommandHandler`, `releaseStartOperation`, `githubActionsReleaseUseCase.Run`, `GitHubActionsReleaseRunner.Run` | The facade composes one coordinator, one typed token boundary, and one clock; the use case owns the visible safety order and delegates each mutation to a focused operation. |
| `pkg/release` V2 Git | Preflight, targeted staging, exact commit verification, tag creation, ordered pushes, dispatch verification, and recovery tag inspection | `GitReleaseCoordinator`, `githubActionsReleaseGitAdapter`, `gitReleaseDispatchVerifier`, `resumeGitAdapter` | Active release/resume share one coordinator instance through consumer-owned capabilities; the former one-call `Coordinate` convenience path was removed in C2. |
| `pkg/release` state/files | Plan and apply version files; update and restore V2 state | `MaterializationTransaction`, `StateTransaction`, `KnownReleaseFiles` | Snapshots support bounded local restore before commit uncertainty. |
| `pkg/release` execution journal | Durable intended-release identity, monotonic phases, pending actions, and execution-specific persistence | `ReleaseExecutionJournal`, `ReleaseExecutionJournalStore` | Store-specific validation/mutations use the shared fixed journal location and secure-write mechanics below the Git common directory. |
| `pkg/release` dispatch | Immutable workflow request, dispatch-specific persistence/classification, typed token, GitHub target, and HTTP client | `ReleaseDispatchRequest`, `DispatchJournalStore`, `GitHubActionsDispatchToken`, `GitHubActionsDispatcher`, `GitHubActionsDispatchClient` | Explicit accepted/rejected/unknown outcomes; the token stays typed and redacted through dispatch adapters. |
| `pkg/release` recovery | Typed command boundary, read-only assessment, pure continuation policy, and reuse of active named operations | `resumeCommandHandler`, `resumeReleaseUseCase`, `AssessReleaseExecutionRecovery`, `resolveResumeRecovery` | Recovery receives focused Git evidence and reuses active tag/dispatch/push/handoff capabilities without a second orchestration path. |
| `pkg/release/tool/*` | GoReleaser, JReleaser, and release-it V1 executor orchestration plus executor-owned system adapters | `NewV1Executor`, `Run`, `CompensationState`, compatibility `Rollback` | Each executor consumes narrow Git/process/file/environment/token/clock capabilities, exposes typed effect evidence to the active application, and retains its characterized direct rollback delegate for compatibility callers. |

## Command-to-flow map

### `init`

- Entry: `main.main` -> `init.HandleInit`.
- Request parsing: `parseInitCommandRequest` is the only init path that reads the untyped flag map and produces `initCommandRequest`; wrong raw types retain the prior zero-value/default behavior.
- Application boundary: `initializeV2RepositoryUseCase.Execute` applies the pure V1/V2/force policy, constructs one normal or plugin unit, creates a complete config/state pair, validates it, and passes it to the pair writer.
- Domain ownership: `unit_constructor.go` owns defaults and normal/plugin construction; `policy.go` owns side-effect-free file-presence decisions; `repository.go` owns complete pair creation and repository validation.
- Side effects: `config.V2ReleasePairPersister` snapshots both targets, persists `.neko/release.pair-recovery.json`, creates and writes both temporary files, then records pending/confirmed evidence around config and state replacement. Returned replace failures trigger restoration of both snapshots; next-process recovery uses only the durable evidence and observed files.
- State mutations: replaces both V2 files when permitted; `--force` never overwrites V1.
- Output: `response_mapper.go` constructs the text-oriented success response or stable structured failure from a typed result/failure.
- Error behavior: stable codes include `CONFIG_CONFLICT`, `V1_CONFIG_EXISTS`, `CONFIG_EXISTS`, `INVALID_FLAGS`, `VALIDATION_ERROR`, and `SAVE_ERROR`.
- Existing tests: handler characterization plus focused parser, constructor, policy, mutation, use-case, response-boundary, temp-create/write/replace, rollback, restoration-failure, exact byte/mode, and cleanup tests.

### `unit-add`

- Entry: `main.main` -> `init.HandleUnitAdd`.
- Request parsing: `parseUnitAddCommandRequest` produces a distinct typed request and records unsupported `force` presence without retaining the raw flag map.
- Application boundary: `addV2ReleaseUnitUseCase.Execute` applies the pure V1/V2 presence policy, preserves required-unit/force precedence, constructs the unit, loads the pair once, rejects duplicate state identity, produces an appended copy, validates it, and invokes the same pair writer.
- State mutation: `appendV2ReleaseUnit` clones existing slices, plugin metadata, and the state map before appending one config unit and one state entry; existing units are not overwritten or mutated.
- Output: the shared mapper preserves the success command `unit-add`, table renderer, data keys, and the characterized compatibility value `init` on error metadata.
- Existing tests: normal/plugin append, partial/missing configuration, duplicate unit/state/plugin name, invalid inputs, load-once/order, no input mutation, pair rollback, and exact restoration are covered.

### `init-options`

- Entry: `main.main` -> `init.GetAvailableOptions`.
- Behavior: returns a hard-coded table matching V2 flags.
- Side effects: response timestamp only.
- Tests: `TestGetAvailableOptionsExposesV2OnlyInitOptions` and manifest contract tests.
- Risk: option metadata is duplicated between Go and `manifest.json`.

### `migrate`

- Entry: `main.main` -> `migrate.HandleMigrate`. The handler parses a typed request, invokes `migrationUseCase.Migrate` once, and maps the typed outcome/failure with an injected response clock. `Run` remains a compatibility facade over the same path.
- Discovery and planning: the root resolver and filesystem plan resolver classify new migration, recovery, already-complete, and conflict states. `constructMigrationPlan` builds and validates the complete target pair without filesystem access; pure policy functions select one typed planning intention and the target/source operations required by disk evidence. `ResolvePlan` remains a compatibility facade.
- Execution order: start or read the journal; persist the target pair when needed through `config.V2ReleasePairPersister`; confirm target persistence; verify exact target bytes plus strict V2 loading/validation; archive the V1 source when needed; confirm the archive; verify the byte-identical backup; remove the journal.
- Recovery: the immutable plan selects `persistMigrationTarget` or `retainMigrationTarget` and `archiveMigrationSource` or `retainArchivedMigrationSource`. Recovery skips effects already proven by the journal and filesystem evidence instead of replaying a generic transition machine. If pair-recovery evidence exists with a migration journal, the target persistence step lets the shared pair persister restore or close it before rewriting the intended pair; pair-recovery evidence without a migration journal is refused as ambiguous.
- State: `migrationJournalStage` validates the compatible serialized values `prepared`, `config-written`, `state-written`, and `v1-archived`. Empty and unknown values are rejected at load time. The schema version, field names, paths, hashes, strings, and journal mode `0644` are unchanged.
- Dry-run: returns the exact ordered response rows and planned config/state JSON without creating `.neko`, writing a journal or targets, or archiving V1.
- Failure behavior: planning, journal, target persistence, target verification, source cleanup, source verification, and restoration are typed internal failure classes while the public `MIGRATION_FAILED`/nil-Go-error contract remains stable. Incomplete pair restoration or an invalid only remaining backup explicitly requires manual recovery.
- Tests: characterization preserves the public envelope, metadata, data keys, row order, JSON, flag defaults, recovery actions, source bytes/mode, and unrelated files. Focused unit/integration tests inject every execution boundary, prove stop order and recoverable disk evidence, validate typed journal transitions, and enforce the planner/policy/execution boundaries.

### `patch`, `minor`, and `major`

- Entry: `main.main` constructs the three V1 executors and calls `release.HandleReleaseWithV1Executors` -> `releaseCommandHandler` with a typed `release.Type`. `HandleRelease` remains the registry-backed compatibility entry.
- Parsing: `ParseReleaseCommandRequest` is the only release-start code that reads the untyped plugin flag map and produces `ReleaseCommandRequest`. Missing or wrongly typed flags preserve the existing zero-value defaults.
- Application boundary: the handler invokes `releaseCommandStarter.Start` exactly once. `releaseStartOperation` loads the canonical repository once and `releaseApplicationPathSelector` selects exactly one V1 or V2 application from `ReleaseRepository.SourceFormat`; the selected application does not reselect.
- Branch: V1 receives a typed intent and distinct preview or execution use case. V2 alone builds `ReleaseExecutionContext`. Active V1 does not call `Service`, fatal `Preflight`, the mixed execution-context builder, or the mutable registry.
- Response: application code returns a sealed `ReleaseCommandOutcome` or typed `CommandFailure`; `MapReleaseCommandOutcome` and `MapCommandFailure` construct the stable response from an explicit handler-supplied timestamp.
- Tests: source selection, pure V1 planning, preview immutability, execution order/stopping, response/fatal compatibility, executor commands/ownership, materialization, token/environment/clock boundaries, rollback, V2 Git coordination, and active GitHub Actions behavior are distributed through `pkg/release/*_test.go` and `pkg/release/tool/*`.

#### V2 dry-run

- Orchestration: `releaseStartOperation` -> `startV2Release` -> `planV2Release`, retaining `BuildReleaseExecutionContext` -> requirements validation -> materialization/known-file/dispatch planning order.
- Decisions: calculate next SemVer and tag; resolve delivery and executor capabilities; plan materialization; calculate known release files; build a dispatch summary.
- Side effects: reads config/state, executor config, manifests, and file hashes; emits logs and a timestamped response. It does not resolve a token or write journals/files/refs/remotes.
- Tests: all `TestHandleRelease*DryRun*` cases in `dry_run_test.go`, plus materializer and coordinator dry-run tests.
- Missing characterization: a single dependency-spy test proving that every network, clock-independent mutation, Git mutation, and journal store remains unused across each executor/delivery combination.

#### V2 GitHub Actions execution

- Facade: `GitHubActionsReleaseRunner.Run` validates the execution request, logs request facts, composes production operations, and invokes `githubActionsReleaseUseCase.Run`.
- Orchestration: `githubActionsReleaseUseCase.Run` exposes the ordered story: token resolution; materialization planning; Git/unresolved-journal preflight; execution-journal preparation; materialization; state write; targeted stage; commit; tag; dispatch-journal preparation; commit push; tag push; workflow dispatch; accepted-handoff confirmation.
- State: each named mutation operation persists its exact pending marker before the side effect and its confirmed phase afterward. Execution and dispatch stores retain separate contracts while sharing common-dir/canonical-write mechanics; all active adapters receive the facade's coordinator and clock.
- Output: `GitHubActionsReleaseResult` is a typed command outcome mapped only in `command_response.go`.
- Existing tests: `github_actions_release_runner_test.go` preserves real-repository happy paths and durable recovery evidence. `github_actions_release_use_case_test.go` proves the full named order, stopping at every replaceable dependency, cleanup order, rejected-dispatch behavior, and captured-log token absence. `github_actions_release_operations_test.go` injects pending-write, side-effect, and confirmation failures around all eight journaled mutations.
- Git verification: `BuildReleaseDispatchRequest` receives `releaseDispatchGitVerifier`; production injects `gitReleaseDispatchVerifier` backed by the facade's existing coordinator. The builder no longer constructs Git infrastructure.

#### V2 local execution

- `releaseStartOperation` returns `V2_LOCAL_DELIVERY_BLOCKED` before executing a local transaction.
- `ReleaseTransaction.Execute` independently returns `v2GitCoordinationUnavailableMessage` for every V2 non-dry-run call.
- `prepareReleaseFilesForCoordinator` and executor capabilities are internal preparation code exercised only by tests; they are not the active public local release path.
- This parallel inactive path is a compatibility constraint during refactoring: it must not be mistaken for production orchestration.

#### V1 execution

- Planning: `pureV1ReleasePlanner` receives a typed `V1ReleasePlanningRequest` and returns current/latest/next versions, tag, commit metadata, executor, canonical V1 config file, and materialized-file ownership without infrastructure access or mutation. Preview uses local tag evidence only and returns a typed result without token, file, Git, executor, or rollback effects.
- Execution: `v1ReleaseExecutionUseCase` first opens the repository's V1 compensation store and continues or refuses any unresolved attempt before planning a new release. It then performs local preview planning, requirements, typed preflight, refreshed execution planning, fixed executor resolution, durable evidence creation, verified V1 config materialization, one executor invocation, and typed completion. The readable order is explicit; no step list, workflow pipeline, state machine, or boolean V1/V2 mode is involved.
- Preconditions: the characterized executor file and `GITHUB_TOKEN`; clean worktree; attached `main` or `master`; configured upstream; branch not reported behind. Fatal preflight is represented as `V1ReleaseFailure` and mapped at the command boundary to the established fatal JSON/exit behavior.
- State mutation and ownership: `.release.neko.json` is written before execution through `V1SaveConfigAt`. GoReleaser owns the release commit, lightweight `v` tag, commit/tag pushes, warning-only snapshot, and publication. JReleaser first synchronizes `jreleaser.yml`, then owns the commit/push and warning-only dry-run while JReleaser owns tag/publication. release-it owns its commit/tag/push/publication internally and package-manager selection still prefers `bun.lock`, then `package-lock.json`, then npm.
- Recovery: the active application stores V1-only evidence at `<git-common-dir>/neko/release/v1-compensation/current.json` before config mutation and executor invocation. Recovery selects exactly one named operation at a time in the fixed order: restore the exact original config bytes; delete the GitHub Release; delete the local tag; delete the remote tag; revert and push a pushed release commit, or reset an unpushed release commit; then clean untracked release files. Every operation persists `pending` before its side effect and confirms only after verification. The first failure stops later effects. A later V1 release invocation automatically continues supported repeatable local operations, but refuses pending or uncertain remote operations, a pending/non-repeatable revert, and an uncertain executor outcome with evidence-path guidance. This remains compensation, not transactional rollback, and unsupported evidence requires manual recovery.
- Executor evidence: GoReleaser failures are automatically compensable only when its captured state proves local effects; push/publication ambiguity is manual. JReleaser is automatically compensable only before commit/push/publication ambiguity. A release-it process failure is always externally uncertain because release-it owns commit, tag, push, and publication internally.
- Tests: fake subprocess, Git, token, environment, clock, file, config, GitHub-client, evidence-store, and reporter capabilities protect the full order/failure and interruption matrix. System compatibility tests use only local fake executables, temporary repositories, and local HTTP transports.

### `resume`

- Entry: `main.main` -> `release.HandleResume` -> `resumeCommandHandler` -> `resumeReleaseUseCase`.
- Parsing and response: `ParseResumeCommandRequest` creates the typed request; the handler invokes `releaseResumer.Resume` once and maps a sealed `ResumeCommandOutcome` or `CommandFailure` with its injected response clock.
- Discovery: `locateResumableExecution` requires V2, resolves one unit and its current upstream remote, and finds exactly one unresolved execution journal matching remote URL and unit.
- Assessment: `AssessReleaseExecutionRecovery` verifies journal structure, known-file hashes, and local tag evidence through an injected tag inspector without remote access. Non-dry continuation separately reconstructs the journal-bound execution context and rejects current-config drift.
- Dry-run: returns that assessment without requiring `GITHUB_TOKEN` or modifying the journal/worktree.
- Policy and selection: `resolveResumeRecovery` is a pure resolver from journal plus local assessment to one typed operation or refusal. `resumeReleaseOperationSelector` maps the supported classification once to `resumeFromCommitCreatedOperation`, `resumeFromTagCreatedOperation`, `resumeFromTagPushedOperation`, or `returnCompletedReleaseHandoffOperation`; the selected operation does not switch on the execution phase again.
- Continuation: commit-created preserves the characterized already-present-tag block and otherwise reuses Stage 3 tag creation before delegating to tag-created. Tag-created reuses Stage 3 dispatch preparation, commit push, and tag push in the active order. Tag-pushed uses a separate pure dispatch decision and reuses Stage 3 dispatch and handoff confirmation.
- Dispatch and token boundary: prepared or missing dispatch state permits one fresh dispatch; accepted dispatch is reused without redispatch or token resolution; request-started, rejected, and unknown remain no-retry refusals. Token lookup occurs only inside the fresh-dispatch operation after earlier continuation effects have succeeded and returns the same typed token used by active release.
- Completed handoff: an explicit dependency-free operation returns the existing handoff result, while production discovery continues to exclude handoff-ready journals as resolved.
- Restrictions: it will not calculate a new version, continue before a confirmed commit, prove ambiguous push completion, or redispatch a terminal dispatch journal.
- Existing tests: dry-run read-only behavior, ordered assessment output, no/exactly-one journal selection, corrupt/conflicted/config-drift handling, every execution-state/pending-action policy combination, operation selection, supported continuation from `commit-created`, `tag-created`, and `tag-pushed`, expected-tag-already-present blocking, completed-handoff behavior, ambiguous-push blocking, no push-state inference, accepted dispatch reuse without a token, terminal dispatch no-retry behavior, and focused continuation failure boundaries.
- Retained limitations: there is no remote-state probe, automatic retry, journal repair, or continuation from pre-commit/ambiguous push evidence. Production discovery deliberately excludes completed journals.

### `history`

- Entry: `history.HandleHistory` parses `historyQueryRequest`, invokes `historyQueryUseCase.Query` once, and maps the typed result/failure with a response clock.
- Read ownership: `historyRepositoryReader` loads the canonical repository; the command-owned `historyGitReader` exposes only legacy tags/counts and V2 unit-tags/path-counts. No mutating Git capability enters the use case.
- V1: uses all local tags and direct commit counts. The legacy adapter intentionally retains the established empty-success/zero-count behavior when its package functions suppress Git errors.
- V2: exact unit-prefixed tags reachable from `HEAD`, ordered by history, with counts constrained to unit pathspecs. Unit-tag and count errors map to `GIT_HISTORY_FAILED` with a nil handler Go error.
- Tests: parser, one-invocation handler, fixed mapper, focused stop-point, same-commit ordering, empty result, real-Git path filtering, and worktree/index/ref immutability tests supplement `git/tag_test.go`.

### `contributors`

- Entry: `contributors.HandleContributors` parses `contributorsQueryRequest`, invokes `contributorsQueryUseCase.Query` once, and maps its typed result/failure.
- Read ownership: a command-owned repository reader and contributor-only Git port expose repository-wide or selected-path shortlog reads; the use case preserves the adapter's deterministic order and clones returned entries.
- V1: repository-wide `git shortlog`; V2: selected-unit pathspecs. Git failures remain structured `GIT_CONTRIBUTORS_FAILED` responses with nil handler Go errors.
- Tests: typed defaults, handler invocation/mapping, V1/V2 capability selection, dependency stopping, empty results, selected paths, response contracts, and worktree/index/HEAD immutability are explicit.

### `validate`

- Entry: `validate.HandleValidate` parses `validationQueryRequest`, invokes `validationQueryUseCase.Query` once, and maps typed validation facts/failures outside application code.
- Read ownership: `validationRepositoryReader` returns canonical repository data plus the presence fact needed to preserve `CONFIG_NOT_FOUND` versus `CONFIG_INVALID`; `legacyRequirementsValidator` is the only V1 environment/filesystem requirements capability.
- V2: strict load/validation already occurred in `LoadReleaseRepository`; optional `--show` returns cloned normalized unit facts for pure response formatting. It remains token-independent and non-mutating.
- V1: resolves the legacy unit, revalidates the model, then invokes the established token/executor requirements adapter in that order. The token requirement remains characterized compatibility behavior, not a recommendation.
- Tests: typed defaults, fixed response mapping, load/presence classification, V1 validation/requirements stop order, V2 dependency isolation and no-alias behavior, stable rows/errors, and exact config/state immutability supplement config/workflow tests.

### `plugin-index`

- Entry: `pluginindex.HandlePluginIndex` parses one typed render/check/persist mode, invokes `generatePluginIndexUseCase.Run` once, and maps a typed result. `--check` with `--output`, discovery, building, and persistence failures intentionally remain Go errors and therefore top-level `EXECUTION_ERROR` values.
- Discovery and validation: `pluginIndexQueryUseCase` owns config/state/manifest reads through `pluginIndexSourceReader`; pure candidate/completion functions validate state SemVer and manifest identity, duplicate checks retain their prior order, and entries are stably sorted by plugin name. Public `Generate` is the compatibility facade over this query.
- Output building: `jsonPluginIndexOutputBuilder` alone creates complete pretty or compact JSON bytes with the stable schema/order and trailing newline. It chooses no path, reads no files, and constructs no response. `Write` and `WriteWithOptions` remain public compatibility wrappers over those complete bytes.
- Persistence: output mode passes complete bytes and the unchanged requested path to `atomicPluginIndexOutputPersister`. It creates requested parent directories as `0755`, overwrites the arbitrary requested target, uses `0644` for a new file, preserves an existing target's mode, writes/fsyncs/closes a target-local temporary file, then renames it and discards any unconsumed temporary file. A returned pre-replace/write/replace failure preserves the prior target; no config, state, manifest, Git, journal, or unrelated file is mutated.
- Modes: check performs discovery only; default render performs discovery then building and returns raw JSON; output performs discovery, building, and the explicit single-file command effect. There is still no output-path confinement policy, publication action, cancellation source beyond the supplied context, or schema change.
- Tests: query/read stop points, typed entry validation, deterministic output, parser/handler/use-case boundaries, all three modes, builder/writer failure, creation/replacement/modes, injected create/write/replace failures, original preservation, temporary cleanup, unrelated-file preservation, response compatibility, and workflow scripts are explicit.

## V1 compatibility subsystem

### Boundary and source selection

V1 and V2 share only canonical repository loading and the normalized `ReleaseRepository`/`ReleaseUnit` read model. `releaseApplicationPathSelector` is the single active release source-format decision. V1 selection creates `V1ReleaseIntent`; V2 selection creates the V2-only execution context. Neither selected application switches on source format again.

The pre-refactor responsibilities were spread across `releaseStartOperation`, `Service`, fatal `Preflight`, `VersionGuard`, the mutable tool registry, `ToolBase`, and concrete executors. The active V1 path now consists of:

```text
typed command request
  -> canonical source selection
  -> typed V1 intent
  -> pure preview/execution planning
  -> focused requirements and preflight
  -> focused V1 config materialization
  -> one fixed V1 executor
  -> typed result or classified failure
  -> command-owned response/fatal mapping
```

`V1ReleaseIntent`, `V1ReleasePreviewRequest`, `V1ReleaseExecutionRequest`, `V1ReleasePlan`, `V1ExecutorRequest`, `V1ReleaseResult`, and `V1ReleaseFailure` are the application contracts. They contain release facts, not response rows, raw flags, callbacks, dependency maps, open files, mutable execution phases, or secrets.

### Model, validation, planning, preview, and execution ownership

- `pkg/config` remains the sole owner of `V1ReleaseConfig`, strict V1 loading/validation, normalization to the virtual `default` unit, canonical bytes, and `.release.neko.json` writing. `V1SaveConfigAt` adds explicit repository-root ownership while `V1SaveConfig` remains a direct current-directory facade.
- V1 release requirements own the legacy token-plus-executor-file contract. V1 preflight owns the exact clean/attached/main-or-master/upstream/not-behind checks and fatal codes. These semantics are not reused by V2 because V2 preconditions and failure policy differ.
- `pureV1ReleasePlanner` owns patch/minor/major version and metadata calculation. Preview and execution are separate use cases after shared pure planning; preview cannot reach mutation dependencies.
- `v1ReleaseExecutionUseCase` owns only the visible application order and invokes the pure, executor-specific recovery decision. It delegates evidence persistence, file, repository, executor, named compensation, reporting, and version-evidence effects to focused capabilities.
- Application code returns typed V1 results/failures. Existing release command mappers own `plugin.Response`, timestamps, renderer hints, metadata, ordering, nil-Go-error behavior, and fatal compatibility.

### File, Git, executor, token, environment, and clock ownership

- `v1ReleaseConfigFileMaterializer` owns planned-version bytes through the canonical V1 store. `v1CompensationConfigFileAdapter` restores and verifies the exact original bytes captured before mutation. Neither uses the V2 config/state pair persister because the disk and recovery contracts differ.
- `SystemV1GitWriter` owns the exact shared legacy commit/tag/push commands. The active application owns fixed named compensation operations through root-aware, verifying Git adapters and V1-only durable evidence; it does not call the direct compatibility `V1ReleaseRollback`. They remain separate from `GitReleaseCoordinator`: V1 uses `commit -a`, allow-empty release commits, fixed `v` tags, immediate pushes, and destructive compensation with a distinct evidence contract.
- Production `main` constructs GoReleaser, JReleaser, and release-it explicitly and passes an immutable fixed catalog to the V1 application. The catalog invokes exactly one configured executor. Concrete executor packages no longer self-register or inspect source format.
- Each executor has consumer-owned process/file/config/environment/token/clock ports matching its actual behavior. GoReleaser and release-it receive the legacy environment and redact `GITHUB_TOKEN` from process output/errors. JReleaser resolves the legacy token explicitly, appends only `JRELEASER_GITHUB_TOKEN`, and uses an injected clock for inception-year generation. Shared redaction preserves underlying error causes.
- V1 response timestamps use the existing command clock. The compensation store uses the injected clock for evidence `createdAt` and `updatedAt`; these are recovery facts, not response values. The only executor wall-clock read is behind JReleaser's system clock adapter for generated init configuration.

### Shared versus isolated capabilities

Shared capabilities have identical contracts:

- canonical V1 model/load/validation/write operations;
- normalized repository/unit read models;
- pure SemVer interpretation and release command response mapping where the schemas already match;
- V1 binary lookup, file existence, process-result redaction, release Git writes, executor evidence vocabulary, and fixed compensation order shared by all V1 executors.

Isolated capabilities differ materially:

- V1 requirements/preflight, Git mutation, compensation evidence, token handling, executor commands, and single-file materialization remain separate from V2 journals, targeted staging, typed dispatch token, recovery, and evidence-preserving failure policy;
- GoReleaser, JReleaser, and release-it keep separate subprocess/config/environment/clock ports because their command, file, token, publication, and ownership contracts differ;
- migration may consume only canonical V1 read models/loaders/validation. Architecture guards prohibit migration from importing the V1 release use case, executor request, Git mutation, or rollback internals.

### Compatibility facades and bounded limitations

The retained public compatibility surfaces are direct delegates:

- `HandleRelease` composes the registry-backed catalog for callers that deliberately retain the old entry point; production uses `HandleReleaseWithV1Executors`.
- importing `pkg/release/tool` explicitly opts into `Register/Get`; the three concrete packages no longer mutate the registry from `init`.
- `Service`, `Preflight`, `Tool`, `ToolBase`, executor `Execute`, `Release`, and `RevertRelease`, and the mixed context builder remain for direct callers/tests and delegate to the isolated V1 behavior.
- zero-value executor construction is retained only for those legacy facades; active executors arrive fully composed through `NewV1Executor` and do not construct hidden dependencies during execution.

C1 completed the support decision for those surfaces. Deprecated surfaces now point to tested replacements where one exists: explicit `HandleReleaseWithV1Executors` composition, `PlanV1Release`, `BuildV2ReleaseExecutionContext`, explicit V1 config root/path functions, `Run` with `V1ExecutorRequest`, `Rollback`, and `MapCommandFailure`. Deferred surfaces such as fatal `Preflight`, `Tool`, `ToolBase`, and legacy executor `Init` remain unmarked because no exact public replacement exists. Concrete executor `Rollback` methods now own the direct rollback adapter call, while `RevertRelease` is the deprecated direct delegate.

Bounded limitations remain:

- the compatibility registry and version-evidence package variables remain mutable for old callers/tests but are unreachable from production release composition and deprecated where a tested explicit replacement exists;
- direct callers of the legacy `V1ReleaseRollback` compatibility surface retain its characterized in-memory, best-effort behavior; the active V1 application does not use it;
- pending/uncertain remote deletion or push, a pending revert, release-it failure, and executor push/publication ambiguity intentionally require manual recovery rather than remote inference or blind retry;
- the active release invocation is recorded as one pending executor effect, so interruption inside an executor is conservatively classified as manual instead of inferred from remote state;
- the single `current.json` record is retained after completion and may be replaced by a later attempt; inspection, archival, and schema-lifecycle tooling remain deferred to H3;
- `ReleaseTransaction` retains inactive V2-local preparation tests because F2 has not decided whether local V2 delivery is a product goal. Production blocks V2 local execution, no concrete V1 executor implements or branches into that path, and the former JReleaser V2-local bypass was removed rather than activated;
- process-global workspace selection and compatibility current-directory facades remain outside this stage.

## Important data models

### Repository and unit

`config.ReleaseRepository` is the normalized read model shared by V1 and V2. V1 becomes one virtual `default` unit. `config.ReleaseUnit` combines immutable config facts with the current version from state. This avoids format branching in history, contributors, and most planning.

The source-of-truth disk models are:

- V1: `config.V1ReleaseConfig` in `.release.neko.json`.
- V2 architecture: `config.V2ReleaseConfig` in `.neko/release.config.json`.
- V2 mutable versions: `config.V2ReleaseState` in `.neko/release.state.json`.
- V2 pair recovery: `schemaVersion: 1` evidence in `.neko/release.pair-recovery.json`, owned only by `config.V2ReleasePairPersister`.

### Planning and materialization

- `VersionPlan`: next version and tag with no writes.
- `ReleaseExecutionContext`: normalized input for planning/execution.
- `ExecutorCapabilities` and `DeliveryContract`: current ownership/support descriptions.
- `MaterializationPlan` and `MaterializedFileChange`: planned file bytes, hashes, mode, reason, and commit requirement.
- `KnownReleaseFiles`: exact repository-relative set permitted in the V2 release commit, including planned state pre/post hashes.
- `StateSnapshot` and `MaterializationSnapshot`: exact local restore inputs before an unsafe boundary.

`plugin_manifest_materializer.go` selects plugin behavior from validated `ReleaseUnit.IsPlugin` and materializes the normalized `ReleaseUnit.PluginManifestPath`. Existing `plugin-release` and `plugin-ui` paths remain canonical configuration data, and any validated plugin unit follows the same path without a production registry.

### Canonical active V2 adapter ownership

- Release-owned file planning is driven by `ReleaseExecutionContext.Unit` plus `KnownReleaseFiles`; plugin manifest location comes only from validated unit metadata.
- `GitHubActionsReleaseRunner` and resume composition each create one `GitReleaseCoordinator`. Consumer-owned Git adapters expose only preflight/stage/commit/tag/push, dispatch verification, remote lookup, or tag inspection needed at each call site.
- `ReleaseExecutionJournalStore` exclusively owns execution-journal lookup, validation, and intention-revealing mutations. `DispatchJournalStore` exclusively owns dispatch-journal preparation, validation, transition persistence, and terminal-state resolution.
- `releaseJournalFiles` is not a generic store: it owns only the two fixed journal directories, common-dir resolution, canonical JSON bytes, `0700` directory creation, and atomic `0600` replacement used identically by both stores.
- `EnvironmentGitHubActionsDispatchTokenResolver` is the only production V2 environment reader and returns `GitHubActionsDispatchToken`; formatting is redacted, and only dispatch adapters unwrap it for authorization and error sanitization.
- `ReleaseClock` is the active release/resume timestamp capability. One injected clock supplies command responses and the composed V2 execution/dispatch stores and dispatcher; model-level zero-time fallbacks remain compatibility behavior for direct callers.
- Public store, dispatcher, and runner constructors remain compatibility entry points with production defaults. The isolated V1 application/adapters, migration-specific root/filesystem adapters, and inactive `ReleaseTransaction` remain deliberately outside active V2 composition and do not compete with it. The former `GitReleaseCoordinator.Coordinate` convenience method was removed in C2; active code uses focused coordinator methods through release/resume operations.

### Git and delivery results

- `GitReleasePreflight`: branch, remote, upstream branch.
- `GitReleaseResult`: progress and recovery facts for commit/tag/push coordination.
- `GitHubActionsReleaseResult`: user-facing handoff facts.
- `ReleaseDispatchRequest`: immutable exact workflow inputs and identity.
- `GitHubRepositoryTarget`: parsed GitHub.com owner/repository from the selected upstream remote.

### Durable state machines

Execution journal confirmed phases are strictly monotonic:

```text
prepared
  -> preflight-validated
  -> materialization-applied
  -> state-written
  -> release-files-staged
  -> commit-created
  -> tag-created
  -> dispatch-journal-prepared
  -> commit-pushed
  -> tag-pushed
  -> handoff-ready
```

Every mutating phase after preflight has a matching `ReleaseExecutionPendingAction`, persisted before the side effect and cleared only by confirming the next phase. `ReleaseExecutionJournal.ConfirmPhase` prevents skipped or backward transitions and protects once-only commit/tag/dispatch identity fields.

Dispatch journal transitions are:

```text
prepared -> request-started -> accepted | rejected | unknown
```

All states after `request-started` are terminal for the current dispatcher. Existing terminal journals block another request.

Migration has a separate typed journal vocabulary and an explicit operation order:

```text
prepared -> config-written -> state-written -> v1-archived -> validated -> journal removed
```

The serialized strings remain compatible with earlier journals. New execution persists the complete target pair together and confirms `state-written`; `config-written` remains readable for interrupted older executions. The journal type validates persisted stages, while recovery policy is expressed as typed evidence classifications and file operations rather than a generic state-machine executor.

V2 pair recovery has a separate focused record at `.neko/release.pair-recovery.json` with schema version 1. It stores the exact config/state paths, prior config/state existence, bytes, modes, and hashes, intended config/state bytes and hashes, typed config/state replacement evidence (`not-started`, `pending`, `confirmed`), typed restoration evidence, and completion status. It contains no callbacks, generic state maps, plugin responses, environment values, or secrets. Unknown versions, unknown evidence values, hash mismatches, invalid modes, and inconsistent confirmed/completed evidence fail closed.

## External dependencies and side effects

| Dependency | Current access | Test seam | Risk |
| --- | --- | --- | --- |
| Working directory | `workspace.ChangeToProjectRoot`, `ToolBase.InUnitRoot`, many relative paths | temp dirs plus `os.Chdir` | Process-global state prevents parallel handler tests and hides path dependencies. |
| Filesystem | shared config pair persistence; focused init, plugin-index, migration, V1 materialization, compensation evidence/config, preflight, and executor config/file boundaries | pair replacement seams; V1 evidence-store/config ports; command-owned file/config ports; temporary directories | V1 uses its canonical single-file writer plus a private `0700` common-dir evidence directory and atomically replaced `0600` record; it intentionally does not share the V2 pair transaction. Inactive paths retain some direct `os.*`. |
| Git | active V2 release/resume use one `GitReleaseCoordinator`; active V1 uses `SystemV1GitWriter`, root-aware named compensation adapters, and a preflight repository port; direct compatibility callers may still use `V1ReleaseRollback`; queries retain command-owned read ports | fake V1 Git/evidence capabilities, coordinator runner, query capabilities, and real temp repositories | V1 destructive compensation uses a fixed V1-only evidence contract and remains intentionally isolated from V2 recovery. |
| Environment/token | V1-owned legacy token/environment ports; V2 `EnvironmentGitHubActionsDispatchTokenResolver` returning `GitHubActionsDispatchToken` | sentinel fake token/environment/process adapters | V1 and V2 intentionally retain different token types, variable injection, messages, and behavior. |
| Network | bounded, root-aware V1 GitHub Release client; injected V2 dispatch transport | fake V1 remover/client and local HTTP transport; V2 `RoundTripper` | The active V1 client has a finite timeout, bounded response reads, explicit repository root, a narrow typed-token boundary, and verified GET/DELETE/not-found behavior. |
| Time | `ReleaseClock` for release/resume responses, active V2 persistence, and V1 compensation evidence; command-owned query clocks; JReleaser init `v1Clock` | injected fixed clocks and persisted timestamp tests | V1 evidence timestamps support auditability but do not infer completion; direct compatibility model fallbacks remain. |
| External executables | V1-owned Git/executor process and binary-locator adapters; `du` and inactive paths retain direct execution | fake per-executor runners plus local fake processes | Exact V1 command order, environment, outputs, warnings, failures, and ownership are isolated and characterized. |
| Logging | package-global `log.Verbose` and direct logging throughout domain/orchestration | source assertions and output inspection | Presentation concerns occur inside planning and side-effect code. |
| Tool registry | explicit compatibility-only `pkg/release/tool` aggregator | registry characterization; production fixed catalog tests | The mutable map remains for old callers but production neither imports nor reads it. |

## Confirmed behavioral invariants

The following are current behavior. They are not statements that every behavior is ideal.

| ID | Confirmed invariant | Source evidence | Characterization evidence |
| --- | --- | --- | --- |
| INV-01 | In V2, config owns unit architecture and state owns unit versions. Tags are derived as `tagPrefix + nextVersion`; they are not stored in state. | `config.V2ReleaseConfig`, `config.V2ReleaseState`, `PlanUnitVersionBump` | `config/v2_test.go`, `planner_test.go`, `state_transaction_test.go` |
| INV-02 | A V2 release updates only the selected unit's state entry and preserves other entries. | `StateTransaction.WriteUnitVersion` | `TestStateTransactionUpdatesOnlySelectedUnit` |
| INV-03 | A plugin release materializes the selected plugin manifest from validated `ReleaseUnit.PluginManifestPath` before the release commit; any validated plugin unit follows the same rule. | `appendPluginManifestMaterialization`, `ReleaseUnit.IsPlugin`, `ReleaseUnit.PluginManifestPath` | existing self-release byte assertions plus arbitrary validated plugin-unit materialization and dry-run tests |
| INV-04 | V2 non-dry-run GitHub Actions execution resolves `GITHUB_TOKEN` before any journal or repository mutation. | first dependency call in `githubActionsReleaseUseCase.Run` | command token-before-mutation test plus the full use-case call-order and dependency-failure matrix |
| INV-05 | V2 Git preflight requires an attached branch with configured remote/upstream, an exactly clean worktree and index, and an unused target tag. It does not require `main` or `master`. | `GitReleaseCoordinator.Preflight` | `TestGitReleasePreflight*` |
| INV-06 | After planned writes, V2 stages only `.neko/release.state.json` plus materialized changes marked required for the release commit. Foreign changes block staging and are not silently unstaged. | `KnownReleaseFiles`, `Stage`, `VerifyStagedFiles`, `UnstageKnown` | staging/foreign-file tests in `git_release_coordinator_test.go` |
| INV-07 | The V2 release commit message is `chore(release): <unit> <tag>`, contains exactly the known release files, and its committed selected-unit version equals the planned next version. | `ReleaseCommitMessage`, `Commit`, `VerifyCommit` | `TestGitReleaseCommitContainsExactFilesMessageAndVersion`, runner plugin materialization test |
| INV-08 | The V2 unit tag is lightweight, encodes the selected next version, and targets the exact release commit. Re-creating an already-correct local tag is idempotent; a different target fails. | `GitReleaseCoordinator.CreateTag` | tag creation/idempotency/conflict tests |
| INV-09 | V2 pushes the release commit before the unit tag. A commit-push failure skips tag push; a tag-push failure does not roll back the pushed commit. | `Push`, `PushCommit`, `PushTag`; active runner order | coordinator push-order tests and active-runner commit-push/tag-push failure tests |
| INV-10 | The execution journal is stored as mode `0600` below `<git-common-dir>/neko/release/executions`, outside the worktree. Identity is a SHA-256 of immutable release intent including remote and base SHA. | `ReleaseExecutionIdentity`, `ReleaseExecutionJournalStore`, `releaseJournalFiles` | exact-byte/mode, identity, store, and linked-worktree tests |
| INV-11 | The execution journal records a pending action before each active mutation and confirms the matching phase after success. Phases cannot skip or move backward; once-only identifiers cannot change. | `storeAndRun`, explicit runner pending blocks, `BeginPending`, `ConfirmPhase` | execution journal state-machine/store tests plus active-runner commit, tag, and push failure recovery assertions |
| INV-12 | A dispatch journal is prepared after local commit/tag creation and before either push. Its immutable identity includes the final release commit SHA. | active runner order, `BuildReleaseDispatchRequest`, `DispatchJournalStore.Prepare` | dispatch request/journal tests; happy-path runner tests |
| INV-13 | Workflow dispatch inputs are exactly `unit`, `version`, `tag`, and `release_sha`; the ref is the unit tag and the release SHA is the verified commit. | `canonicalWorkflowDispatchInputs`, `BuildReleaseDispatchRequest` | dispatch request/client tests and workflow contract tests |
| INV-14 | Dispatch persists `request-started` before HTTP. A 2xx response is accepted; 400/401/403/404/422/429 are rejected; transport errors, redirects, 5xx, and unexpected outcomes are unknown. | `GitHubActionsDispatcher.Dispatch`, `classifyGitHubActionsDispatchResponse` | dispatcher/client response, timeout, redirect, and outbound-call journal-observation tests |
| INV-15 | A terminal dispatch journal (`request-started`, `accepted`, `rejected`, or `unknown`) prevents automatic redispatch. Unknown results are never treated as safe retries. | `DispatchJournalStore.Prepare`, `GitHubActionsDispatcher.Dispatch` | terminal-journal/state-transition tests, active-runner rejected/unknown tests, and resume no-retry tests |
| INV-16 | An ambiguous pending commit/tag push blocks resume. Resume also refuses to infer completion from `dispatch-journal-prepared` or `commit-pushed`. | `resolveResumeRecovery`, `resumeReleaseUseCase` | pure state/pending policy table, direct pending commit/tag push tests, no-inference tests, and successful `commit-created`/`tag-created`/`tag-pushed` continuation tests |
| INV-17 | Resume uses one existing unresolved journal for the selected remote and unit, never calculates a new version, and blocks when zero or multiple journals match. | `locateResumableExecution`, `FindUnresolved`, `reconstructResumeExecutionContext` | resume discovery, dry-run, application-use-case, and command-contract tests |
| INV-18 | A handoff-ready execution journal is considered resolved and is excluded by `FindUnresolved`; a new release command therefore plans from updated V2 state rather than reopening the completed transaction. | `FindUnresolved` and active runner state update | happy-path runner and explicit completed-journal exclusion tests; a subsequent active release remains uncharacterized |
| INV-19 | V2 release dry-run does not resolve a token, fetch, write state/manifests/journals, run an executor, commit, tag, push, dispatch, publish, or invoke rollback. It still validates the executor config file and reads planned file content/hashes. | `releaseStartOperation`, `ValidateRequirementsForContext`, `planV2Release` | `dry_run_test.go`, materializer tests, coordinator dry-run test |
| INV-20 | V1 preview uses local evidence, returns the calculated version, and does not fetch, resolve a token, write config, invoke Git/executor/rollback, or construct a command response. Real execution refreshes tag evidence only after preflight. | `v1ReleasePreviewUseCase`, `v1ReleaseExecutionUseCase`, `v1ReleasePlanningOperation` | V1 planner/preview/execution order tests and compatibility two-pass evidence test |
| INV-21 | Active V1 execution creates strict private evidence before mutation. Each required compensation persists pending intent before its side effect, verifies success before confirmation, and runs in the fixed config/GitHub/local-tag/remote-tag/revert-or-reset/cleanup order. Supported repeatable local pending work may continue; remote, non-repeatable, corrupt, or uncertain evidence fails closed. | `v1ReleaseExecutionUseCase`, `SelectV1CompensationOperation`, `continueV1Compensation`, named V1 compensation operations and store | characterization, schema/store/policy, operation interruption, adapter, GitHub-client, and next-invocation integration tests |
| INV-22 | V2 code does not perform destructive automatic rollback after commit/tag/push uncertainty. Local snapshots are restored only before the unsafe boundary; later failures preserve evidence. | active runner, `GitReleaseCoordinator`, `ReleaseTransaction.fail` | source assertions plus active-runner commit, tag, commit-push, and tag-push failure tests; state/materialization/store-write boundaries still lack active-runner seams |
| INV-23 | Execution and dispatch journals contain release facts and hashes, not file bytes or tokens. The typed dispatch token redacts all string formatting, and dispatch errors are capped and redact the exact secret. | journal models, `GitHubActionsDispatchToken`, `sanitizeDispatchText`, store permissions | typed-token formatting/source tests, journal/dispatcher tests, and sentinel assertions across logs, runner errors, command responses, and both journals |
| INV-24 | Public response status/error schemas and error codes are command contracts, but they are currently constructed in multiple packages. Unexpected handler errors become fatal top-level `EXECUTION_ERROR`. | `plugin.Response`, `main.main`, handler response helpers | focused V2 release/resume status, code, metadata, renderer, and ordered-item contracts; other public commands remain incomplete |
| INV-25 | Root V1 migration writes a content-hashed compatible journal before target persistence, uses the shared crash-recoverable writer for one complete V2 pair, verifies exact target bytes and strict V2 validity before archiving byte-identical V1 content, and removes the journal only after target/source verification. Recovery selects typed operations from journal, pair-recovery evidence, and file evidence. | `migrationPlanExecution.Execute`, `V2ReleasePairPersister`, migration execution adapters and policy | command-contract, journal-stage, operation-order, pair-recovery, boundary-failure, restoration, backup-verification, and interruption recovery tests |
| INV-26 | V2 local non-dry-run execution is blocked. GitHub Actions owns build and publish after accepted handoff; local release tools are not invoked by that V2 path. | `releaseStartOperation`, `GitHubActionsReleaseRunner.Run` | V2 block tests and runner fake dispatch tests |
| INV-27 | Init, unit-add, and migration validate one complete V2 config/state pair before persistence and reuse `config.V2ReleasePairPersister`. The persister creates durable pair-recovery evidence before unsafe replacement, records pending evidence before each target rename, verifies bytes before confirmation, verifies the complete intended pair before closing evidence, and can next-process restore exact prior bytes/modes/existence or close an already-complete intended pair. Ambiguous or corrupt evidence requires manual recovery. | initialization use cases, migration planner/execution, `V2ReleasePairPersister`, `.neko/release.pair-recovery.json` | focused new/update/migration, pair evidence/schema/policy, next-process recovery, temp-create/write, first/second replace, exact restore, restore-failure, cleanup, byte, and mode tests |
| INV-28 | Validate, history, contributors, and plugin-index check/render queries receive only command-owned read capabilities and do not mutate release files, Git worktree/index/refs, journals, environment, or plugin state. V1 validate still resolves its token through the requirements read, and legacy history retains suppressed Git failures. | `validationQueryUseCase`, `historyQueryUseCase`, `contributorsQueryUseCase`, `pluginIndexQueryUseCase` | parser/use-case/handler stop-point tests plus config/state/tree and real-Git worktree/index/ref immutability contracts |
| INV-29 | Plugin-index output mode builds the complete stable JSON bytes before passing them and the unchanged requested path to one atomic persister. New parents/files use `0755`/`0644`; overwrite is allowed and preserves an existing target mode; returned write/replace failures preserve the old target and clean temporary files. | `jsonPluginIndexOutputBuilder`, `atomicPluginIndexOutputPersister`, `config.AtomicFileReplacement` | exact pretty/compact schema tests plus creation, replacement, mode, injected write/replace, original-preservation, unrelated-file, and cleanup tests |
| INV-30 | Production selects V1 or V2 once from canonical `SourceFormat`; active V1 uses a fixed executor catalog and active V2 alone builds the V2 execution context. | `releaseApplicationPathSelector`, `HandleReleaseWithV1Executors`, `releaseStartOperation` | source-selector, fixed-catalog, production-composition, and architecture tests |
| INV-31 | V1 patch/minor/major planning is pure and deterministic. The typed plan owns exact next version, `v` tag, release commit metadata, `.release.neko.json`, executor identity, and materialized files without infrastructure dependencies. | `PlanV1Release`, `V1ReleasePlan` | deterministic planner table and infrastructure-free architecture guard |
| INV-32 | GoReleaser, JReleaser, and release-it preserve their distinct command/config/push/publication ownership and warning-only dry-run behavior through replaceable ports. Executor outputs/errors redact the legacy token while preserving underlying causes. | concrete `Run` methods, executor system adapters, `RedactV1ProcessResult` | command-order/failure/ownership tests, injected adapter tests, clock test, and sentinel secret tests |
| INV-33 | Migration reads canonical V1 data but cannot import V1 execution, executor, Git mutation, or rollback internals. V1 executors do not implement the inactive V2-local transaction or inspect source format. | migrate imports, concrete executor orchestration | migration-direction and executor-orchestration architecture tests |

## Architecture strengths

- V1 and V2 load into one `ReleaseRepository`/`ReleaseUnit` read model without erasing the legacy source.
- Release source selection occurs once; active V1 and V2 have distinct typed applications and do not reselect internally.
- V1 planning/preview/execution, requirements/preflight, materialization, executor invocation, Git compensation, token/environment/clock access, and response mapping have explicit owners and fake-driven seams.
- V2 JSON loading is strict and validates unit IDs, paths, workflow confinement, tag namespace overlap, plugin metadata, and config/state consistency.
- Version and tag planning is mostly pure and typed.
- Known release files make the V2 commit allowlist explicit and verifiable.
- Validated unit metadata is the sole active owner of plugin manifest materialization paths.
- State and materialization writes use atomic single-file replacement and exact snapshots.
- V2 Git coordination has a replaceable command runner and strong real-repository characterization.
- Execution and dispatch journal states are typed, monotonic, and persisted outside the worktree.
- Dispatch target parsing, redirect refusal, response classification, and token redaction are conservative and well tested.
- V2 failure policy preserves evidence after unsafe operations instead of destructive rollback.
- Migration planning, recovery policy, ordered execution, journal persistence, target persistence/verification, and source archive/verification have distinct owners and focused failure seams.
- Init, unit-add, and migration reuse one rollback-backed V2 pair persister; migration verifies the target before archiving V1.
- Validate, history, contributors, and plugin-index have typed command boundaries with command-owned read capabilities and deterministic mappers.
- Plugin-index discovery, JSON output construction, and atomic single-file persistence are distinct owners with focused failure seams.
- Manifest, routes, docs, workflows, V2 self-release state, and plugin index scripts have cross-file contract tests.

## Concrete hotspots and mixed abstraction levels

### `releaseStartOperation` and the active release use case

`HandleReleaseWithV1Executors` is the production presentation/composition entry: parse a typed request, invoke one starter, and map one typed outcome/failure with an injected clock. `releaseStartOperation` loads the repository and delegates the one source-format decision to `releaseApplicationPathSelector`. V1 then owns unit resolution and its typed preview/execution use cases; V2 alone builds the V2 execution context. For active V2 GitHub Actions execution, `GitHubActionsReleaseRunner.Run` remains a facade over the readable named-operation use case. `HandleRelease` and `newReleaseStartOperation` retain registry-backed composition only as direct compatibility facades.

### Resume composition

`HandleResume` is a typed presentation boundary over `resumeReleaseUseCase`. Discovery, assessment, context compatibility, pure policy resolution, and one named continuation operation are separate responsibilities. Production composition reuses the active release tag, dispatch-preparation, push, workflow-dispatch, and handoff capabilities with the same coordinator, journal stores, typed token, and clock; it retains conservative no-inference/no-retry policy.

### Parallel transaction paths

`ReleaseTransaction` still overlaps with concepts used by the active release use case, but `ReleaseTransaction.Execute` is deliberately blocked while F2 remains undecided. Its private V2 preparation logic is tested only as retained scaffold. No concrete V1 executor plugs into it, and the former JReleaser V2 source-format bypass was removed. C2 removed `GitReleaseCoordinator.Coordinate`; the active use case continues to call focused coordinator methods directly through named operations that interleave journal transitions. This is a bounded post-refactor limitation, not an active second orchestration path or a planned Stage 10.

### Init and configuration persistence

`HandleInit` and `HandleUnitAdd` are command boundaries: each parses one distinct typed request, invokes one focused use case, and maps one typed result or failure. Raw flags stop in `command_request.go`; pure normal/plugin unit construction, file-presence policy, and complete pair creation/append are separate. C2 removed the private `buildV2InitConfigFromFlags` bridge after tests moved to typed parser/constructor coverage.

`config.V2ReleasePairPersister` is the canonical pair writer shared by init, unit-add, and migration. It canonicalizes both values, creates `.neko`, resolves any unresolved pair evidence, captures exact bytes/modes/existence for both targets, persists `.neko/release.pair-recovery.json`, creates and fully writes/fsyncs both temporary files, records config replacement pending, renames config, verifies config bytes, confirms config replacement, records state replacement pending, renames state, verifies state bytes, confirms state replacement, strictly validates the complete intended pair, marks the evidence complete, and removes the evidence.

This is crash-recoverable paired replacement, not cross-file atomicity. A process, kernel, machine, or filesystem failure between independent renames can still expose a mixed pair, but the durable evidence lets the next pair-writing command classify the state. If both intended files are present and the complete pair validates, recovery closes the evidence without rewriting the pair. If only part of the target was applied and every observed file matches either prior or intended evidence, recovery restores exact prior bytes, modes, and existence and then proceeds with the new requested write. If current files, evidence schema, hashes, modes, or evidence values conflict, recovery fails closed with manual-recovery guidance. Pair-specific temp files are discarded during recovery when evidence proves the operation owner. No backup files or generic transaction framework are created. A failed new-pair attempt may still leave an empty `.neko` directory. Successful config/state files retain mode `0644`.

### Migration ownership and recovery

`HandleMigrate` is now a strict command boundary. Untyped flags stop in `command_request.go`; `migrationUseCase` resolves one root and one immutable plan; `response_mapper.go` alone owns `plugin.Response`. The wrong-typed `dry-run` flag still defaults to execution for compatibility. `Plan`, `ResolvePlan`, and `Run` remain narrow public compatibility facades.

Source discovery reads V1 or the byte-identical backup and captures exact bytes, mode, and existence. `planner.go` constructs the complete typed V2 config/state target and canonical bytes, validates the pair, and performs no filesystem writes. `policy.go` owns pure format/evidence classification and selects the required planning, target, and source operations. `migrationPlanExecution.Execute` makes the safety order visible and delegates to focused journal, pair-persistence, target-verification, source-archive, and archived-source-verification capabilities. The former duplicate per-file target writes and procedural `executePlan`/`archiveV1`/`validateFinal` path were removed.

A returned target-persistence failure invokes the shared persister's exact pair restoration and leaves the active V1 source plus journal evidence. Journal-confirmation or target-verification failure after a successful pair write preserves the active V1 source, target pair, and journal for evidence-driven retry. Only after exact target verification may V1 be renamed to `.release.neko.json.v1.bak`. After that rename, the hash-matched backup is the authoritative source evidence; source-confirmation, verification, or final journal-removal failure preserves the pair, backup, and journal. If restoration is incomplete, or the only remaining backup cannot be verified against the planned source, the typed failure requires manual recovery.

These guarantees cover returned filesystem errors and deterministic next-run recovery; they do not claim process- or machine-crash atomicity. A crash between the two target renames can expose a mixed pair, and a crash between an effect and its journal confirmation can leave evidence that the next run must classify. The migration journal, pair-recovery evidence, and exact file hashes make supported states recoverable or safely completable, but no generic transaction engine repairs arbitrary corruption. An empty `.neko` directory may remain after a failed attempt.

### Plugin manifest ownership

V2 config validation and normalized `ReleaseUnit` metadata own plugin manifest identity and location. Materialization consumes that metadata directly; no active hard-coded unit mapping or generic path registry remains.

### Read-only query and plugin-index ownership

Validate, history, and contributors each retain a separate user-visible query intention. Raw flags stop in their command request parser; handlers invoke one query and one mapper; query results contain typed facts rather than `plugin.Response` rows. Repository, Git, and V1 requirements reads are consumer-owned capabilities with no mutation methods. The former duplicated `getFlagString`, error-response constructors, direct clocks, and handler-level row construction were removed without introducing a shared query service or universal result.

Plugin-index is explicitly not one pure query in output mode. `pluginIndexQueryUseCase` discovers and validates typed entries through read-only config/state/manifest sources and orders them by plugin name. `jsonPluginIndexOutputBuilder` transforms the complete typed index to stable bytes. `atomicPluginIndexOutputPersister` alone selects the unchanged command-supplied target and performs the single-file effect. Check mode ends after discovery, render mode ends after building, and persist mode is the readable `query -> build -> persist` path. The persister is not used for release config/state, manifests, journals, or unrelated artifacts.

`evidence` is a read-only H3 query across the explicit evidence families: release-execution journals, dispatch journals, migration journals, V1 compensation evidence, and V2 pair-recovery evidence. It returns typed redacted records and diagnostics instead of raw JSON, keeps corrupt/unsupported/conflicting files visible without dumping their content, and orders records deterministically. `evidence-archive` is the only H3 lifecycle operation. It supports `archive-completed` only for completed release-execution, completed V1 compensation, and completed V2 pair-recovery evidence, requires family, identity, current digest, and explicit confirmation, re-observes the evidence, writes and verifies an exact private archive, then removes the completed source. Dispatch and migration evidence remain inspect-only because accepted dispatches and migration journals can still be needed for handoff audit or owner-specific recovery.

### Response and error duplication

Release start/resume, V1 preview/execution, init/unit-add, validate, history, contributors, evidence, evidence-archive, plugin-index, and migration each now have typed results/failures and command-owned response mappers with explicit clocks. The mappers remain command-specific because their schemas are not one universal result contract. V1 structured and fatal failures are classified in application code and mapped only at the release command boundary, preserving status, codes, message meaning, metadata, renderer hints, item order, nil-Go-error behavior, JSON fatal output, and deterministic timestamps. Init/unit-add intentionally retain the characterized compatibility value `init` for unit-add error metadata. Plugin-index intentionally retains top-level fatal `EXECUTION_ERROR` mapping.

### Multiple side-effect adapters

Active V2 release and resume use one coordinator boundary, one typed token boundary, shared focused journal file mechanics, and one explicit clock. V1 uses separate focused Git writer/compensation, legacy token/environment, executor process/config, file materialization, and JReleaser clock adapters because those contracts differ from V2. Migration has its own narrowly scoped root, journal, target, and source adapters. Identical low-level V1 binary lookup, file existence, and secret redaction are shared; unlike executor and V1/V2 semantics are not forced through flags or a universal manager.

## Test structure and current seams

The suite is package-local and predominantly uses temporary real files and real temporary Git repositories. This gives strong integration confidence for Git and persistence behavior but also couples many tests to process cwd and installed Git.

Existing replaceable seams include:

- init/unit-add `v2PresenceReader`, `v2PairLoader`, `v2PairValidator`, and `v2PairWriter` consumer ports, plus config/state-specific temp-create/write/replace/restore operations inside the shared pair persister;
- migration root/plan ports plus focused journal, target-pair persistence, target-verification, source-archive, and archived-source-verification capabilities;
- validate `validationRepositoryReader` and `legacyRequirementsValidator`, history `historyRepositoryReader`/`historyGitReader`, and contributors `contributorsRepositoryReader`/`contributorsGitReader` read-only capabilities;
- evidence query and archive use cases with explicit family, identity, digest, and confirmation inputs;
- plugin-index `pluginIndexSourceReader`, query/builder/persister command ports, and persistence-specific directory/stat/atomic-replacement operations;
- `gitCommandRunner` inside the single active `GitReleaseCoordinator` and shared journal common-dir mechanics.
- `GitHubActionsWorkflowDispatchClient` and injected HTTP transport.
- `GitHubActionsDispatchTokenResolver`.
- `ReleaseClock` across response mapping, active journal stores, runner, and dispatcher.
- `VersionMaterializer`.
- `transactionExecutor` in the inactive `ReleaseTransaction` preparation path.
- V1 preview/execution plan builders, requirements, preflight repository, config store, fixed executor catalog, reporter, release Git writer, compensation evidence store/policy/named operations, bounded GitHub Release client, per-executor process/config/file/environment/token/clock ports, and shared binary/file/redaction adapters.
- package variables `refreshVersionTags` and `latestVersionTag` for V1 version-guard tests.

Important missing seams include:

- process-global workspace/current-directory compatibility paths and inactive release transaction factories;
- direct compatibility callers can still reach the legacy best-effort `V1ReleaseRollback`; its behavior is not the active V1 recovery protocol;
- registry and version-evidence globals remain mutable only for compatibility tests/direct callers;
- a command-decoding policy for wrong flag types; the Stage 2 parsers deliberately preserve silent defaults because rejection would be a new public behavior.

## Bounded post-refactor limitations, prioritized

1. Active V1 compensation is interruption-safe for supported local actions, but deliberately requires manual recovery for pending/uncertain remote actions, pending revert, corrupt evidence, and uncertain executor outcomes; direct legacy rollback callers remain best-effort.
2. Pair and migration crash recovery is evidence-driven for supported config/state and archival windows, but it still refuses corrupt, externally edited, unsupported, or owner-ambiguous evidence and does not claim cross-file atomicity.
3. `ReleaseTransaction` preparation remains inactive while F2 is undecided; production does not select it. The former `GitReleaseCoordinator.Coordinate` convenience path was removed in C2.
4. Process-global workspace selection and compatibility current-directory facades still limit parallel in-process command execution.
5. Completed V2 release behavior after journal exclusion and subsequent planning remains less directly characterized than the primary release/recovery matrix.
6. Plugin-index symlink/output-confinement policy remains deliberately undefined; the established arbitrary requested-path behavior is preserved.

## Compatibility constraints for future work

- Preserve the stdin/stdout `plugin.Request`/`plugin.Response` contract.
- Preserve public command names and manifest flags unless a behavior change is explicitly requested and documented.
- Preserve stable error codes, renderer hints, data keys, and table item order until contract tests authorize a change.
- Preserve the characterized V1 behavior and compatibility facades unless a separately authorized support/removal decision changes them.
- Preserve V2 state/config ownership, unit selection, tag format, exact known-file commit contents, commit message, lightweight tag target, and commit-before-tag push order.
- Preserve journal schema versions, identity inputs, file locations/permissions, state order, pending markers, and terminal dispatch behavior.
- Preserve the `GITHUB_TOKEN` non-disclosure boundary.
- Preserve dry-run and recovery read-only guarantees.
- Preserve query-command structured-versus-fatal boundaries, deterministic row order, and plugin-index schema/format/path/overwrite/mode contracts.
- Do not activate V2 local execution, standalone dispatch/retry, or a new publication adapter as an incidental refactor.
- Do not rename or move public symbols until callers and contract tests make that change explicit.

## Final refactor status

The final architecture audit found no active V1/V2 mixed orchestration, scattered source-format selection in release execution, raw flags in application code, application-owned `plugin.Response`, generic workflow pipeline, dependency bag, versioned engine, boolean V1/V2 selector, replacement god function, duplicate active Git/journal implementation, or unbounded token/clock access in deterministic boundaries. Shared code is limited to identical contracts; V1-, V2-, migration-, and command-specific behavior remains isolated where semantics differ.

The post-refactor verification found one bounded deviation from the strict presentation rule: active V2 application and focused operation code still emits progress through the package-global terminal logger. This does not mix response construction into application code or create a second orchestrator, but it is recorded as an architecture violation in [post-refactor-review.md](post-refactor-review.md) rather than being hidden by the completed-stage ledger.

- Completed stages: 9 / 9
- Remaining stages: 0
- Release Plugin refactor: completed
- Completed roadmap milestones: H1 — Make V1 compensation interruption-safe; H2 — Make pair and migration crash recovery explicit; H3 — Add evidence-safe journal inspection and lifecycle support; C1 — Decide and deprecate V1 compatibility surfaces; C2 — Retire superseded and inactive release paths
- Next milestone: DX1 — Isolate release progress reporting

H1, H2, H3, C1, and the later milestones are maintained in [post-refactor-roadmap.md](post-refactor-roadmap.md). Roadmap milestones are not refactor stages; the historical refactor ledger remains closed.
