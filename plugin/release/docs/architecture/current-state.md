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
| `pkg/init` | Parse init/unit-add flags, validate, construct V2 config/state, write both files, build responses | `HandleInit`, `HandleUnitAdd`, `buildV2InitConfigFromFlags`, `writeV2Files` | Parsing, use-case logic, two-file persistence, and presentation are in one file. |
| `pkg/migrate` | Plan, execute, journal, recover, and present root V1-to-V2 migration | `ResolvePlan`, `Run`, `executePlan`, `HandleMigrate` | Uses a worktree migration journal distinct from release journals. |
| `pkg/validate` | Load V1/V2, validate requirements, render config rows | `HandleValidate`, `validateV1Response`, `validateV2Response` | V1 validation resolves `GITHUB_TOKEN`; V2 config validation does not. |
| `pkg/history` | Unit selection, unit tag discovery, path-filtered commit counts, response mapping | `HandleHistory`, `handleV2History` | V1 uses every tag; V2 uses exact `TagSpec` matches. |
| `pkg/contributors` | Unit selection, path-filtered Git shortlog, response mapping | `HandleContributors` | V1 and V2 Git access share direct package functions. |
| `pkg/pluginindex` | Generate deterministic public plugin registry data, optionally write or render it | `HandlePluginIndex`, `Generate`, `WriteWithOptions` | Handler errors are returned as Go errors and become `EXECUTION_ERROR` in `main`. |
| `pkg/git` | Legacy Git queries, V1 preflight, tag/history queries, and destructive V1 rollback helpers | `IsClean`, `LatestTag`, `UnitTagsInHistory`, `DeleteRemoteTag`, `HardResetTo` | Direct `exec.Command` and `http.DefaultClient`; mostly no injected seam. |
| `pkg/release` planning | Version bump, execution context, delivery/capability descriptions, materialization plan | `PlanUnitVersionBump`, `BuildReleaseExecutionContext`, `ResolveDelivery`, `ResolveExecutorCapabilities`, `ResolveVersionMaterializer` | Useful typed models exist, but some capability data describes inactive V2 local behavior. |
| `pkg/release` V1 | V1 preflight, version guard, executor registry, local executor execution and rollback | `Service.Run`, `Preflight`, `Tool`, `ToolBase`, `VersionGuard` | Uses global tool registration and direct process/Git/environment dependencies. |
| `pkg/release` V2 GitHub Actions | Active V2 transaction orchestration | `HandleRelease`, `GitHubActionsReleaseRunner.Run` | The runner coordinates nearly every release phase and journal transition. |
| `pkg/release` V2 Git | Preflight, targeted staging, exact commit verification, tag creation, ordered pushes | `GitReleaseCoordinator` | Has an injectable command runner; its `Coordinate` convenience path is not the active runner path. |
| `pkg/release` state/files | Plan and apply version files; update and restore V2 state | `MaterializationTransaction`, `StateTransaction`, `KnownReleaseFiles` | Snapshots support bounded local restore before commit uncertainty. |
| `pkg/release` execution journal | Durable intended-release identity, monotonic phases, pending actions, atomic store | `ReleaseExecutionJournal`, `ReleaseExecutionJournalStore` | Stored below the Git common directory, outside the worktree. |
| `pkg/release` dispatch | Immutable workflow request, dispatch journal, GitHub target, HTTP client, outcome classification | `ReleaseDispatchRequest`, `DispatchJournal`, `GitHubActionsDispatcher`, `GitHubActionsDispatchClient` | Explicit accepted/rejected/unknown outcomes; tokens are adapter inputs only. |
| `pkg/release` recovery | Read-only assessment and conservative continuation | `AssessReleaseExecutionRecovery`, `HandleResume`, `resumeJournal` | Resume reimplements portions of the active runner ordering. |
| `pkg/release/tool/*` | GoReleaser, JReleaser, and release-it V1/local adapter behavior | `GoReleaser`, `JReleaser`, `ReleaseIt` | Tools own subprocesses and mutable rollback state. |

## Command-to-flow map

### `init`

