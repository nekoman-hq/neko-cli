# Release Plugin Current Architecture

## Purpose and audit basis

This document describes the Release Plugin as it exists in the current checkout. It is not a target package design and does not assume that an earlier refactor exists.

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
| `pkg/config` | V1/V2 disk models, strict loading, validation, normalization, unit and tag selection, atomic file writes | `ReleaseRepository`, `ReleaseUnit`, `LoadReleaseRepository`, `ValidateV2`, `ResolveReleaseUnit`, `TagSpec`, `AtomicWriteFile` | `ReleaseRepository` is the shared normalized model. V1 remains a compatibility source. |
| `pkg/init` | Typed init/unit-add command boundaries, focused initialization use cases, pure unit/pair construction, explicit file policy, and rollback-backed pair persistence | `HandleInit`, `HandleUnitAdd`, `initializeV2RepositoryUseCase`, `addV2ReleaseUnitUseCase`, `v2ReleasePairPersister` | Handlers parse, invoke one use case, and map a typed result/failure; persistence prepares both targets and has a bounded restore contract. |
| `pkg/migrate` | Plan, execute, journal, recover, and present root V1-to-V2 migration | `ResolvePlan`, `Run`, `executePlan`, `HandleMigrate` | Uses a worktree migration journal distinct from release journals. |
| `pkg/validate` | Load V1/V2, validate requirements, render config rows | `HandleValidate`, `validateV1Response`, `validateV2Response` | V1 validation resolves `GITHUB_TOKEN`; V2 config validation does not. |
| `pkg/history` | Unit selection, unit tag discovery, path-filtered commit counts, response mapping | `HandleHistory`, `handleV2History` | V1 uses every tag; V2 uses exact `TagSpec` matches. |
| `pkg/contributors` | Unit selection, path-filtered Git shortlog, response mapping | `HandleContributors` | V1 and V2 Git access share direct package functions. |
| `pkg/pluginindex` | Generate deterministic public plugin registry data, optionally write or render it | `HandlePluginIndex`, `Generate`, `WriteWithOptions` | Handler errors are returned as Go errors and become `EXECUTION_ERROR` in `main`. |
| `pkg/git` | Legacy Git queries, V1 preflight, tag/history queries, and destructive V1 rollback helpers | `IsClean`, `LatestTag`, `UnitTagsInHistory`, `DeleteRemoteTag`, `HardResetTo` | Direct `exec.Command` and `http.DefaultClient`; mostly no injected seam. |
| `pkg/release` planning | Version bump, execution context, delivery/capability descriptions, materialization plan | `PlanUnitVersionBump`, `BuildReleaseExecutionContext`, `ResolveDelivery`, `ResolveExecutorCapabilities`, `ResolveVersionMaterializer` | Useful typed models exist, but some capability data describes inactive V2 local behavior. |
| `pkg/release` V1 | V1 preflight, version guard, executor registry, local executor execution and rollback | `Service.Run`, `Preflight`, `Tool`, `ToolBase`, `VersionGuard` | Uses global tool registration and direct process/Git/environment dependencies. |
| `pkg/release` V2 GitHub Actions | Typed command boundary, active release use case, named journaled operations, and production facade | `releaseCommandHandler`, `releaseStartOperation`, `githubActionsReleaseUseCase.Run`, `GitHubActionsReleaseRunner.Run` | The facade composes one coordinator, one typed token boundary, and one clock; the use case owns the visible safety order and delegates each mutation to a focused operation. |
| `pkg/release` V2 Git | Preflight, targeted staging, exact commit verification, tag creation, ordered pushes, dispatch verification, and recovery tag inspection | `GitReleaseCoordinator`, `githubActionsReleaseGitAdapter`, `gitReleaseDispatchVerifier`, `resumeGitAdapter` | Active release/resume share one coordinator instance through consumer-owned capabilities; `Coordinate` remains an inactive convenience path. |
| `pkg/release` state/files | Plan and apply version files; update and restore V2 state | `MaterializationTransaction`, `StateTransaction`, `KnownReleaseFiles` | Snapshots support bounded local restore before commit uncertainty. |
| `pkg/release` execution journal | Durable intended-release identity, monotonic phases, pending actions, and execution-specific persistence | `ReleaseExecutionJournal`, `ReleaseExecutionJournalStore` | Store-specific validation/mutations use the shared fixed journal location and secure-write mechanics below the Git common directory. |
| `pkg/release` dispatch | Immutable workflow request, dispatch-specific persistence/classification, typed token, GitHub target, and HTTP client | `ReleaseDispatchRequest`, `DispatchJournalStore`, `GitHubActionsDispatchToken`, `GitHubActionsDispatcher`, `GitHubActionsDispatchClient` | Explicit accepted/rejected/unknown outcomes; the token stays typed and redacted through dispatch adapters. |
| `pkg/release` recovery | Typed command boundary, read-only assessment, pure continuation policy, and reuse of active named operations | `resumeCommandHandler`, `resumeReleaseUseCase`, `AssessReleaseExecutionRecovery`, `resolveResumeRecovery` | Recovery receives focused Git evidence and reuses active tag/dispatch/push/handoff capabilities without a second orchestration path. |
| `pkg/release/tool/*` | GoReleaser, JReleaser, and release-it V1/local adapter behavior | `GoReleaser`, `JReleaser`, `ReleaseIt` | Tools own subprocesses and mutable rollback state. |