- Entry: `main.main` -> `init.HandleInit`.
- Request parsing: `getFlagBool` and `buildV2InitConfigFromFlags` read untyped flag values.
- Domain decisions: existing V1/V2 conflict rules, defaults, executor/delivery/kind rules, and plugin metadata are selected inside `pkg/init/handler.go`.
- Side effects: `writeV2Files` creates `.neko`, atomically writes config, then atomically writes state.
- State mutations: replaces both V2 files when permitted; `--force` never overwrites V1.
- Output: handler constructs text-oriented success data or `initErrorResponse`.
- Error behavior: stable codes include `CONFIG_CONFLICT`, `V1_CONFIG_EXISTS`, `CONFIG_EXISTS`, `INVALID_FLAGS`, `VALIDATION_ERROR`, and `SAVE_ERROR`.
- Existing tests: `pkg/init/handler_test.go` covers options, local/GitHub Actions/plugin creation, conflicts, force, and many invalid inputs.
- Missing characterization: failure between the config and state atomic writes, exact preservation behavior after that failure, response metadata for `unit-add` errors, and injected filesystem failures.

### `unit-add`

- Entry: `main.main` -> `init.HandleUnitAdd`.
- Orchestration: loads both V2 files, constructs a unit and state entry, validates the combined target, serializes, then calls the same two-file writer as init.
- State mutation: appends one config unit and one state map entry; existing units must not be overwritten.
- Existing tests: normal/plugin append, partial/missing configuration, duplicate unit/state/plugin name, and invalid inputs in `pkg/init/handler_test.go`.
- Missing characterization: crash/failure atomicity across the two files and preservation of byte formatting for existing unrelated units.

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

- Entry: `main.main` -> `release.HandleRelease` with a typed `release.Type`.
- Shared parsing: load repository, read `dry-run` and `unit`, require explicit unit selection for multi-unit repositories.
- Branch: V2 uses `ReleaseExecutionContext`; V1 uses `Service` and the legacy executor registry.
- Response: `HandleRelease`, `v2DryRunPlanResponse`, and `githubActionsReleaseResponse` construct presentation objects directly.
- Tests: planning, context, requirements, dry-run, materialization, V2 Git coordination, active GitHub Actions happy paths, and V1 rollback guard tests are distributed through `pkg/release/*_test.go`.

#### V2 dry-run

- Orchestration: `BuildReleaseExecutionContext` -> `ValidateRequirementsForContext` -> `v2DryRunPlanResponse`.
- Decisions: calculate next SemVer and tag; resolve delivery and executor capabilities; plan materialization; calculate known release files; build a dispatch summary.
- Side effects: reads config/state, executor config, manifests, and file hashes; emits logs and a timestamped response. It does not resolve a token or write journals/files/refs/remotes.
- Tests: all `TestHandleRelease*DryRun*` cases in `dry_run_test.go`, plus materializer and coordinator dry-run tests.
- Missing characterization: a single dependency-spy test proving that every network, clock-independent mutation, Git mutation, and journal store remains unused across each executor/delivery combination.

#### V2 GitHub Actions execution

- Orchestration: `GitHubActionsReleaseRunner.Run`.
- Ordered side effects: token resolution; materialization planning; Git preflight; execution-journal preparation; materialization; state write; targeted stage; commit; tag; dispatch-journal preparation; commit push; tag push; workflow dispatch; handoff confirmation.
- State: two journals are updated manually around mutations, while `GitReleaseResult` supplies dispatch request facts.
- Output: a `GitHubActionsReleaseResult` is mapped to the plugin response in the same file.
- Existing tests: `github_actions_release_runner_test.go` covers two complete accepted happy paths; lower-level tests cover many components.
- Missing characterization: end-to-end failure injection before and after every unsafe runner step, journal-write failure after a successful side effect, rejected/unknown dispatch at runner level, unresolved-journal blocking, and no-secret assertions for all returned errors/logs.

#### V2 local execution

- `HandleRelease` returns `V2_LOCAL_DELIVERY_BLOCKED` before executing a local transaction.
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

- Entry: `main.main` -> `release.HandleResume` -> `resumeJournal`.
- Selection: requires V2; resolves one unit and its current upstream remote; finds exactly one unresolved execution journal matching remote URL and unit.
- Assessment: `AssessReleaseExecutionRecovery` verifies journal structure, known-file hashes, and local tag evidence without remote access.
- Dry-run: returns that assessment without requiring `GITHUB_TOKEN` or modifying the journal/worktree.
- Non-dry-run: blocks corrupt/conflicted journals and pending push actions; reconstructs context from the journal; requires current config to match; then handles only selected journal phases.
- Continuation: can create a missing tag after confirmed commit, prepare dispatch journal, push commit then tag from the `tag-created` state, or dispatch from a confirmed `tag-pushed` state.
- Restrictions: it will not calculate a new version, continue before a confirmed commit, prove ambiguous push completion, or redispatch a terminal dispatch journal.
- Existing tests: dry-run read-only behavior, no-journal behavior, exactly-one selection, and pre-commit blocking in `resume_test.go`; assessment statuses/conflicts in `release_execution_recovery_test.go`.
- Missing characterization: successful resume from `commit-created`, `tag-created`, and `tag-pushed`; already-existing expected tag behavior; each dispatch terminal state; config drift variants; remote identity drift; and side-effect failure injection.

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

`plugin_manifest_materializer.go` currently selects plugin manifests through the hard-coded `pluginManifestPathsByUnit` map for `plugin-release` and `plugin-ui`, even though `ReleaseUnit.PluginManifestPath` already carries validated V2 metadata.

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
| Filesystem | direct `os.*` across config/init/migrate/release/pluginindex/tools | temp directories; a few transaction abstractions | Most handlers cannot inject read/write/rename failures. |
| Git | `pkg/git` direct subprocesses; `GitReleaseCoordinator` runner; direct Git in migration/transaction/tools | coordinator runner and real temp Git repositories | Multiple adapters expose different error and logging semantics. |
| Environment/token | `pkg/config.GetPAT` and `EnvironmentGitHubActionsDispatchTokenResolver` | `t.Setenv`, token resolver interfaces | V1 and V2 use different token abstractions and messages. |
| Network | V1 `http.DefaultClient` GitHub deletion; V2 injected `RoundTripper` dispatch client | strong V2 client seam; weak V1 seam | V1 rollback network behavior is hard to isolate. |
| Time | direct `time.Now` in handlers/models; injected store/dispatcher clocks; JReleaser init year | partial injected clocks | Response and initial journal timestamps are nondeterministic at some boundaries. |
| External executables | `git`, `goreleaser`, `jreleaser`, `npm`, `bun`, `npx`, `du` | mostly real executable lookup/subprocess | V1 executor unit tests do not isolate command ordering/failures. |
| Logging | package-global `log.Verbose` and direct logging throughout domain/orchestration | source assertions and output inspection | Presentation concerns occur inside planning and side-effect code. |
| Tool registry | global map populated by blank imports | package-global registry | Registration order/state is implicit. |

## Confirmed behavioral invariants

The following are current behavior. They are not statements that every behavior is ideal.