## Command-to-flow map

### `init`

- Entry: `main.main` -> `init.HandleInit`.
- Request parsing: `parseInitCommandRequest` is the only init path that reads the untyped flag map and produces `initCommandRequest`; wrong raw types retain the prior zero-value/default behavior.
- Application boundary: `initializeV2RepositoryUseCase.Execute` applies the pure V1/V2/force policy, constructs one normal or plugin unit, creates a complete config/state pair, validates it, and passes it to the pair writer.
- Domain ownership: `unit_constructor.go` owns defaults and normal/plugin construction; `policy.go` owns side-effect-free file-presence decisions; `repository.go` owns complete pair creation and repository validation.
- Side effects: `v2ReleasePairPersister` snapshots both targets, creates and writes both temporary files, then replaces config followed by state. Returned replace failures trigger restoration of both snapshots.
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

- Entry: `main.main` -> `migrate.HandleMigrate` -> `migrate.Run`.
- Planning: `ResolvePlan` resolves the Git root and selects new migration, recovery, already-complete, or conflict behavior.
- Side effects: creates `.neko`; writes a content-hashed migration journal; writes config; writes state; renames V1 to `.release.neko.json.v1.bak`; validates; removes the journal.
- State: migration stages are string constants `prepared`, `config-written`, `state-written`, and `v1-archived`.
- Dry-run: returns exact planned config/state JSON without creating `.neko`.
- Tests: `pkg/migrate/migration_test.go` covers normal migration, dry-run, already migrated, conflicts, and interruption recovery at every recorded stage.
- Missing characterization: injected failure on each write/rename/journal removal and rejection of unknown journal stage values. The journal stage is not a typed validated state machine.

### `patch`, `minor`, and `major`

- Entry: `main.main` -> `release.HandleRelease` -> `releaseCommandHandler` with a typed `release.Type`.
- Parsing: `ParseReleaseCommandRequest` is the only release-start code that reads the untyped plugin flag map and produces `ReleaseCommandRequest`. Missing or wrongly typed flags preserve the existing zero-value defaults.
- Application boundary: the handler invokes `releaseCommandStarter.Start` exactly once. Production wiring uses `releaseStartOperation`, which still loads the repository, resolves the unit, builds execution context, and selects the V1 or V2 compatibility path.
- Branch: V2 uses `ReleaseExecutionContext`; V1 uses `Service` and the legacy executor registry.
- Response: application code returns a sealed `ReleaseCommandOutcome` or typed `CommandFailure`; `MapReleaseCommandOutcome` and `MapCommandFailure` construct the stable response from an explicit handler-supplied timestamp.
- Tests: planning, context, requirements, dry-run, materialization, V2 Git coordination, active GitHub Actions happy paths, and V1 rollback guard tests are distributed through `pkg/release/*_test.go`.

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

- Orchestration: `Service.Run` -> `Preflight` -> `VersionGuard` -> registered `Tool` -> `executeRelease`.
- Preconditions: executor file and `GITHUB_TOKEN`; clean worktree; attached `main` or `master`; configured upstream; branch not reported behind.
- State mutation: V1 config version is written before executor execution. Each tool may update its own version files, create a release commit/tag, push, and publish.
- Recovery: on a post-mutation failure, the config is restored and `Tool.RevertRelease` may delete GitHub releases/tags, create reverts, hard-reset local state, and clean untracked files based on recorded `GitReleaseState` flags.
- Tests: V1 dry-run, version guards, requirements, configuration compatibility, and the empty-state rollback guard are covered. Full executor side-effect ordering and rollback outcomes lack isolated characterization.

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

- Entry: `history.HandleHistory`.
- V1: uses all local tags and direct commit counts.
- V2: exact unit-prefixed tags reachable from `HEAD`, ordered by history, with counts constrained to unit pathspecs.
- Tests: `history_test.go` and `git/tag_test.go` cover unit filtering, path counts, and explicit selection.
- Missing characterization: errors from each Git query and deterministic behavior when multiple matching tags point to one commit.

### `contributors`

- Entry: `contributors.HandleContributors`.
- V1: repository-wide `git shortlog`; V2: selected-unit pathspecs.
- Tests: `contributors_test.go` and `git/tag_test.go`.
- Missing characterization: Git failure response contract and stable item ordering beyond Git output order.

### `validate`

- Entry: `validate.HandleValidate`.
- V2: strict load/validation already occurred in `LoadReleaseRepository`; optional `--show` renders normalized unit facts.
- V1: revalidates config and checks token/executor configuration through `ValidateRequirements`.
- Tests: `validate_test.go`, config tests, workflow validation tests.
- Missing characterization: stable error mapping for every invalid V2 file combination and whether read-only V1 validation should continue to require a token; the current token requirement is behavior, not a target recommendation.

### `plugin-index`

- Entry: `pluginindex.HandlePluginIndex` -> `Generate`.
- Decisions: load V2 config/state, select `kind: plugin` units, validate manifest name/version, reject duplicate names/tags, sort by plugin name.
- Side effects: optional direct `os.WriteFile` to an arbitrary requested output path; default output is raw JSON.
- Tests: generator validation, deterministic order, check/default/output modes, and workflow script integration.
- Missing characterization: stable plugin error code/response behavior, atomic output writes, output path confinement policy, filesystem failure injection, and clock/cancellation behavior.

## Important data models

### Repository and unit

`config.ReleaseRepository` is the normalized read model shared by V1 and V2. V1 becomes one virtual `default` unit. `config.ReleaseUnit` combines immutable config facts with the current version from state. This avoids format branching in history, contributors, and most planning.

The source-of-truth disk models are:

- V1: `config.V1ReleaseConfig` in `.release.neko.json`.
- V2 architecture: `config.V2ReleaseConfig` in `.neko/release.config.json`.
- V2 mutable versions: `config.V2ReleaseState` in `.neko/release.state.json`.

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
- Public store, dispatcher, and runner constructors remain compatibility entry points with production defaults. V1 token/Git/tool adapters, migration Git/filesystem code, inactive `ReleaseTransaction`, and `GitReleaseCoordinator.Coordinate` remain deliberately outside this stage and do not compete with the active V2 path.

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

Migration has a third, implicit state machine expressed as strings and ordered function calls:

```text
prepared -> config-written -> state-written -> v1-archived -> validated -> journal removed
```

## External dependencies and side effects