| ID | Confirmed invariant | Source evidence | Characterization evidence |
| --- | --- | --- | --- |
| INV-01 | In V2, config owns unit architecture and state owns unit versions. Tags are derived as `tagPrefix + nextVersion`; they are not stored in state. | `config.V2ReleaseConfig`, `config.V2ReleaseState`, `PlanUnitVersionBump` | `config/v2_test.go`, `planner_test.go`, `state_transaction_test.go` |
| INV-02 | A V2 release updates only the selected unit's state entry and preserves other entries. | `StateTransaction.WriteUnitVersion` | `TestStateTransactionUpdatesOnlySelectedUnit` |
| INV-03 | A plugin release materializes the selected plugin manifest before the release commit; today this is restricted to the two hard-coded plugin unit IDs. | `appendPluginManifestMaterialization`, `pluginManifestPathsByUnit` | plugin materializer and dry-run tests; missing coverage for arbitrary validated plugin metadata |
| INV-04 | V2 non-dry-run GitHub Actions execution resolves `GITHUB_TOKEN` before any journal or repository mutation. | early token resolution in `GitHubActionsReleaseRunner.Run` | `TestHandleReleaseV2GitHubActionsRequiresTokenBeforeMutation`; no full dependency-spy assertion |
| INV-05 | V2 Git preflight requires an attached branch with configured remote/upstream, an exactly clean worktree and index, and an unused target tag. It does not require `main` or `master`. | `GitReleaseCoordinator.Preflight` | `TestGitReleasePreflight*` |
| INV-06 | After planned writes, V2 stages only `.neko/release.state.json` plus materialized changes marked required for the release commit. Foreign changes block staging and are not silently unstaged. | `KnownReleaseFiles`, `Stage`, `VerifyStagedFiles`, `UnstageKnown` | staging/foreign-file tests in `git_release_coordinator_test.go` |
| INV-07 | The V2 release commit message is `chore(release): <unit> <tag>`, contains exactly the known release files, and its committed selected-unit version equals the planned next version. | `ReleaseCommitMessage`, `Commit`, `VerifyCommit` | `TestGitReleaseCommitContainsExactFilesMessageAndVersion`, runner plugin materialization test |
| INV-08 | The V2 unit tag is lightweight, encodes the selected next version, and targets the exact release commit. Re-creating an already-correct local tag is idempotent; a different target fails. | `GitReleaseCoordinator.CreateTag` | tag creation/idempotency/conflict tests |
| INV-09 | V2 pushes the release commit before the unit tag. A commit-push failure skips tag push; a tag-push failure does not roll back the pushed commit. | `Push`, `PushCommit`, `PushTag`; active runner order | coordinator push-order tests and active-runner commit-push/tag-push failure tests |
| INV-10 | The execution journal is stored as mode `0600` below `<git-common-dir>/neko/release/executions`, outside the worktree. Identity is a SHA-256 of immutable release intent including remote and base SHA. | `ReleaseExecutionIdentity`, `ReleaseExecutionJournalStore` | execution journal identity/store/worktree tests |
| INV-11 | The execution journal records a pending action before each active mutation and confirms the matching phase after success. Phases cannot skip or move backward; once-only identifiers cannot change. | `storeAndRun`, explicit runner pending blocks, `BeginPending`, `ConfirmPhase` | execution journal state-machine/store tests plus active-runner commit, tag, and push failure recovery assertions |
| INV-12 | A dispatch journal is prepared after local commit/tag creation and before either push. Its immutable identity includes the final release commit SHA. | active runner order, `BuildReleaseDispatchRequest`, `DispatchJournalStore.Prepare` | dispatch request/journal tests; happy-path runner tests |
| INV-13 | Workflow dispatch inputs are exactly `unit`, `version`, `tag`, and `release_sha`; the ref is the unit tag and the release SHA is the verified commit. | `canonicalWorkflowDispatchInputs`, `BuildReleaseDispatchRequest` | dispatch request/client tests and workflow contract tests |
| INV-14 | Dispatch persists `request-started` before HTTP. A 2xx response is accepted; 400/401/403/404/422/429 are rejected; transport errors, redirects, 5xx, and unexpected outcomes are unknown. | `GitHubActionsDispatcher.Dispatch`, `classifyGitHubActionsDispatchResponse` | dispatcher/client response, timeout, redirect, and outbound-call journal-observation tests |
| INV-15 | A terminal dispatch journal (`request-started`, `accepted`, `rejected`, or `unknown`) prevents automatic redispatch. Unknown results are never treated as safe retries. | `DispatchJournalStore.Prepare`, `GitHubActionsDispatcher.Dispatch` | terminal-journal/state-transition tests, active-runner rejected/unknown tests, and resume no-retry tests |
| INV-16 | An ambiguous pending commit/tag push blocks resume. Resume also refuses to infer completion from `dispatch-journal-prepared` or `commit-pushed`. | `HandleResume`, `resumeJournal` | direct pending commit/tag push tests, no-inference tests, and successful `commit-created`/`tag-created`/`tag-pushed` continuation tests |
| INV-17 | Resume uses one existing unresolved journal for the selected remote and unit, never calculates a new version, and blocks when zero or multiple journals match. | `FindUnresolved`, `HandleResume`, `executionContextFromJournal` | all four `resume_test.go` tests |
| INV-18 | A handoff-ready execution journal is considered resolved and is excluded by `FindUnresolved`; a new release command therefore plans from updated V2 state rather than reopening the completed transaction. | `FindUnresolved` and active runner state update | happy-path runner and explicit completed-journal exclusion tests; a subsequent active release remains uncharacterized |
| INV-19 | V2 release dry-run does not resolve a token, fetch, write state/manifests/journals, run an executor, commit, tag, push, dispatch, publish, or invoke rollback. It still validates the executor config file and reads planned file content/hashes. | `HandleRelease`, `ValidateRequirementsForContext`, `v2DryRunPlanResponse` | `dry_run_test.go`, materializer tests, coordinator dry-run test |
| INV-20 | V1 dry-run does not fetch or rewrite V1 config and reports the calculated version. V1 real release still performs legacy preflight and executor behavior. | `Service.GetNewVersion`, `VersionGuardWithOptions` | `TestDryRunDoesNotFetchOrWriteConfigAndShowsNextVersion` |
| INV-21 | V1 rollback is reachable only after `Service.Run` enters its mutating phase, and `ToolBase.RevertGitRelease` does nothing when no mutation is recorded. Once mutation is recorded, legacy destructive cleanup remains possible. | `Service.Run`, `GitReleaseState.hasMutatingStep`, `RevertGitRelease` | `TestRevertGitReleaseWithoutMutatingStepIsNoop`; full rollback characterization is missing |
| INV-22 | V2 code does not perform destructive automatic rollback after commit/tag/push uncertainty. Local snapshots are restored only before the unsafe boundary; later failures preserve evidence. | active runner, `GitReleaseCoordinator`, `ReleaseTransaction.fail` | source assertions plus active-runner commit, tag, commit-push, and tag-push failure tests; state/materialization/store-write boundaries still lack active-runner seams |
| INV-23 | Execution and dispatch journals contain release facts and hashes, not file bytes or tokens. Dispatch errors are capped and redact the exact token. | journal models, `sanitizeDispatchText`, store permissions | journal/dispatcher tests and sentinel assertions across runner errors, command responses, and both journals; captured-log coverage is still missing |
| INV-24 | Public response status/error schemas and error codes are command contracts, but they are currently constructed in multiple packages. Unexpected handler errors become fatal top-level `EXECUTION_ERROR`. | `plugin.Response`, `main.main`, handler response helpers | focused V2 release/resume status, code, metadata, renderer, and ordered-item contracts; other public commands remain incomplete |
| INV-25 | Root V1 migration writes a content-hashed journal before V2 files, archives byte-identical V1 content, validates the final V2 repository, and removes the journal only after success. | `migrate.executePlan`, `archiveV1`, `validateFinal` | all migration and interruption recovery tests |
| INV-26 | V2 local non-dry-run execution is blocked. GitHub Actions owns build and publish after accepted handoff; local release tools are not invoked by that V2 path. | `HandleRelease`, `GitHubActionsReleaseRunner.Run` | V2 block tests and runner fake dispatch tests |

## Architecture strengths

- V1 and V2 load into one `ReleaseRepository`/`ReleaseUnit` read model without erasing the legacy source.
- V2 JSON loading is strict and validates unit IDs, paths, workflow confinement, tag namespace overlap, plugin metadata, and config/state consistency.
- Version and tag planning is mostly pure and typed.
- Known release files make the V2 commit allowlist explicit and verifiable.
- State and materialization writes use atomic single-file replacement and exact snapshots.
- V2 Git coordination has a replaceable command runner and strong real-repository characterization.
- Execution and dispatch journal states are typed, monotonic, and persisted outside the worktree.
- Dispatch target parsing, redirect refusal, response classification, and token redaction are conservative and well tested.
- V2 failure policy preserves evidence after unsafe operations instead of destructive rollback.
- Manifest, routes, docs, workflows, V2 self-release state, and plugin index scripts have cross-file contract tests.

## Concrete hotspots and mixed abstraction levels

### `GitHubActionsReleaseRunner.Run`

This function owns token policy, plan construction, Git preflight, repository target validation, unresolved-journal policy, journal lifecycle, file transactions, Git mutations, dispatch request construction, pushes, HTTP dispatch, recovery result construction, and logging. High-level use-case decisions alternate with low-level store calls and Git operations. The ordering is safety-critical but is represented by one long procedural function rather than named use-case steps behind an injected dependency set.

### `HandleRelease`

This handler parses flags, loads repository state, selects units, branches between V1 and V2, builds execution context, validates requirements, invokes production execution, constructs dry-run plans, and maps multiple response schemas/error codes. Command-boundary work and business orchestration are interleaved.

### `HandleResume` and `resumeJournal`

Resume duplicates runner responsibilities for tag creation, dispatch-journal preparation, push ordering, dispatch, and handoff confirmation. Boolean parameters `loadOnly` and `pushed` select materially different safety behavior in `prepareDispatchJournalForResume` and `dispatchRequestForResume`. The persisted state machine is typed, but its continuation policy is spread across nested conditionals and in-memory state assignments.

### Parallel transaction paths