| Dependency | Current access | Test seam | Risk |
| --- | --- | --- | --- |
| Working directory | `workspace.ChangeToProjectRoot`, `ToolBase.InUnitRoot`, many relative paths | temp dirs plus `os.Chdir` | Process-global state prevents parallel handler tests and hides path dependencies. |
| Filesystem | focused init pair repository/disk boundaries; direct `os.*` remain across config/migrate/release/pluginindex/tools | init pair temp/create/write/replace/restore seams, temp directories, and a few release/migration abstractions | Init pair failures are isolated; most other handlers still cannot inject every read/write/rename failure. |
| Git | active V2 release/resume use one `GitReleaseCoordinator`; `pkg/git` and direct Git remain in V1, migration, inactive transaction, and tools | coordinator runner, focused consumer ports, and real temp Git repositories | Legacy flows still expose different error and logging semantics; Stage 5 does not rewrite them. |
| Environment/token | V1 `pkg/config.GetPAT`; V2 `EnvironmentGitHubActionsDispatchTokenResolver` returning `GitHubActionsDispatchToken` | `t.Setenv`, typed token resolver interfaces | V1 and V2 intentionally retain different token messages and behavior. |
| Network | V1 `http.DefaultClient` GitHub deletion; V2 injected `RoundTripper` dispatch client | strong V2 client seam; weak V1 seam | V1 rollback network behavior is hard to isolate. |
| Time | one `ReleaseClock` for release/resume responses and active V2 persisted timestamps; compatibility zero-time fallbacks and other commands/tools still use direct time | injected fixed clock and end-to-end persisted timestamp tests | Non-release commands, V1 tools, and direct model callers remain outside the active V2 clock boundary. |
| External executables | `git`, `goreleaser`, `jreleaser`, `npm`, `bun`, `npx`, `du` | mostly real executable lookup/subprocess | V1 executor unit tests do not isolate command ordering/failures. |
| Logging | package-global `log.Verbose` and direct logging throughout domain/orchestration | source assertions and output inspection | Presentation concerns occur inside planning and side-effect code. |
| Tool registry | global map populated by blank imports | package-global registry | Registration order/state is implicit. |

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
| INV-20 | V1 dry-run does not fetch or rewrite V1 config and reports the calculated version. V1 real release still performs legacy preflight and executor behavior. | `Service.GetNewVersion`, `VersionGuardWithOptions` | `TestDryRunDoesNotFetchOrWriteConfigAndShowsNextVersion` |
| INV-21 | V1 rollback is reachable only after `Service.Run` enters its mutating phase, and `ToolBase.RevertGitRelease` does nothing when no mutation is recorded. Once mutation is recorded, legacy destructive cleanup remains possible. | `Service.Run`, `GitReleaseState.hasMutatingStep`, `RevertGitRelease` | `TestRevertGitReleaseWithoutMutatingStepIsNoop`; full rollback characterization is missing |
| INV-22 | V2 code does not perform destructive automatic rollback after commit/tag/push uncertainty. Local snapshots are restored only before the unsafe boundary; later failures preserve evidence. | active runner, `GitReleaseCoordinator`, `ReleaseTransaction.fail` | source assertions plus active-runner commit, tag, commit-push, and tag-push failure tests; state/materialization/store-write boundaries still lack active-runner seams |
| INV-23 | Execution and dispatch journals contain release facts and hashes, not file bytes or tokens. The typed dispatch token redacts all string formatting, and dispatch errors are capped and redact the exact secret. | journal models, `GitHubActionsDispatchToken`, `sanitizeDispatchText`, store permissions | typed-token formatting/source tests, journal/dispatcher tests, and sentinel assertions across logs, runner errors, command responses, and both journals |
| INV-24 | Public response status/error schemas and error codes are command contracts, but they are currently constructed in multiple packages. Unexpected handler errors become fatal top-level `EXECUTION_ERROR`. | `plugin.Response`, `main.main`, handler response helpers | focused V2 release/resume status, code, metadata, renderer, and ordered-item contracts; other public commands remain incomplete |
| INV-25 | Root V1 migration writes a content-hashed journal before V2 files, archives byte-identical V1 content, validates the final V2 repository, and removes the journal only after success. | `migrate.executePlan`, `archiveV1`, `validateFinal` | all migration and interruption recovery tests |
| INV-26 | V2 local non-dry-run execution is blocked. GitHub Actions owns build and publish after accepted handoff; local release tools are not invoked by that V2 path. | `releaseStartOperation`, `GitHubActionsReleaseRunner.Run` | V2 block tests and runner fake dispatch tests |
| INV-27 | Init and unit-add validate one complete V2 config/state pair before persistence. Both temporary files are created, written, chmodded, and fsynced before config then state replacement; a returned replace failure attempts exact restoration of both prior byte/mode/existence snapshots. | `initializeV2RepositoryUseCase`, `addV2ReleaseUnitUseCase`, `v2ReleasePairPersister` | focused new/update, temp-create/write, first/second replace, exact restore, restore-failure, cleanup, byte, and mode tests |