`ReleaseTransaction`, `GitReleaseCoordinator.Coordinate`, and the active `GitHubActionsReleaseRunner.Run` overlap in planning, state/materialization, Git, and recovery concepts. `ReleaseTransaction.Execute` is deliberately blocked, while its private preparation logic is tested. `Coordinate` is functional but bypassed by the active runner, which calls coordinator methods directly to interleave journal transitions. These paths create ambiguity about the production orchestration boundary.

### Init and configuration persistence

`pkg/init/handler.go` contains command metadata, untyped flag parsing, domain defaults/validation, state construction, two-file persistence, logging, next-step guidance, and response creation. Each file write is atomic, but the config/state pair is not a transaction: a second-write failure can leave a partial target.

### Plugin manifest duplication

V2 config carries validated `plugin.manifest`, and `ReleaseUnit` exposes it, but `plugin_manifest_materializer.go` separately hard-codes unit-to-path mappings. A new valid plugin unit can enter the plugin index yet will not be materialized by release execution without a Go change.

### Response and error duplication

Every command package creates timestamps, metadata, table rows, and error structures. `history` and `contributors` duplicate the same helpers. `initErrorResponse` always labels its command as `init`, including errors returned from `unit-add`. `plugin-index` often returns Go errors, unlike handlers that return structured error responses. Stable codes and item ordering are therefore distributed rather than centrally classified.

### Multiple side-effect adapters

Git appears through `pkg/git`, `GitReleaseCoordinator`, direct subprocesses in `ReleaseTransaction`, direct subprocesses in migration, and direct subprocesses in tools. Tokens appear through both `pkg/config.GetPAT` and the V2 resolver interface. Time is partly injected and partly direct. These inconsistencies limit use-case tests and make failure behavior differ by flow.

## Test structure and current seams

The suite is package-local and predominantly uses temporary real files and real temporary Git repositories. This gives strong integration confidence for Git and persistence behavior but also couples many tests to process cwd and installed Git.

Existing replaceable seams include:

- `gitCommandRunner` inside `GitReleaseCoordinator` and journal stores.
- `GitHubActionsWorkflowDispatchClient` and injected HTTP transport.
- `GitHubActionsDispatchTokenResolver`.
- injected clock functions in journal stores and dispatcher.
- `VersionMaterializer`.
- `transactionExecutor` in the inactive `ReleaseTransaction` preparation path.
- package variables `refreshVersionTags` and `latestVersionTag` for V1 version-guard tests.

Important missing seams include:

- an application-level dependency set for active release and resume;
- filesystem/config/state repositories with failure injection; the active runner constructs materialization/state transactions and execution/dispatch stores internally, so write and post-effect confirmation failures cannot be injected safely;
- injectable runner/dispatcher dependencies at `HandleRelease` and `HandleResume`; fresh successful resume dispatch cannot be command-tested without real HTTP, while accepted-journal reuse is testable without a request;
- one Git port used by all Release Plugin flows;
- an injected response clock;
- subprocess runners for V1 tools;
- an HTTP client for V1 GitHub rollback;
- a command decoder that rejects wrong flag types instead of silently applying defaults.

## Missing characterization coverage, prioritized

1. Remaining active V2 runner failure matrix: execution/dispatch journal writes, post-effect confirmation writes, materialization/state writes, snapshot restoration, and staging boundaries that require the Stage 3 dependency seams.
2. Remaining resume matrix: every persisted phase/pending-action combination, the current expected-tag-already-present edge case, and fresh accepted dispatch through an injected command-level dispatcher.
3. Stable command contracts outside V2 release/resume: exact error code/message/details, response metadata command, renderer hint, and deterministic item order for every other public command.
4. Init/unit-add paired-file failure behavior and exact preservation of pre-existing config/state.
5. V1 executor order and rollback characterization with fake subprocess/network adapters before any extraction.
6. Arbitrary plugin-unit manifest materialization from validated V2 metadata.
7. Secret non-disclosure in captured logs and remaining unreachable filesystem/journal failure paths.
8. Completed release behavior after exclusion: subsequent active version planning from the committed V2 state.
9. Migration injected failure at every unsafe filesystem boundary and unknown/corrupt journal stage handling.
10. History, contributors, validate, and plugin-index Git/filesystem error mapping and stable output ordering.

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

The smallest useful target is not a generic architecture package tree. It is a command boundary plus typed use cases whose ordered unsafe steps depend on narrow Git, filesystem/state, journal, dispatch, token, and clock interfaces, while reusing the existing canonical release models.