## Architecture strengths

- V1 and V2 load into one `ReleaseRepository`/`ReleaseUnit` read model without erasing the legacy source.
- V2 JSON loading is strict and validates unit IDs, paths, workflow confinement, tag namespace overlap, plugin metadata, and config/state consistency.
- Version and tag planning is mostly pure and typed.
- Known release files make the V2 commit allowlist explicit and verifiable.
- Validated unit metadata is the sole active owner of plugin manifest materialization paths.
- State and materialization writes use atomic single-file replacement and exact snapshots.
- V2 Git coordination has a replaceable command runner and strong real-repository characterization.
- Execution and dispatch journal states are typed, monotonic, and persisted outside the worktree.
- Dispatch target parsing, redirect refusal, response classification, and token redaction are conservative and well tested.
- V2 failure policy preserves evidence after unsafe operations instead of destructive rollback.
- Manifest, routes, docs, workflows, V2 self-release state, and plugin index scripts have cross-file contract tests.

## Concrete hotspots and mixed abstraction levels

### `releaseStartOperation` and the active release use case

`HandleRelease` is a presentation boundary: parse a typed request, invoke one starter, and map one typed outcome/failure with an injected clock. The transitional `releaseStartOperation` still loads repository state, selects units, builds execution context, branches between V1 and V2, validates/plans dry-runs, constructs the production runner, and invokes execution. For active V2 GitHub Actions execution, `GitHubActionsReleaseRunner.Run` is now a facade; `githubActionsReleaseUseCase.Run` owns the readable operation order and delegates planning, preflight, journals, state/files, Git, dispatch, and handoff to explicit replaceable operations.

### Resume composition

`HandleResume` is a typed presentation boundary over `resumeReleaseUseCase`. Discovery, assessment, context compatibility, pure policy resolution, and one named continuation operation are separate responsibilities. Production composition reuses the active release tag, dispatch-preparation, push, workflow-dispatch, and handoff capabilities with the same coordinator, journal stores, typed token, and clock; it retains conservative no-inference/no-retry policy.

### Parallel transaction paths

`ReleaseTransaction` and `GitReleaseCoordinator.Coordinate` still overlap with concepts used by the active release use case. `ReleaseTransaction.Execute` is deliberately blocked, while its private preparation logic is tested. `Coordinate` is functional but bypassed by the active use case, whose named operations call coordinator methods directly to interleave journal transitions. The active production boundary is now explicit, but these inactive/convenience paths remain later consolidation work.

### Init and configuration persistence

`HandleInit` and `HandleUnitAdd` are command boundaries: each parses one distinct typed request, invokes one focused use case, and maps one typed result or failure. Raw flags stop in `command_request.go`; pure normal/plugin unit construction, file-presence policy, and complete pair creation/append are separate. `buildV2InitConfigFromFlags` remains only as a narrow compatibility seam over the typed parser and constructor.

`v2ReleasePairPersister` is the sole init pair writer. It canonicalizes both values, creates `.neko`, captures exact bytes/modes/existence for both targets in memory, creates and fully writes/fsyncs both temporary files, then renames config followed by state. A returned error from either rename triggers restoration of both snapshots: an existing target is atomically restored with its exact bytes and mode, while a previously absent target is removed. Both restoration attempts run even when the first fails, temporary files are discarded, and any restoration failure is surfaced with `manual recovery required`. No backup files are created.

This is bounded rollback, not cross-file atomicity. A process, kernel, machine, or filesystem failure between successful renames can still leave a mixed pair because no single cross-file atomic primitive exists. A failed new-pair attempt may leave an empty `.neko` directory. Successful config/state files retain mode `0644`.

### Plugin manifest ownership

V2 config validation and normalized `ReleaseUnit` metadata own plugin manifest identity and location. Materialization consumes that metadata directly; no active hard-coded unit mapping or generic path registry remains.

### Response and error duplication

Release start and resume centralize typed failures and exact response mapping in `command_response.go`, with explicit timestamps supplied by their handler clocks. Init and unit-add now share `response_mapper.go`, but intentionally retain the characterized compatibility value `init` for unit-add error metadata; correcting that value is a later explicit public-contract decision. Other command packages still create timestamps, metadata, table rows, and error structures independently. `history` and `contributors` duplicate the same helpers. `plugin-index` often returns Go errors, unlike handlers that return structured error responses. Stable contracts outside release start/resume/init therefore remain distributed.

### Multiple side-effect adapters

Active V2 release and resume now use one coordinator boundary, one typed token boundary, shared focused journal file mechanics, and one explicit clock. `pkg/git`, V1 token lookup, direct subprocesses in `ReleaseTransaction`, migration, and tools, and direct clocks outside active V2 remain compatibility paths for later stages; they retain differing semantics but do not compete inside the active V2 flow.

## Test structure and current seams

The suite is package-local and predominantly uses temporary real files and real temporary Git repositories. This gives strong integration confidence for Git and persistence behavior but also couples many tests to process cwd and installed Git.

Existing replaceable seams include:

- init/unit-add `v2PresenceReader`, `v2PairLoader`, `v2PairValidator`, and `v2PairWriter` consumer ports, plus config/state-specific temp-create/write/replace/restore operations inside the focused pair persister;
- `gitCommandRunner` inside the single active `GitReleaseCoordinator` and shared journal common-dir mechanics.
- `GitHubActionsWorkflowDispatchClient` and injected HTTP transport.
- `GitHubActionsDispatchTokenResolver`.
- `ReleaseClock` across response mapping, active journal stores, runner, and dispatcher.
- `VersionMaterializer`.
- `transactionExecutor` in the inactive `ReleaseTransaction` preparation path.
- package variables `refreshVersionTags` and `latestVersionTag` for V1 version-guard tests.

Important missing seams include:

- filesystem/config/state repositories for flows outside init and active release; active release transaction factories and journal operation ports permit failure injection without changing production stores;
- replaceable facade construction inside `releaseStartOperation`; release and resume composition itself now injects focused coordinator, store, dispatch, token, and clock dependencies;
- one Git port used by all Release Plugin flows;
- subprocess runners for V1 tools;
- an HTTP client for V1 GitHub rollback;
- a command-decoding policy for wrong flag types; the Stage 2 parsers deliberately preserve silent defaults because rejection would be a new public behavior.

## Missing characterization coverage, prioritized

1. Stable command contracts outside V2 release/resume/init/unit-add: exact error code/message/details, response metadata command, renderer hint, and deterministic item order for every other public command.
2. V1 executor order and rollback characterization with fake subprocess/network adapters before any extraction.
3. Remaining secret non-disclosure and filesystem/journal failure paths outside the active release operation seams.
4. Completed release behavior after exclusion: subsequent active version planning from the committed V2 state.
5. Migration injected failure at every unsafe filesystem boundary and unknown/corrupt journal stage handling.
6. History, contributors, validate, and plugin-index Git/filesystem error mapping and stable output ordering.

## Compatibility constraints for future work

- Preserve the stdin/stdout `plugin.Request`/`plugin.Response` contract.
- Preserve public command names and manifest flags unless a behavior change is explicitly requested and documented.
- Preserve stable error codes, renderer hints, data keys, and table item order until contract tests authorize a change.
- Preserve V1 behavior, including current token and rollback semantics, during behavior-preserving extraction.
- Preserve V2 state/config ownership, unit selection, tag format, exact known-file commit contents, commit message, lightweight tag target, and commit-before-tag push order.
- Preserve journal schema versions, identity inputs, file locations/permissions, state order, pending markers, and terminal dispatch behavior.
- Preserve the `GITHUB_TOKEN` non-disclosure boundary.
- Preserve dry-run and recovery read-only guarantees.
- Do not activate V2 local execution, standalone dispatch/retry, or a new publication adapter as an incidental refactor.
- Do not rename or move public symbols until callers and contract tests make that change explicit.

Active release and resume share canonical metadata-driven release files, focused Git capabilities, store-specific journals over common secure file mechanics, one typed V2 token, and one explicit clock. Init and unit-add now have typed boundaries, focused application use cases, pure policy/construction/mutation, and bounded rollback-backed pair persistence without a generic transaction framework. The exact next refactor stage is Stage 7: extract read-only query use cases and plugin-index output persistence.
