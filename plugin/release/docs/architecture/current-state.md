# Release Plugin Current Architecture

## Purpose and audit basis

This document describes the Release Plugin as it exists after the July 2026 code-quality refactor. It is the detailed behavioral and data-contract reference rather than a target design.

The concise dependency view, responsibility map, lifecycle review, and terminology glossary are maintained in [post-refactor-review.md](post-refactor-review.md). Changed-code controls are in [maintainability-policy.md](maintainability-policy.md), preserved surfaces are summarized in [compatibility-notes.md](compatibility-notes.md), and future safety, compatibility, developer-experience, and feature decisions remain in [architecture-evolution.md](architecture-evolution.md).

The V1 compatibility policy support decision and retired-path cleanup removal record for retained V1 compatibility surfaces are recorded in [v1-compatibility-policy.md](v1-compatibility-policy.md). That register is the authoritative source for Keep, Deprecate, Defer, Removed, and future-removal decisions.

The audit follows the current command routes in `plugin/release/main.go`, every production package under `plugin/release/internal` and `plugin/release/pkg`, the tests under `plugin/release`, the plugin manifest, the repository V2 release files, and the release workflows. Existing repository-wide release documentation was used only as supporting context where the source and tests confirmed it.

The active production scope is `plugin/release`. Shared contracts inspected for integration context include `pkg/plugin/types.go`, `pkg/errors/plugin_errors.go`, and `pkg/config/env.go`.

## Runtime topology

The plugin is a stdin/stdout JSON executable:

1. `main.main` decodes one `plugin.Request` from stdin.
2. It sets global plugin metadata and verbose logging.
3. `workspace.ResolveRepositoryRoot` resolves the release root once at the CLI boundary without changing process cwd; the read-only `doctor`, `units`, and `pipeline` source-inspection commands use `ResolveInspectionRepositoryRoot` so invalid sources can become structured results.
4. `handleRequestAt` routes the command through explicit-root handlers where supported.
5. The handler normally returns a `plugin.Response`; an unexpected Go error is converted by `main` to fatal `EXECUTION_ERROR` output.
6. `main` JSON-encodes the response to stdout.

The retained `workspace.ChangeToProjectRoot` function remains a compatibility helper for direct callers that still rely on process-wide cwd mutation. Production command routing no longer calls it. The migration command keeps its legacy Git-root discovery facade for CLI compatibility while also exposing an explicit-root handler for embedders.

The public command contract is duplicated between `manifest.json` and the switch in `main.go`. `manifest_test.go` characterizes their agreement and also checks the repository command documentation.

## Package and responsibility map

| Area | Current responsibility | Important symbols | Notes |
| --- | --- | --- | --- |
| `main` | Plugin protocol entry, explicit root resolution, command routing, fatal error fallback | `main`, `handleRequestAt` | Uses a command switch rather than a command registry. Production routing passes a resolved root instead of changing cwd. |
| `internal/releasetool` | Canonical release-tool identity, configuration candidates, and shared V1 behavior | `Identity`, `V1BehaviorFor`, `ConfigCandidates` | Shared facts apply across real release-tool integrations and contain no tool-specific command model. |
| `internal/releasetool/goreleaser` | GoReleaser config parsing, invocation classification, and artifact contracts | `ParseConfig`, `Invocation`, `ClassifyArguments`, `VerifyArtifactContract` | Pure GoReleaser format and command facts; no filesystem, HTTP, journal, or presentation dependency. |
| `internal/releasetool/jreleaser` | Canonical JReleaser config model, local load/save codec, rewrite, and version facts | `LoadConfigAt`, `SaveConfigAt`, `RewriteProjectVersion` | Intentionally reads and writes only the selected local JReleaser config; no HTTP, journal, Doctor, or presentation policy. |
| `internal/releasetool/releaseit` | Canonical release-it config model, local load/save codec, and defaults | `LoadConfigAt`, `SaveConfigAt`, `InitDefaultConfig` | Intentionally reads and writes only the selected local release-it config; no HTTP, journal, Doctor, or presentation policy. |
| `internal/releaseworkflow` | Canonical workflow input, repository-target, and ordered consumer-operation facts | `DispatchInputDefinition`, `CanonicalDispatchInputContract`, `GitHubRepositoryTarget`, `ConsumerWorkflowFacts`, `InspectConsumerWorkflowDocument` | Contains no HTTP, journal, token, execution, or presentation policy. |
| `internal/githubdispatch` | One bounded workflow-dispatch POST transport and response sanitization | `Client`, `Client.Post` | Contains no retry, journal, token-resolution, or lifecycle policy. |
| `internal/releasesource` | Tolerant local V1/V2 source inspection | `Snapshot`, `Read` | Read-only and independent of root Release. |
| `internal/legacyrequirements` | Shared source-format V1 token and configuration-file requirements | `Validate`, `ValidateForInspection` | Lifecycle callers retain progress logs; deterministic Validate inspection uses the same checks without query narration. Active execution-context requirements remain a distinct unit-root contract. |
| `pkg/workspace` | Select V2 Git root or legacy nearest-V1 root, expose a typed repository root, and retain cwd switching compatibility | `RepositoryRoot`, `ResolveRepositoryRoot`, `ValidateRepositoryRoot`, `ResolveProjectRoot`, `ChangeToProjectRoot` | `ChangeToProjectRoot` is retained for compatibility; production command routing no longer depends on `os.Chdir`. |
| `pkg/config` | V1/V2 disk models, strict loading, validation, normalization, unit and tag selection, atomic file writes, and canonical crash-recoverable V2 pair persistence | `ReleaseRepository`, `ReleaseUnit`, `LoadReleaseRepository`, `LoadReleaseRepositoryForInspection`, `V1ValidateForInspection`, `ValidateV2`, `ResolveReleaseUnit`, `TagSpec`, `AtomicWriteFile`, `V2ReleasePairPersister` | `ReleaseRepository` is the shared normalized model. Inspection entry points apply identical validation without lifecycle narration. Init and migration reuse one V2 config/state writer and one pair-recovery evidence protocol. V1 remains a compatibility source. |
| `pkg/init` | Typed init/unit-add command boundaries, focused initialization use cases, pure unit/pair construction, and explicit file policy | `HandleInit`, `HandleUnitAdd`, `initializeV2RepositoryUseCase`, `addV2ReleaseUnitUseCase` | Handlers parse, invoke one use case, and map a typed result/failure; validated pairs are passed to the shared config persister. |
| `pkg/migrate` | Typed command presentation, source discovery, pure target planning/recovery policy, ordered failure-aware execution, journaling, and root V1-to-V2 migration | `HandleMigrate`, `migrationUseCase`, `migrationPlan`, `migrationPlanExecution`, `ResolvePlan`, `Run` | Uses focused filesystem operations and a worktree migration journal distinct from release journals. |
| `pkg/validate` | Typed validation request/result boundary, focused V1/V2 validation query, concise summary, and describe/show response projection | `HandleValidate`, `validationQueryUseCase`, `mapValidationQueryResponse` | V1 validation retains its requirements adapter and `GITHUB_TOKEN` dependency; V2 config validation is token-independent and read-only. `--show` retains mode-sensitive JSON while global describe changes only human visibility. |
| `pkg/history` | Typed history query, format-specific read-only Git capabilities, and response mapping | `HandleHistory`, `historyQueryUseCase`, `historyGitReader` | V1 deliberately retains non-erroring tag/count queries; V2 uses exact `TagSpec` matches and structured Git failures. |
| `pkg/contributors` | Typed contributor query, repository/unit selection, focused shortlog capabilities, and response mapping | `HandleContributors`, `contributorsQueryUseCase`, `contributorsGitReader` | V1 repository-wide and V2 path-filtered reads share one command-owned read port without mutation capabilities. |
| `pkg/evidence` | Read-only redacted inspection and guarded completed-evidence archival | `evidenceQueryUseCase`, `evidenceArchiveUseCase`, `ResolveReleaseEvidenceLocations` | Queries receive canonical Git-common-dir paths without receiving execution, dispatch, or V1 compensation stores; archival owns its separate narrow mutation boundary. |
| `pkg/pluginindex` | Typed command modes, deterministic discovery/validation/order, pure JSON output building, and atomic requested-path persistence | `HandlePluginIndex`, `pluginIndexQueryUseCase`, `jsonPluginIndexOutputBuilder`, `atomicPluginIndexOutputPersister` | Check/render/persist retain their established outputs; all command failures remain Go errors that become top-level `EXECUTION_ERROR`. |
| `pkg/git` | Legacy Git queries and the underlying compatibility operations used by focused V1 adapters | `IsClean`, `LatestTag`, `UnitTagsInHistory`, `Current`, `Contributors`, `ContributorsForPaths` | Direct process details remain below release-owned V1 ports; active V1 application code does not import retired raw retired-path cleanup helpers. |
| `pkg/release` planning | Version bump, execution context, delivery/capability descriptions, materialization plan | `PlanUnitVersionBump`, `BuildReleaseExecutionContext`, `ResolveDelivery`, `ResolveExecutorCapabilities`, `ResolveVersionMaterializer` | `github-actions` is the supported V2 delivery mode; `local` is retained only for V1 compatibility and invalid V2 reporting. |
| `pkg/release` plan inspection | Token-free read-only local release-plan inspection for one selected source and unit | `HandlePlanAt`, `releasePlanInspectionUseCase`, `ReleasePlanInspection`, `planV2ReleaseFacts` | Reuses canonical V1/V2 planning facts, maps responses only at the command boundary, and does not inspect journals, remotes, tokens, or recovery evidence. |
| `pkg/release` pipeline composition | Authoritative read-only execution/dispatch journal discovery, exact identity correlation inputs, local Git evidence, recovery assessment, resume eligibility, dispatch retry safety, and Doctor-fact adaptation | `HandlePipelineAt`, `inspectPipelineRuntime`, `inspectPipelineVerification`, `AssessReleaseExecutionRecovery`, `resolveResumeRecovery`, `resolveResumeDispatch` | Maps immutable runtime and verification snapshots to `internal/pipelineinspection` and owns no new transition or mutation behavior. Default is offline/token-free; explicit remote verification delegates to `internal/doctor` and owns no HTTP client or token resolver. It never fetches or reads remote Git refs. |
| `internal/doctor` | Default-offline Release V2 config/state and GitHub Actions readiness, focused local contract evidence, explicit bounded GitHub read verification, and a narrow neutral-fact snapshot API | internal inspection use case, readers, diagnostics, response mapping, `InspectLocalVerification`, `InspectRemoteVerification`; root `HandleDoctorAt` facade | Uses typed facts and deterministic aggregation. Both Doctor and Pipeline reuse one fact owner and one GitHub client; remote mode injects exact GitHub GET reads only. |
| `internal/unitoverview` | Strictly local Release V2 config/state inventory with current version, tag shape, metadata, alignment, and concise issues | internal inspection use case and response mapping; root `HandleUnitsAt` facade | Has no workflow parser, Git/network/token/store/writer/planner capability. |
| `internal/pipelineinspection` | Release V2 configured-stage, runtime, and verification read-model projection and response presentation for one selected unit | `HandlePipelineRuntimeVerificationAt`, immutable `LifecycleStage`, `RuntimeSnapshot`, and `VerificationSnapshot` data, internal typed result and mapper; root `HandlePipelineAt` composition | Reads the V2 source pair and selected confined workflow, then projects supplied authoritative journal/Git/recovery and neutral Doctor facts. It has no Doctor import, Git/HTTP client, token resolver, journal store, recovery/resume/retry implementation, writer/executor capability, and does not import root Release. |
| `internal/contextvalidation` | Focused release-context inspection, ordered local checks, Git evidence, actionable failure and describe projections | internal validation use case; root `HandleReleaseContextValidationAt` facade | Independently available version/tag contradictions are both retained for presentation while the stable primary error and exit policy remain. It does not import root Release or own release execution. |
| `internal/workflowinit` | Typed V2 workflow source/selection, canonical rendering, create/unchanged/conflict planning, and narrow atomic creation | internal command use case; root `HandleGitHubWorkflowInitAt` facade | GitHub-Actions-only create semantics; no token/network/Git capability or implicit update. |
| `pkg/release` V1 | Typed V1 intent/planning/preview/execution/failures, focused requirements/preflight/materialization/Git/rollback adapters, and explicit executor selection | `V1ReleaseIntent`, `PlanV1Release`, `v1ReleasePreviewUseCase`, `v1ReleaseExecutionUseCase`, `V1Executor` | Production uses a fixed executor catalog. `Service`, `Preflight`, `Tool`, `ToolBase`, and `Register/Get` are bounded compatibility facades. |
| `pkg/release` V2 GitHub Actions | Typed command boundary, active release use case, named journaled operations, production facade, and typed progress reporting | `releaseCommandHandler`, `releaseStartOperation`, `githubActionsReleaseUseCase.Run`, `GitHubActionsReleaseRunner.Run`, `ReleaseProgress` | The facade composes one coordinator, one typed token boundary, one clock, and typed progress/diagnostic adapters; the use case owns the visible safety order and delegates each mutation to a focused operation. |
| `pkg/release` V2 Git | Preflight, targeted staging, exact commit verification, tag creation, ordered pushes, dispatch verification, and recovery tag inspection | `GitReleaseCoordinator`, `githubActionsReleaseGitAdapter`, `gitReleaseDispatchVerifier`, `resumeGitAdapter` | Active release/resume share one coordinator instance through consumer-owned capabilities; the former one-call `Coordinate` convenience path was removed in retired-path cleanup. |
| `pkg/release` state/files | Plan and apply version files; update and restore V2 state | `MaterializationTransaction`, `StateTransaction`, `KnownReleaseFiles` | Snapshots support bounded local restore before commit uncertainty. |
| `pkg/release` execution journal | Durable intended-release identity, monotonic phases, pending actions, and execution-specific persistence | `ReleaseExecutionJournal`, `ReleaseExecutionJournalStore` | Store-specific validation/mutations use the shared fixed journal location and secure-write mechanics below the Git common directory. |
| `pkg/release` dispatch | Immutable workflow request, dispatch-specific persistence/classification, typed token, GitHub target, and compatibility facade over the focused transport | `ReleaseDispatchRequest`, `DispatchJournalStore`, `GitHubActionsDispatchToken`, `GitHubActionsDispatcher`, `GitHubActionsDispatchClient` | Explicit accepted/rejected/unknown outcomes; HTTP is owned by `internal/githubdispatch`, while lifecycle and token resolution remain outside it. |
| `pkg/release` recovery | Typed command boundary, read-only assessment, pure continuation policy, and reuse of active named operations | `resumeCommandHandler`, `resumeReleaseUseCase`, `AssessReleaseExecutionRecovery`, `resolveResumeRecovery` | Recovery receives focused Git evidence and reuses active tag/dispatch/push/handoff capabilities without a second orchestration path. |
| `pkg/release/tool/*` | GoReleaser, JReleaser, and release-it V1 executor orchestration plus executor-owned system adapters | `NewV1Executor`, `Run`, `CompensationState`, compatibility `Rollback` | Each executor consumes narrow Git/process/file/environment/token/clock capabilities, exposes typed effect evidence to the active application, and retains its characterized direct rollback delegate for compatibility callers. |

## Command-to-flow map

### `init`

- Entry: `main.main` -> `handleRequestAt` -> `init.HandleInitAt`. `init.HandleInit` remains a compatibility facade that resolves the root from `plugin.Request.Context.WorkingDir` or current cwd.
- Request parsing: `parseInitCommandRequest` is the only init path that reads the untyped flag map and produces `initCommandRequest`; wrong raw types retain the prior zero-value/default behavior.
- Application boundary: `initializeV2RepositoryUseCase.Execute` applies the pure V1/V2/force policy, constructs one normal or plugin unit, creates a complete config/state pair, validates it, and passes it to the pair writer.
- Domain ownership: `unit_constructor.go` owns defaults and normal/plugin construction; `policy.go` owns side-effect-free file-presence decisions; `repository.go` owns complete pair creation and repository validation.
- Side effects: `config.V2ReleasePairPersister` snapshots both targets, persists `.neko/release.pair-recovery.json`, creates and writes both temporary files, then records pending/confirmed evidence around config and state replacement. Returned replace failures trigger restoration of both snapshots; next-process recovery uses only the durable evidence and observed files.
- State mutations: replaces both V2 files when permitted; `--force` never overwrites V1.
- Output: `response_mapper.go` constructs the text-oriented success response or stable structured failure from a typed result/failure.
- Error behavior: stable codes include `CONFIG_CONFLICT`, `V1_CONFIG_EXISTS`, `CONFIG_EXISTS`, `INVALID_FLAGS`, `VALIDATION_ERROR`, and `SAVE_ERROR`.
- Existing tests: handler characterization plus focused parser, constructor, policy, mutation, use-case, response-boundary, temp-create/write/replace, rollback, restoration-failure, exact byte/mode, and cleanup tests.

### `unit-add`

- Entry: `main.main` -> `handleRequestAt` -> `init.HandleUnitAddAt`. `init.HandleUnitAdd` remains a compatibility facade that resolves the root from `plugin.Request.Context.WorkingDir` or current cwd.
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
- Explicit-root embedding: `migrate.HandleMigrateAt` composes a fixed migration root without changing process cwd. The CLI route keeps `HandleMigrate` so migration's existing Git-root discovery and nested-V1 refusal behavior stay unchanged.
- Discovery and planning: the root resolver and filesystem plan resolver classify new migration, recovery, already-complete, and conflict states. `constructMigrationPlan` builds and validates the complete target pair without filesystem access; pure policy functions select one typed planning intention and the target/source operations required by disk evidence. `ResolvePlan` remains a compatibility facade.
- Execution order: start or read the journal; persist the target pair when needed through `config.V2ReleasePairPersister`; confirm target persistence; verify exact target bytes plus strict V2 loading/validation; archive the V1 source when needed; confirm the archive; verify the byte-identical backup; remove the journal.
- Recovery: the immutable plan selects `persistMigrationTarget` or `retainMigrationTarget` and `archiveMigrationSource` or `retainArchivedMigrationSource`. Recovery skips effects already proven by the journal and filesystem evidence instead of replaying a generic transition machine. If pair-recovery evidence exists with a migration journal, the target persistence step lets the shared pair persister restore or close it before rewriting the intended pair; pair-recovery evidence without a migration journal is refused as ambiguous.
- State: `migrationJournalStage` validates the compatible serialized values `prepared`, `config-written`, `state-written`, and `v1-archived`. Empty and unknown values are rejected at load time. The schema version, field names, paths, hashes, strings, and journal mode `0644` are unchanged.
- Dry-run: returns the exact ordered response rows and planned config/state JSON without creating `.neko`, writing a journal or targets, or archiving V1.
- Failure behavior: planning, journal, target persistence, target verification, source cleanup, source verification, and restoration are typed internal failure classes while the public `MIGRATION_FAILED`/nil-Go-error contract remains stable. Incomplete pair restoration or an invalid only remaining backup explicitly requires manual recovery.
- Tests: characterization preserves the public envelope, metadata, data keys, row order, JSON, flag defaults, recovery actions, source bytes/mode, and unrelated files. Focused unit/integration tests inject every execution boundary, prove stop order and recoverable disk evidence, validate typed journal transitions, and enforce the planner/policy/execution boundaries.

### `patch`, `minor`, and `major`

- Entry: `main.main` constructs the three V1 executors and calls `release.HandleReleaseWithV1ExecutorsAt` with the resolved root -> `releaseCommandHandler` with a typed `release.Type`. `HandleRelease` and `HandleReleaseWithV1Executors` remain compatibility facades that resolve the root before delegating to their explicit-root counterparts.
- Parsing: `ParseReleaseCommandRequest` is the only release-start code that reads the untyped plugin flag map and produces `ReleaseCommandRequest`. Missing or wrongly typed flags preserve the existing zero-value defaults.
- Application boundary: the handler invokes `releaseCommandStarter.Start` exactly once. `releaseStartOperation` loads the canonical repository once and `releaseApplicationPathSelector` selects exactly one V1 or V2 application from `ReleaseRepository.SourceFormat`; the selected application does not reselect.
- Branch: V1 receives a typed intent and distinct preview or execution use case. V2 alone builds `ReleaseExecutionContext`. Active V1 does not call `Service`, fatal `Preflight`, the mixed execution-context builder, or the mutable registry.
- Response: application code returns a sealed `ReleaseCommandOutcome` or typed `CommandFailure`; `MapReleaseCommandOutcome` and `MapCommandFailure` construct the stable response from an explicit handler-supplied timestamp.
- Tests: source selection, pure V1 planning, preview immutability, execution order/stopping, response/fatal compatibility, executor commands/ownership, materialization, token/environment/clock boundaries, rollback, V2 Git coordination, and active GitHub Actions behavior are distributed through `pkg/release/*_test.go` and `pkg/release/tool/*`.

#### V2 dry-run

- Orchestration: `releaseStartOperation` -> `startV2Release` -> `planV2Release`, retaining `BuildReleaseExecutionContext` -> requirements validation -> materialization/known-file/dispatch planning order.
- Decisions: calculate next SemVer and tag; resolve delivery and executor capabilities; plan materialization; calculate known release files; build a dispatch summary.
- Side effects: reads config/state, executor config, manifests, and file hashes; emits typed progress through the configured reporter and returns a timestamped response. It does not resolve a token or write journals/files/refs/remotes.
- Tests: all `TestHandleRelease*DryRun*` cases in `dry_run_test.go`, plus materializer and coordinator dry-run tests.
- Missing characterization: a single dependency-spy test proving that every network, clock-independent mutation, Git mutation, and journal store remains unused across each executor/delivery combination.

#### Release plan inspection

- Entry: `main.main` -> `handleRequestAt` -> `release.HandlePlanAt` -> `releasePlanCommandHandler` -> `releasePlanInspectionUseCase`.
- Request parsing: `ParsePlanCommandRequest` reads only `--change` and `--unit` from the untyped flag map. `--change` is required and maps to the same release type values as `patch`, `minor`, and `major`.
- Source selection: the use case loads the canonical repository once and calls `selectReleaseApplicationPath` once. V1 and V2 planning do not reselect the source.
- V2 planning: resolves one unit, builds `BuildV2ReleaseExecutionContext` with read-only planning semantics, and reuses `planV2ReleaseFacts` for requirements validation, `BuildReleasePlan`, materialization planning/validation, `KnownReleaseFiles`, and GitHub Actions dispatch-summary facts. The command response is mapped from `ReleasePlanInspection`, not from dry-run text.
- V1 planning: resolves the virtual `default` unit and uses `PlanV1Release` for a local planning subset. The result explicitly limits V1 known-release-file and latest-tag-evidence semantics instead of running V1 release logic.
- Side effects: reads config/state, executor config, manifest/version files, and local file hashes needed by canonical planning. It does not read tokens, construct token resolvers, open remotes, read execution or dispatch journals, inspect recovery evidence, create directories, write files, mutate Git, dispatch workflows, publish, or run executors.
- Output: `MapReleasePlanInspection` preserves the established machine-readable `data.items` rows and complete typed limitations. Its transport-only `presentation.Properties` projection renders selected source/unit, current version, requested change, next version, tag, executor, delivery, workflow, working directory, unit root, planned materialized files, known release files, readiness, blockers, individually titled limitations, and local-only status. Core alone selects bounded two-column or vertical property layout from the actual writer width; presentation metadata remains absent from public JSON.
- Tests: command parser/handler/response tests, V1/V2 inspection tests, local-blocker tests, explicit-root tests, no-mutation and secret absence tests, manifest/docs/route tests, and architecture guards protecting canonical planning reuse and the absence of mutation-capable dependencies.

#### GitHub Actions workflow scaffolding

- Entry: `main.main` -> `handleRequestAt` ->
  `release.HandleGitHubWorkflowInitAt` ->
  `githubWorkflowScaffoldCommandHandler`.
- Parsing: only optional `--unit`, `--path`, and `--dry-run` values cross the
  raw flag boundary. Dry-run selects a typed preview intent; execution selects
  create intent. No provider, force, managed, or arbitrary command mode exists.
- Source and selection: the narrow source reader accepts only a structurally
  valid Release V2 config/state pair with no V1 conflict or unresolved pair
  evidence. Target resolution selects one unique configured path by default,
  one unit path through `--unit`, or one exact configured path through
  `--path`. Shared paths remain one scope; multiple distinct paths are
  ambiguous until exactly selected.
- Contract: `workflowDispatchInputDefinition` owns the exact ordered input
  names and descriptions used by request construction, dispatch filtering,
  CI validation output, and `githubActionsReleaseWorkflowSpec`. One focused
  template renders deterministic workflow contract version `1`. The Golden
  Path documentation snippet is tested byte-for-byte against the public
  renderer.
- Plan: `githubWorkflowGenerationPlanner` receives source and renderer reads,
  no writer. It validates YAML and classifies missing content as create,
  byte-identical content as unchanged, and any other content as conflict.
- Mutation: only `githubWorkflowScaffoldCreateUseCase` receives the narrow
  `githubWorkflowOutputCreator`. The filesystem creator revalidates physical
  parent containment, writes a target-local candidate, syncs it, and publishes
  it atomically without clobbering a target that appears after planning. New
  directories use `0755`; the workflow uses `0644`.
- Output: normal human responses use ordered properties; preview uses
  transport-only preformatted text for summary plus exact YAML. Public JSON
  retains typed facts and includes generated content only for preview.
  Expected conflicts and request/source/path errors are nil-Go-error command
  responses with exit code `1`.
- Side effects: preview is read-only. Create writes only the selected missing
  workflow. Neither path receives a Git mutator, token resolver, network or
  dispatch client, journal/evidence writer, state persister, release runner, or
  executor.
- Ownership: existing manual workflows and customized generated workflows are
  never updated. Builds, tests, artifacts, signing, publication, credentials,
  GitHub Release creation, release notes, and deployment remain consumer-owned.
- Tests: command/manifest/output contracts, exact Golden Path rendering,
  selection ambiguity, V1/V2 source failures, deterministic YAML, all preview
  classifications, idempotency, no second write, differing-file preservation,
  symlink and path safety, explicit-root isolation, atomic race protection,
  secret absence, and static capability guards.

#### Release V2 integration doctor

- Entry: `main.main` resolves `workspace.ResolveInspectionRepositoryRoot` only
  for the read-only `doctor` and `units` source-inspection commands. Doctor then
  routes through `HandleDoctorAt`, the typed command handler,
  and one `integrationDoctorInspectionUseCase`. The handler alone parses the
  optional `--verify-remote` boolean. An absent flag preserves the offline
  request; a wrong raw type becomes `INVALID_DOCTOR_REQUEST`.
- Root boundary: inspection root resolution selects the enclosing Git root
  without requiring V2 config and state to be mutually valid. All other
  commands retain strict `ResolveRepositoryRoot`; missing, partial, V1-only,
  mixed, malformed, recovery-blocked, and structurally invalid sources can
  therefore become typed source-inspection findings instead of top-level root
  errors.
- Source inspection: the filesystem source reader checks only local V1/V2
  presence, strict V2 JSON, and the existing read-only pair-recovery readiness
  marker. The source inspector reuses canonical config/state structural
  validation and normalization. It returns no repository when source facts are
  unsafe or invalid.
- Scope: default inspection sorts every unit and deduplicates workflow paths.
  `--unit` selects one unit fact but retains every configured unit sharing the
  selected workflow so shared-workflow conflicts cannot be hidden.
- Workflow read: the filesystem reader reuses canonical workflow path and
  physical containment checks, reads one regular file, recognizes canonical
  generated bytes, and parses YAML nodes without evaluating expressions or
  running workflow commands.
- Workflow inspection: focused functions own dispatch inputs/triggers,
  permissions, concurrency, checkout, installation ordering and pins,
  context-validator arguments/output wiring, validator step identity, and the
  consumer extension point. Additional focused inspectors verify supported
  consumer operations and GoReleaser fields, generic origin/installer/registry
  identity, CLI and Release Plugin artifacts, secret-reference classification
  and permission/output safety, and GitHub release/plugin-index target identity
  and ordering. Optional extra inputs and unrelated verification triggers are
  not rejected blindly. Unsupported shapes remain limited; no build-system
  heuristic, full shell parser, general GoReleaser schema, or remote fact is
  invented.
- Permission inspection: one focused Doctor file parses the supported GitHub
  permission scalar/mapping forms, applies workflow inheritance and explicit
  job replacement, and accepts a job write only when a same-job direct
  predicate supports that scope. The closed predicates recognize non-snapshot
  GoReleaser release, direct GitHub CLI release creation/upload, and explicit
  GitHub Container Registry pushes. Workflow writes, `write-all`, unsupported
  forms, OIDC, unrelated scopes, and unsupported job writes remain
  `PERMISSIONS_BROAD`; missing explicit coverage remains
  `PERMISSIONS_IMPLICIT`. The inspection does not evaluate expressions, parse
  shell programs, infer success from names/secret presence, or prove remote
  publication. The publication inspector reads only the two repository-confined
  plugin-index scripts and known literal arguments.
- Explicit remote inspection: only a request with `VerifyRemote` invokes the
  injected `integrationDoctorRemoteInspector`. The focused GitHub reader owns
  exact repository, default-branch workflow content, workflow metadata,
  recognized variable, referenced custom-secret name, Actions policy, exact
  release/tag/asset, and exact durable-run reads. Production targets
  `https://api.github.com`; every possible request is `GET`, redirects are
  refused, timeout is 12 seconds, response bodies are capped at 1 MiB, and
  there is no automatic retry or list/latest/fuzzy discovery. Doctor owns no
  durable run ID and therefore makes no workflow-run request.
- Authentication boundary: repository identity is anonymous-first. A missing
  or unauthorized anonymous identity may resolve `GITHUB_TOKEN` once for one
  authenticated identity lookup, which disambiguates a private repository.
  Public workflow/release/tag reads remain anonymous; protected Actions-policy,
  recognized-variable, and custom-secret-name reads reuse the same lazily
  resolved token. Secret values, token text, authorization headers, raw private
  response bodies, and arbitrary variables or secret names do not enter the
  result.
- Verification model: one small typed fact contains subject, category, state,
  evidence, repository-relative references, optional unit/workflow, and optional
  limitation class. States are `verified`, `missing`, `mismatch`, `unsupported`,
  `not_verifiable`, `not_attempted`, `unavailable`, `unauthorized`, and
  `rate_limited`; limitation classes are `remote`, `runtime`, and
  `mutation_required`. An additive remote summary records `not_requested`,
  `complete`, `partial`, or `unavailable` plus verified/unresolved/failed
  counts. This is not an Evidence store, diagnostics engine, registry, state
  machine, or provider abstraction.
- Boundary mapping: offline remote-workflow, repository-variable, and
  exact-dispatch limitations are emitted only when their local predicates
  exist. Successful explicit reads replace the first two, narrow installation
  and publication limitations, and append focused remote facts. Exact dispatch
  authorization remains `mutation_required`; runtime execution, secret-value
  validity, future target acceptance, and future consumer behavior remain
  honest limitations. The former unconditional limitation loop does not
  return. Consumer, installation, credential, and publication limitations come
  from their corresponding focused inspector after a verified or explicitly
  unsupported local shape.
- Result: diagnostics use the closed severities `error`, `warning`,
  `recommendation`, and `not_verifiable`. Stable ordering is severity, scope,
  unit, workflow, code, and message. Any error yields `not_ready`; otherwise a
  warning yields `ready_with_warnings`; recommendations and not-verifiable
  facts alone yield `ready`. Definite remote missing, mismatch, disabled, or
  invalid facts are errors. Unauthorized, rate-limited, unavailable,
  unsupported, and ambiguous private-resource outcomes remain unresolved
  evidence and cannot erase successful independent facts.
- Output: `mapIntegrationDoctorResult` alone constructs the plugin response.
  JSON contains `readiness`, severity/verified counts, ordered units, ordered
  workflows, additive ordered verifications, additive `remote_verification`,
  and ordered diagnostics.
  Transport-only presentation metadata composes a
  titled readiness/count summary, a titled severity/code index with optional
  target and scope, and complete ordered property records headed by severity and
  code. The mapper assigns closed semantic roles to readiness, positive counts,
  severity, and code while leaving ordinary diagnostic fields neutral. Core
  alone maps roles to interactive terminal styles, fits optional columns,
  selects table or vertical records, and wraps long workflow, message, and
  remediation values at normal, narrow, and unknown widths. A non-empty
  `NO_COLOR` and every non-terminal destination disable ANSI. Public JSON, raw
  JSON, and GitHub output exclude the presentation projection and remain
  ANSI-free. `not_ready` requests exit code `1`; the two ready states request
  exit code `0`.
- Side effects: the default path never invokes the optional remote inspector or
  token resolver. Explicit remote mode injects only the focused GitHub reader
  and token resolver; no config/state/workflow writer, Git command or mutator,
  dispatcher, journal store, Evidence writer, release runner, executor, or
  publication adapter reaches the use case. Neither mode reads journals,
  executes a process, dispatches, uploads, publishes, repairs, or mutates local
  or remote state. Doctor does not call or own the unit overview or pipeline
  inspection; each is an independent command capability.
- Tests: strict source variants, structural workflow checks, canonical and
  custom workflow classification, shared scope, readiness/exit policy,
  deterministic diagnostics, token and secret absence, exact file metadata
  preservation, explicit-root/cwd isolation, responsive human output, JSON
  isolation, default no-network/no-token behavior, exact fake-server endpoints,
  anonymous/private authentication, partial failures, rate limits, repository,
  workflow, variable, secret-name, Actions-policy, release/tag/asset and exact
  run classifications, output redaction, and static no-mutation/no-framework/
  exact-identity guards.

#### Release V2 unit overview

- Entry: `main.main` routes the flat `units` command through the explicit
  inspection root to `HandleUnitsAt`, one typed command handler, and one
  `unitOverviewInspectionUseCase`. `HandleUnits` provides the embedding facade
  that resolves a root without changing process cwd. There are no
  command-specific flags or unit selector.
- Source boundary: `filesystemLocalV2SourceReader` is the shared narrow local
  V2 presence/strict-load/recovery-readiness owner used by both the overview
  and Doctor source facade. The overview does not call Doctor orchestration,
  workflow readers, or workflow inspectors. Expected missing, malformed,
  unsupported, V1-only, mixed, and recovery-blocked source states become a
  typed `source_issue` and exit `1`, not a Go error.
- Unit derivation: a structurally valid pair is normalized through canonical
  config/state owners. A parseable schema-2 pair with unit-level parity or
  validation failures is projected from the union of config and state IDs so
  no incomplete or invalid unit disappears. Current canonical versions come
  only from state through `CanonicalReleaseVersion`; raw invalid state remains
  `configured_version`. Tag prefix validation and `tag_shape`/
  `configured_tag` construction reuse `TagSpec` and never inspect Git or plan a
  future version. Executor, delivery, and workflow-path facts reuse the
  canonical V2 structure validator.
- Classification: unit alignment is the closed derived set `aligned`,
  `config_only`, `state_only`, or `invalid`. Unit issue severity is `error` or
  `warning`. Repository status is `valid`, `has_issues`, or `source_invalid`;
  only `valid` exits `0`. Summary counts total, aligned, incomplete, invalid,
  distinct workflow paths, and whether both strict source files are usable.
- Ordering: rows sort by unit ID; issues sort by severity, unit ID, code, and
  message; distinct repository-relative workflow paths sort lexically.
- Output: `mapUnitOverviewResult` alone constructs the plugin response. Machine
  data contains `status`, `summary`, `units`, `workflow_paths`, and optional
  `source_issue`. Unit rows expose only stable inventory facts and preserve
  empty issues as arrays. Transport-only `presentation.Table` metadata declares Unit,
  Version, and Status essential, followed by optional name, tag, executor,
  delivery, workflow, working-directory, and concise issue columns. Core owns
  width detection, fitting, truncation/wrapping, and vertical fallback.
- Safety: the use case receives only a narrow source reader. It has no
  config/state/workflow writer, YAML parser, Doctor workflow inspector, Git
  reader or mutator, network/GitHub client, token resolver, build-system
  reader, planner, release executor, journal/Evidence store, dispatcher, or cwd
  and environment mutation capability. Config, state, workflow content, file
  mode, and mtime remain unchanged.
- Tests: manifest/help/routing contracts; strict source, unit metadata,
  version/tag, alignment, summary, exit, and deterministic-order scenarios;
  normal/narrow/unknown/wide presentation and JSON isolation; root/nested/
  explicit/two-repository isolation; no-mutation/no-token runtime checks;
  Doctor/workflow-scaffold regressions; and static no-capability,
  no-workflow-parser, no-Doctor, no-state-machine, and no-framework guards.

#### Release V2 pipeline inspection

- Entry: `main.main` routes `pipeline` through the explicit inspection root to
  `pkg/release.HandlePipelineAt`. The root facade supplies a fresh static
  projection of lifecycle metadata plus immutable, read-only runtime and
  verification snapshots; `internal/pipelineinspection` projects that supplied
  data and does not import `pkg/release` or `internal/doctor`.
- Source and unit policy: the capability reads the local V2 source pair,
  recovery-readiness marker, and selected repository-confined workflow. It
  reuses canonical V2 validation, unit resolution, executor identity, tag, and
  dispatch-input facts. Multi-unit repositories require `--unit`; V1 and
  unsupported source/executor/delivery/workflow contracts are typed failures.
- Stage ownership: `release_lifecycle_facts.go` describes the exact direct-call
  root operation order without functions or handlers. `releaseworkflow`
  derives ordered consumer operations from local YAML and reuses the focused
  `goreleaser.ClassifyArguments` fact; Doctor consumes the same workflow facts
  and focused GoReleaser classifier.
  Production execution remains straight-line calls in
  `githubActionsReleaseUseCase.Run` and never iterates over descriptors. The
  runtime projection reuses the authoritative execution recovery and resume /
  retry-safety decisions; it owns no transition table or continuation policy.
- Runtime composition: `pkg/release` reads the repository's execution and
  dispatch journal stores, validates canonical identities, selects without
  timestamp or recency inference, correlates dispatches only through the
  recorded dispatch-journal identity, and observes bounded local Git facts.
  Local Git observation covers branch, HEAD, index/worktree state, expected
  commit existence/content/reachability, expected tag existence/target, and
  known-file recovery evidence. It never fetches or reads remote refs.
- Verification composition: default Pipeline calls
  `doctor.InspectLocalVerification`, which constructs no client or token
  resolver. Explicit `--verify-remote` calls
  `doctor.InspectRemoteVerification`, which reuses Doctor's single bounded
  GET-only GitHub client and lazy token resolver. Root maps neutral states and
  limitation classes; the internal projector owns stable IDs, summary counts,
  JSON, and presentation. Neither path consumes Doctor diagnostics, readiness,
  remediation, response mapping, or presentation.
- Output: top-level lifecycle status is one of `ready`, `active`, `resumable`, `blocked`,
  `uncertain`, `rejected`, `completed`, or `invalid`. `invalid` is structured
  inspection data with exit code `1`; all other successful projections use
  exit code `0`. `completed` means an exact accepted dispatch handoff, not
  publication completion. JSON schema version `1` retains the configured
  fields and additively includes execution, dispatch, local-Git, recovery,
  manual-intervention, and verification sections with deterministic non-null
  arrays and no presentation metadata. Verification failure, partial, or
  unresolved status never changes lifecycle status, resume eligibility, exit
  policy, or stage completion.
- Presentation: the internal response mapper alone declares an always-visible
  `Summary`, conditional actionable `Findings`, and describe-only `Verification
  Facts`, `Configured Pipeline`, safe execution/dispatch/local-Git/recovery
  evidence, and complete numbered `Limitations`. Findings are a presentation-
  only projection of existing verification, lifecycle, journal, Git, recovery,
  and manual-intervention facts; they do not derive a second diagnostic or
  lifecycle policy. Verification makes check/status/scope essential and
  subject/evidence optional. Pipeline makes stage/runtime/owner essential and
  admits number, location, mutation, then evidence as width permits.
  Presentation-only groups preserve global stage order: canonical plugin-index
  stage IDs take the registry group; consumer/release-tool owners or non-root
  workflow sources take the consumer group; remote locations/mutations and
  handoff confirmation take the handoff group; the remainder is local
  preparation. Empty groups are omitted, and fully unobserved runtime data
  receives one concise note. Humanized labels do not alter machine
  vocabularies. Core's existing renderer owns describe-only filtering,
  chained-table composition, group headings, notes, section titles, terminal
  width, wrapping, deterministic vertical fallback, and semantic TTY-only
  color. Global describe adds structured sections and metadata; global verbose
  independently adds captured logs. These transport declarations are excluded
  from public and raw JSON; Pipeline production code imports neither renderer
  nor terminal code.
- Safety: root composition has read-only journal and bounded local Git query
  capability but no Git mutator, direct token resolver, HTTP/dispatch client,
  writer, release-tool runner, cwd mutation, or duplicated
  retry/resume/transition policy. Only the explicit remote branch may delegate
  to Doctor's existing GET/token boundary. The internal inspector receives data
  only. Core consumes describe locally and does not serialize it into the
  plugin request, so describe cannot select that remote branch. Neither layer
  calculates a future version or invokes another command handler.
- Tests: source/unit/request contracts, exact root and real-workflow order,
  conditional plugin stages, immutable metadata ownership, all execution and
  dispatch phases, malformed/conflicting/unlinked evidence, exact identity
  correlation, local Git mismatch and recovery evidence, authoritative resume
  and retry-safety outcomes, deterministic no-recency selection, read-only
  isolation, local and Loopback-only remote Doctor facts, exact additive JSON
  schema and vocabularies, stable IDs, responsive presentation, and static
  no-mutation/single-GET-client/no-framework guards.

#### V2 GitHub Actions execution

- Facade: `GitHubActionsReleaseRunner.Run` validates the execution request, reports request facts through `ReleaseProgress`, composes production operations, and invokes `githubActionsReleaseUseCase.Run`.
- Orchestration: `githubActionsReleaseUseCase.Run` exposes the ordered story: token resolution; materialization planning; Git/unresolved-journal preflight; execution-journal preparation; materialization; state write; targeted stage; commit; tag; dispatch-journal preparation; commit push; tag push; workflow dispatch; accepted-handoff confirmation.
- State: each named mutation operation persists its exact pending marker before the side effect and its confirmed phase afterward. Execution and dispatch stores retain separate contracts while sharing common-dir/canonical-write mechanics; all active adapters receive the facade's coordinator and clock.
- Output: `GitHubActionsReleaseResult` is a typed command outcome mapped only in `release_response.go`.
- Existing tests: `github_actions_release_runner_test.go` preserves real-repository happy paths and durable recovery evidence. `github_actions_release_use_case_test.go` proves the full named order, stopping at every replaceable dependency, cleanup order, rejected-dispatch behavior, and captured-log token absence. `github_actions_release_operations_test.go` injects pending-write, side-effect, and confirmation failures around all eight journaled mutations.
- Git verification: `BuildReleaseDispatchRequest` receives `releaseDispatchGitVerifier`; production injects `gitReleaseDispatchVerifier` backed by the facade's existing coordinator. The builder no longer constructs Git infrastructure.

#### V2 release progress reporting

- Boundary: active V2 start/planning, runner/use-case, named operations, Git progress, and dispatch progress report `ReleaseProgressEvent` values through `ReleaseProgress`. The interface is synchronous and has no error return.
- Terminal ownership: `release_progress_terminal.go` is the only active V2 progress renderer. It wraps the established plugin stderr logger, preserves lifecycle order and verbose suppression through `log.Verbose`, and projects repository/config/state/snapshot/journal paths to repository-relative or safe artifact labels.
- Diagnostics ownership: verbose Git command diagnostics are separate from release progress through `gitReleaseDiagnostics` and `git_release_diagnostics_terminal.go`.
- Suppression: absent reporters are converted to an explicit no-op; application and operation code does not branch on JSON, quiet, interactive, or renderer configuration.
- Machine output: final `plugin.Response` construction remains in command response mappers and stdout remains JSON-only. Progress uses stderr and is captured as execution logs by the plugin protocol.
- Safety: progress events contain no token/header/environment/body fields or arbitrary maps; call sites pass sanitized remote display values. Reporting cannot choose release policy, retry behavior, journal state, Git/network effects, command success, or response schema.
- Presentation: `release_lifecycle_presentation.go` maps the sealed V1/V2 release outcomes to one shared human contract. Dry-run keeps ordered principal operations and primary materialized files visible; describe adds source/configuration, declared files, execution evidence, Git/handoff, and limitations. The mapper neither reads infrastructure nor changes the established machine rows.

#### V2 local execution

- V2 local delivery is deliberately unsupported for executable V2 releases. Config validation rejects `delivery: "local"` and missing delivery values instead of normalizing them to local execution.
- `releaseStartOperation` maps invalid V2 configs to `CONFIG_INVALID`; the active public V2 release path never reaches executor composition for local delivery.
- `ReleaseTransaction.Execute` is a deprecated compatibility wrapper that rejects local execution directly.
- The former private `ReleaseTransaction` preparation, rollback, and executor-invocation scaffold was removed after the local-delivery decision.

#### V1 execution

- Planning: `pureV1ReleasePlanner` receives a typed `V1ReleasePlanningRequest` and returns current/latest/next versions, tag, commit metadata, executor, canonical V1 config file, and materialized-file ownership without infrastructure access or mutation. Preview uses local tag evidence only and returns a typed result without token, file, Git, executor, or rollback effects.
- Execution: `v1ReleaseExecutionUseCase` first opens the repository's V1 compensation store and continues or refuses any unresolved attempt before planning a new release. It then performs local preview planning, requirements, typed preflight, refreshed execution planning, fixed executor resolution, durable evidence creation, verified V1 config materialization, one executor invocation, and typed completion. The readable order is explicit; no step list, workflow pipeline, state machine, or boolean V1/V2 mode is involved.
- Preconditions: the characterized executor file and `GITHUB_TOKEN`; clean worktree; attached `main` or `master`; configured upstream; branch not reported behind. Fatal preflight is represented as `V1ReleaseFailure` and mapped at the command boundary to the established fatal JSON/exit behavior.
- State mutation and ownership: `.release.neko.json` is written before execution through `V1SaveConfigAt`. GoReleaser owns the release commit, lightweight `v` tag, commit/tag pushes, warning-only snapshot, and publication. JReleaser first synchronizes `jreleaser.yml`, then owns the commit/push and warning-only dry-run while JReleaser owns tag/publication. release-it owns its commit/tag/push/publication internally and package-manager selection still prefers `bun.lock`, then `package-lock.json`, then npm.
- Recovery: the active application stores V1-only evidence at `<git-common-dir>/neko/release/v1-compensation/current.json` before config mutation and executor invocation. Recovery selects exactly one named operation at a time in the fixed order: restore the exact original config bytes; delete the GitHub Release; delete the local tag; delete the remote tag; revert and push a pushed release commit, or reset an unpushed release commit; then clean untracked release files. Every operation persists `pending` before its side effect and confirms only after verification. The first failure stops later effects. A later V1 release invocation automatically continues supported repeatable local operations, but refuses pending or uncertain remote operations, a pending/non-repeatable revert, and an uncertain executor outcome with evidence-path guidance. This remains compensation, not transactional rollback, and unsupported evidence requires manual recovery.
- Executor evidence: GoReleaser failures are automatically compensable only when its captured state proves local effects; push/publication ambiguity is manual. JReleaser is automatically compensable only before commit/push/publication ambiguity. A release-it process failure is always externally uncertain because release-it owns commit, tag, push, and publication internally.
- Tests: fake subprocess, Git, token, environment, clock, file, config, GitHub-client, evidence-store, and reporter capabilities protect the full order/failure and interruption matrix. System compatibility tests use only local fake executables, temporary repositories, and local HTTP transports.

### `resume`

- Entry: `main.main` -> `release.HandleResumeAt` -> `resumeCommandHandler` -> `resumeReleaseUseCase`. `HandleResume` remains a compatibility facade that resolves the root before delegating.
- Parsing and response: `ParseResumeCommandRequest` creates the typed request; the handler invokes `releaseResumer.Resume` once and maps a sealed `ResumeCommandOutcome` or `CommandFailure` with its injected response clock.
- Presentation: `resume_presentation.go` projects only facts retained by `ResumeCommandOutcome`. Default output remains a recovery decision with exact status, pending action, eligibility, retry safety, guidance, and dry-run mutation boundary. Describe adds safe journal, local-Git assessment, recovery, continuation/handoff, and limitation sections without reading a journal or resolving policy in the mapper.
- Progress: `resume_progress.go` and command-owned call sites narrate discovery, exact selection, local assessment, policy resolution, config/Git validation, the selected continuation, and completion/refusal through verbose-only stderr logs. Completion messages occur only after the owned operation returns successfully; dry-run stops after the no-continuation assessment message.
- Discovery: `locateResumableExecution` requires V2, resolves one unit and its current upstream remote, and finds exactly one unresolved execution journal matching remote URL and unit.
- Assessment: `AssessReleaseExecutionRecovery` verifies journal structure, known-file hashes, and local tag evidence through an injected tag inspector without remote access. Non-dry continuation separately reconstructs the journal-bound execution context and rejects current-config drift.
- Dry-run: returns that assessment without requiring `GITHUB_TOKEN` or modifying the journal/worktree.
- Policy and selection: `resolveResumeRecovery` is a pure resolver from journal plus local assessment to one typed operation or refusal. `resumeReleaseOperationSelector` maps the supported classification once to `resumeFromCommitCreatedOperation`, `resumeFromTagCreatedOperation`, `resumeFromTagPushedOperation`, or `returnCompletedReleaseHandoffOperation`; the selected operation does not switch on the execution phase again.
- Continuation: commit-created preserves the characterized already-present-tag block and otherwise reuses GitHub Actions release use-case extraction tag creation before delegating to tag-created. Tag-created reuses GitHub Actions release use-case extraction dispatch preparation, commit push, and tag push in the active order. Tag-pushed uses a separate pure dispatch decision and reuses GitHub Actions release use-case extraction dispatch and handoff confirmation.
- Dispatch and token boundary: prepared or missing dispatch state permits one fresh dispatch; accepted dispatch is reused without redispatch or token resolution; request-started, rejected, and unknown remain no-retry refusals. Token lookup occurs only inside the fresh-dispatch operation after earlier continuation effects have succeeded and returns the same typed token used by active release.
- Completed handoff: an explicit dependency-free operation returns the existing handoff result, while production discovery continues to exclude handoff-ready journals as resolved.
- Restrictions: it will not calculate a new version, continue before a confirmed commit, prove ambiguous push completion, or redispatch a terminal dispatch journal.
- Existing tests: dry-run read-only behavior, ordered assessment output, no/exactly-one journal selection, corrupt/conflicted/config-drift handling, every execution-state/pending-action policy combination, operation selection, supported continuation from `commit-created`, `tag-created`, and `tag-pushed`, expected-tag-already-present blocking, completed-handoff behavior, ambiguous-push blocking, no push-state inference, accepted dispatch reuse without a token, terminal dispatch no-retry behavior, and focused continuation failure boundaries.
- Retained limitations: there is no remote-state probe, automatic retry, journal repair, or continuation from pre-commit/ambiguous push evidence. Production discovery deliberately excludes completed journals.
- Compatibility: presentation declarations are transport-only. `ResumeCommandOutcome`, journal selection, assessment status, eligibility, retry safety, named-operation selection, machine rows/error envelopes, and exit behavior are unchanged.

### `history`

- Entry: `history.HandleHistoryAt` parses `historyQueryRequest`, invokes `historyQueryUseCase.Query` once, and maps the typed result/failure with a response clock. `HandleHistory` remains a compatibility facade that resolves the root before delegating.
- Read ownership: `historyRepositoryReader` loads the canonical repository; the command-owned `historyGitReader` exposes only legacy tags/counts and V2 unit-tags/path-counts. No mutating Git capability enters the use case.
- Root ownership: the query use case and Git adapter receive the same explicit root. Legacy `pkg/git` helper functions remain cwd facades while the handler uses the root-aware `At` variants.
- V1: uses all local tags and direct commit counts. The legacy adapter intentionally retains the established empty-success/zero-count behavior when its package functions suppress Git errors.
- V2: exact unit-prefixed tags reachable from `HEAD`, ordered by history, with counts constrained to unit pathspecs. Unit-tag and count errors map to `GIT_HISTORY_FAILED` with a nil handler Go error.
- Tests: parser, one-invocation handler, fixed mapper, focused stop-point, same-commit ordering, empty result, real-Git path filtering, and worktree/index/ref immutability tests supplement `git/tag_test.go`.

### `contributors`

- Entry: `contributors.HandleContributorsAt` parses `contributorsQueryRequest`, invokes `contributorsQueryUseCase.Query` once, and maps its typed result/failure. `HandleContributors` remains a compatibility facade that resolves the root before delegating.
- Read ownership: a command-owned repository reader and contributor-only Git port expose repository-wide or selected-path shortlog reads; the use case preserves the adapter's deterministic order and clones returned entries.
- Root ownership: repository and Git reads use the same explicit root. Legacy `pkg/git` helper functions remain cwd facades while the handler uses the root-aware `At` variants.
- V1: repository-wide `git shortlog`; V2: selected-unit pathspecs. Git failures remain structured `GIT_CONTRIBUTORS_FAILED` responses with nil handler Go errors.
- Tests: typed defaults, handler invocation/mapping, V1/V2 capability selection, dependency stopping, empty results, selected paths, response contracts, and worktree/index/HEAD immutability are explicit.

### `validate`

- Entry: `validate.HandleValidateAt` parses `validationQueryRequest`, invokes `validationQueryUseCase.Query` once, and maps typed validation facts/failures outside application code. `HandleValidate` remains a compatibility facade that resolves the root before delegating.
- Read ownership: `validationRepositoryReader` returns canonical repository data plus the presence fact needed to preserve `CONFIG_NOT_FOUND` versus `CONFIG_INVALID`; `legacyRequirementsValidator` is the only V1 environment/filesystem requirements capability.
- Root ownership: V1 and V2 validation read from the explicit root. The V1 requirements adapter uses `release.ValidateRequirementsAt`; `release.ValidateRequirements` remains the current-cwd facade.
- V2: strict load/validation already occurred in `LoadReleaseRepository`; optional `--show` returns cloned normalized unit facts for pure response formatting. It remains token-independent and non-mutating.
- V1: resolves the legacy unit, revalidates the model, then invokes the established token/executor requirements adapter in that order. The token requirement remains characterized compatibility behavior, not a recommendation.
- Tests: typed defaults, fixed response mapping, load/presence classification, V1 validation/requirements stop order, V2 dependency isolation and no-alias behavior, stable rows/errors, and exact config/state immutability supplement config/workflow tests.

### `ci-validate-context`

- Entry: `release.HandleReleaseContextValidationAt` parses the four dispatched strings with the explicit `workspace.RepositoryRoot`, invokes `releaseContextValidationUseCase.Validate` once, and maps `ValidatedReleaseContext` only at the command boundary. Expected validation failures remain structured nil-Go-error responses and opt into a nonzero Core CLI exit.
- Source ownership: `filesystemReleaseContextSourceReader` recognizes V1 only by file presence and never loads it. It requires one complete strict V2 pair, rejects V1/V2 conflict and unresolved pair-recovery evidence, validates config/state alignment, normalizes once, and resolves the explicitly selected unit once through the canonical resolver.
- Policy reuse: strict SemVer parsing validates the dispatched and authoritative versions; `config.TagSpec` alone formats the canonical tag. The command does not calculate a next version or create release intent.
- Git ownership: the narrow read port exposes object format, object type, HEAD commit, exact tag presence, and peeled tag commit. The adapter executes only explicit-root `git rev-parse`, `git cat-file`, and `git show-ref` reads. It accepts SHA-1 and SHA-256 repositories, matching detached HEAD, and lightweight or annotated tags. It never fetches or mutates.
- Result and output: `ValidatedReleaseContext` contains only validated local unit/config/Git facts. Stable flat machine data, ordered human properties, and ten ordered GitHub output declarations are mapper-owned. Core owns explicit destination selection and injection-safe command-file encoding; presentation metadata is absent from public JSON.
- Isolation: application composition has no token resolver, GitHub/dispatch client, remote Git, persister, materializer, journal/evidence writer, recovery mutator, executor, runner, or `plugin.Response`. Real-Git tests assert exact roots, unchanged cwd/worktree/index/refs/config/state, tag variants, non-commit rejection, and no shell command construction.

### `plugin-index`

- Entry: `pluginindex.HandlePluginIndexAt` parses one typed render/check/persist mode, invokes `generatePluginIndexUseCase.Run` once, and maps a typed result. `HandlePluginIndex` remains a compatibility facade that resolves the root before delegating. `--check` with `--output`, discovery, building, and persistence failures intentionally remain Go errors and therefore top-level `EXECUTION_ERROR` values.
- Discovery and validation: `pluginIndexQueryUseCase` owns config/state/manifest reads through `pluginIndexSourceReader`; pure candidate/completion functions validate state SemVer and manifest identity, duplicate checks retain their prior order, and entries are stably sorted by plugin name. Public `Generate` is the compatibility facade over this query.
- Root ownership: command composition passes the explicit root into `generatePluginIndexUseCase`; public `Generate` retains its existing `GenerateOptions.Root` contract.
- Output building: `jsonPluginIndexOutputBuilder` alone creates complete pretty or compact JSON bytes with the stable schema/order and trailing newline. It chooses no path, reads no files, and constructs no response. `Write` and `WriteWithOptions` remain public compatibility wrappers over those complete bytes.
- Output path ownership: `resolvePluginIndexOutputTarget` is the generated integration artifact path owner. Relative `--output` values must be clean repository-root-relative paths and resolve from the explicit root, not process cwd. Explicit absolute output remains a supported CI/temp-artifact target. Repository-contained targets are checked against protected release config/state/evidence, Git internals, and the plugin manifest inputs in the generated index. Existing target directories and target symlinks are rejected; repository-relative parent symlinks must resolve inside the physical repository root.
- Persistence: output mode passes complete bytes and the canonical target to `atomicPluginIndexOutputPersister` while the response keeps the configured display path. The persister creates requested parent directories as `0755`, overwrites an allowed file target, uses `0644` for a new file, preserves an existing target's mode, writes/fsyncs/closes a target-local temporary file, then renames it and discards any unconsumed temporary file. A returned pre-replace/write/replace failure preserves the prior target; no config, state, manifest, Git, journal, or unrelated file is mutated.
- Modes: check performs discovery only; default render performs discovery then building and returns raw JSON; output performs discovery, building, path validation, and the explicit single-file command effect. There is still no publication action, cancellation source beyond the supplied context, or schema change.
- Tests: query/read stop points, typed entry validation, deterministic output, parser/handler/use-case boundaries, all three modes, output path normalization, traversal and protected-path rejection, symlink behavior, builder/writer failure, creation/replacement/modes, injected create/write/replace failures, original preservation, temporary cleanup, unrelated-file preservation, response compatibility, multiple-repository output isolation, and workflow scripts are explicit.

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

V1 compatibility policy completed the support decision for those surfaces. Deprecated surfaces now point to tested replacements where one exists: explicit `HandleReleaseWithV1Executors` composition, `PlanV1Release`, `BuildV2ReleaseExecutionContext`, explicit V1 config root/path functions, `Run` with `V1ExecutorRequest`, `Rollback`, and `MapCommandFailure`. Deferred surfaces such as fatal `Preflight`, `Tool`, `ToolBase`, and legacy executor `Init` remain unmarked because no exact public replacement exists. Concrete executor `Rollback` methods now own the direct rollback adapter call, while `RevertRelease` is the deprecated direct delegate.

Bounded limitations remain:

- the compatibility registry and version-evidence package variables remain mutable for old callers/tests but are unreachable from production release composition and deprecated where a tested explicit replacement exists;
- direct callers of the legacy `V1ReleaseRollback` compatibility surface retain its characterized in-memory, best-effort behavior; the active V1 application does not use it;
- pending/uncertain remote deletion or push, a pending revert, release-it failure, and executor push/publication ambiguity intentionally require manual recovery rather than remote inference or blind retry;
- the active release invocation is recorded as one pending executor effect, so interruption inside an executor is conservatively classified as manual instead of inferred from remote state;
- the single `current.json` record is retained after completion and may be replaced by a later attempt; inspection, archival, and schema-lifecycle tooling remain deferred to evidence inspection and archival;
- V2 local delivery is rejected for executable V2 releases; the deprecated `ReleaseTransaction` wrapper remains only to fail directly and no longer retains private preparation scaffold;
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

V2 `workingDirectory` validation is lexical and physical: symlinked working directories must resolve inside the repository root. Materialized file targets reject symlinks rather than replacing or following them. `JReleaserMaterializer` owns only `jreleaser.yml` below the validated unit root; plugin manifest materialization owns only the validated plugin manifest path for the selected unit.

### Canonical active V2 adapter ownership

- Release-owned file planning is driven by `ReleaseExecutionContext.Unit` plus `KnownReleaseFiles`; plugin manifest location comes only from validated unit metadata.
- `GitHubActionsReleaseRunner` and resume composition each create one `GitReleaseCoordinator`. Consumer-owned Git adapters expose only preflight/stage/commit/tag/push, dispatch verification, remote lookup, or tag inspection needed at each call site.
- `ReleaseExecutionJournalStore` exclusively owns execution-journal lookup, validation, and intention-revealing mutations. `DispatchJournalStore` exclusively owns dispatch-journal preparation, validation, transition persistence, and terminal-state resolution.
- `releaseJournalFiles` is not a generic store: it owns only the two fixed journal directories, common-dir resolution, canonical JSON bytes, `0700` directory creation, and atomic `0600` replacement used identically by both stores.
- `EnvironmentGitHubActionsDispatchTokenResolver` is the only production V2 environment reader and returns `GitHubActionsDispatchToken`; formatting is redacted, and only dispatch adapters unwrap it for authorization and error sanitization.
- `ReleaseClock` is the active release/resume timestamp capability. One injected clock supplies command responses and the composed V2 execution/dispatch stores and dispatcher; model-level zero-time fallbacks remain compatibility behavior for direct callers.
- Public store, dispatcher, and runner constructors remain compatibility entry points with production defaults. The isolated V1 application/adapters, migration-specific root/filesystem adapters, and inactive `ReleaseTransaction` remain deliberately outside active V2 composition and do not compete with it. The former `GitReleaseCoordinator.Coordinate` convenience method was removed in retired-path cleanup; active code uses focused coordinator methods through release/resume operations.

### Git and delivery results

- `GitReleasePreflight`: branch, remote, upstream branch.
- `GitReleaseResult`: progress and recovery facts for commit/tag/push coordination.
- `GitHubActionsReleaseResult`: user-facing handoff facts.
- `ReleaseDispatchRequest`: immutable exact workflow inputs and identity.
- `GitHubRepositoryTarget`: parsed GitHub.com owner/repository from the selected upstream remote.

### Durable lifecycle phases

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
| Working directory | production `handleRequestAt` receives `workspace.RepositoryRoot`; compatibility `workspace.ChangeToProjectRoot`, `ToolBase.InUnitRoot`, and cwd facades remain | explicit-root isolation tests plus retained cwd compatibility tests | Production command routing no longer mutates cwd. Compatibility facades still expose process-global cwd semantics when callers choose them. |
| Filesystem | shared config pair persistence; focused init, plugin-index, migration, V1 materialization, compensation evidence/config, preflight, and executor config/file boundaries | pair replacement seams; V1 evidence-store/config ports; command-owned file/config ports; temporary directories | V1 uses its canonical single-file writer plus a private `0700` common-dir evidence directory and atomically replaced `0600` record; it intentionally does not share the V2 pair transaction. Inactive paths retain some direct `os.*`. |
| Git | active V2 release/resume use one `GitReleaseCoordinator`; active V1 uses `SystemV1GitWriter`, root-aware named compensation adapters, and a preflight repository port; direct compatibility callers may still use `V1ReleaseRollback`; queries retain command-owned read ports | fake V1 Git/evidence capabilities, coordinator runner, query capabilities, and real temp repositories | V1 destructive compensation uses a fixed V1-only evidence contract and remains intentionally isolated from V2 recovery. |
| Environment/token | V1-owned legacy token/environment ports; V2 `EnvironmentGitHubActionsDispatchTokenResolver` returning `GitHubActionsDispatchToken`; explicit Doctor remote verification lazily reuses that typed read identity | sentinel fake token/environment/process adapters plus Doctor no-token/default and recording-resolver tests | V1 and V2 intentionally retain different token types, variable injection, messages, and behavior. Default Doctor never resolves a token; explicit Doctor resolves at most once and never exposes the value. |
| Network | bounded, root-aware V1 GitHub Release client; injected V2 dispatch transport; package-private explicit Doctor GitHub GET client | fake V1 remover/client, V2 `RoundTripper`, and Doctor local `httptest` servers | The active V1 client has a finite timeout, bounded response reads, explicit repository root, a narrow typed-token boundary, and verified GET/DELETE/not-found behavior. Doctor additionally refuses redirects, limits reads to 1 MiB/12 seconds, performs no automatic retry, and exposes only sanitized result classifications. |
| Time | `ReleaseClock` for release/resume responses, active V2 persistence, and V1 compensation evidence; command-owned query clocks; JReleaser init `v1Clock` | injected fixed clocks and persisted timestamp tests | V1 evidence timestamps support auditability but do not infer completion; direct compatibility model fallbacks remain. |
| External executables | V1-owned Git/executor process and binary-locator adapters; `du` and inactive paths retain direct execution | fake per-executor runners plus local fake processes | Exact V1 command order, environment, outputs, warnings, failures, and ownership are isolated and characterized. |
| Progress and logging | active V2 progress uses `ReleaseProgress` plus terminal/diagnostic adapters; V1 and legacy tooling retain package logging; `main` still sets package-global verbose mode | progress characterization, terminal-adapter tests, architecture source assertions, and output inspection | Active V2 application/operation files no longer import the terminal logger. V1 logging redesign and process-global verbose mode remain outside typed release progress reporting. |
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
| INV-11 | The execution journal records a pending action before each active mutation and confirms the matching phase after success. Phases cannot skip or move backward; once-only identifiers cannot change. | named operations in `release_operation_local_files.go` and `release_operation_workflow.go`, `BeginPending`, `ConfirmPhase` | execution journal phase-transition/store tests plus active-runner commit, tag, and push failure recovery assertions |
| INV-12 | A dispatch journal is prepared after local commit/tag creation and before either push. Its immutable identity includes the final release commit SHA. | active runner order, `BuildReleaseDispatchRequest`, `DispatchJournalStore.Prepare` | dispatch request/journal tests; happy-path runner tests |
| INV-13 | Workflow dispatch inputs are exactly `unit`, `version`, `tag`, and `release_sha`; the ref is the unit tag and the release SHA is the verified commit. | `canonicalWorkflowDispatchInputContract`, `canonicalWorkflowDispatchInputValues`, `canonicalWorkflowDispatchInputs` | dispatch contract/request/client tests and workflow contract tests |
| INV-14 | Dispatch persists `request-started` before HTTP. A 2xx response is accepted; 400/401/403/404/422/429 are rejected; transport errors, redirects, 5xx, and unexpected outcomes are unknown. | `GitHubActionsDispatcher.Dispatch`, `classifyGitHubActionsDispatchResponse` | dispatcher/client response, timeout, redirect, and outbound-call journal-observation tests |
| INV-15 | A terminal dispatch journal (`request-started`, `accepted`, `rejected`, or `unknown`) prevents automatic redispatch. Unknown results are never treated as safe retries. | `DispatchJournalStore.Prepare`, `GitHubActionsDispatcher.Dispatch` | terminal-journal/state-transition tests, active-runner rejected/unknown tests, and resume no-retry tests |
| INV-16 | An ambiguous pending commit/tag push blocks resume. Resume also refuses to infer completion from `dispatch-journal-prepared` or `commit-pushed`. | `resolveResumeRecovery`, `resumeReleaseUseCase` | pure state/pending policy table, direct pending commit/tag push tests, no-inference tests, and successful `commit-created`/`tag-created`/`tag-pushed` continuation tests |
| INV-17 | Resume uses one existing unresolved journal for the selected remote and unit, never calculates a new version, and blocks when zero or multiple journals match. | `locateResumableExecution`, `FindUnresolved`, `reconstructResumeExecutionContext` | resume discovery, dry-run, application-use-case, and command-contract tests |
| INV-18 | A handoff-ready execution journal is considered resolved and is excluded by `FindUnresolved`; a new release command therefore plans from updated V2 state rather than reopening the completed transaction. | `FindUnresolved` and active runner state update | happy-path runner and explicit completed-journal exclusion tests; a subsequent active release remains uncharacterized |
| INV-19 | V2 release dry-run does not resolve a token, fetch, write state/manifests/journals, run an executor, commit, tag, push, dispatch, publish, or invoke rollback. It still validates the executor config file and reads planned file content/hashes. | `releaseStartOperation`, `ValidateRequirementsForContext`, `planV2Release` | `dry_run_test.go`, materializer tests, coordinator dry-run test |
| INV-20 | V1 preview uses local evidence, returns the calculated version, and does not fetch, resolve a token, write config, invoke Git/executor/rollback, or construct a command response. Real execution refreshes tag evidence only after preflight. | `v1ReleasePreviewUseCase`, `v1ReleaseExecutionUseCase`, `v1ReleasePlanningOperation` | V1 planner/preview/execution order tests and compatibility two-pass evidence test |
| INV-21 | Active V1 execution creates strict private evidence before mutation. Each required compensation persists pending intent before its side effect, verifies success before confirmation, and runs in the fixed config/GitHub/local-tag/remote-tag/revert-or-reset/cleanup order. Supported repeatable local pending work may continue; remote, non-repeatable, corrupt, or uncertain evidence fails closed. | `v1ReleaseExecutionUseCase`, `SelectV1CompensationOperation`, `continueV1Compensation`, named V1 compensation operations and store | characterization, schema/store/policy, operation interruption, adapter, GitHub-client, and next-invocation integration tests |
| INV-22 | V2 restores materialized files, State, and known-file staging on every safely classified failure before a commit attempt can start, including failure to persist the pending commit action. Restoration failures are joined to the original cause. Once `create-release-commit` is durably pending and Git commit execution begins, commit/verification uncertainty and every later tag/push/dispatch failure preserve local evidence; no post-commit rollback occurs. | `githubActionsReleaseUseCase.Run`, named local-file operations, `restoreMaterializationAfterReleaseFailure`, `restoreLocalReleaseFilesAfterFailure`, `restoreStagedReleaseFilesAfterFailure`, `MaterializationTransaction.Restore`, `StateTransaction.RestoreSnapshot` | operation cleanup/error-joining tests plus real-repository staging, commit-command, commit-verification, tag, commit-push, and tag-push failure tests asserting files, State, index, Git objects, journals, recovery, and Resume policy |
| INV-23 | Execution and dispatch journals contain release facts and hashes, not file bytes or tokens. The typed dispatch token redacts all string formatting, and dispatch errors are capped and redact the exact secret. | journal models, `GitHubActionsDispatchToken`, `sanitizeDispatchText`, store permissions | typed-token formatting/source tests, journal/dispatcher tests, and sentinel assertions across logs, runner errors, command responses, and both journals |
| INV-24 | Public response status/error schemas and error codes are command contracts, but they are currently constructed in multiple packages. Unexpected handler errors become fatal top-level `EXECUTION_ERROR`. | `plugin.Response`, `main.main`, handler response helpers | focused V2 release/resume status, code, metadata, renderer, and ordered-item contracts; other public commands remain incomplete |
| INV-25 | Root V1 migration writes a content-hashed compatible journal before target persistence, uses the shared crash-recoverable writer for one complete V2 pair, verifies exact target bytes and strict V2 validity before archiving byte-identical V1 content, and removes the journal only after target/source verification. Recovery selects typed operations from journal, pair-recovery evidence, and file evidence. | `migrationPlanExecution.Execute`, `V2ReleasePairPersister`, migration execution adapters and policy | command-contract, journal-stage, operation-order, pair-recovery, boundary-failure, restoration, backup-verification, and interruption recovery tests |
| INV-26 | V2 local non-dry-run execution is blocked. GitHub Actions owns build and publish after accepted handoff; local release tools are not invoked by that V2 path. | `releaseStartOperation`, `GitHubActionsReleaseRunner.Run` | V2 block tests and runner fake dispatch tests |
| INV-27 | Init, unit-add, and migration validate one complete V2 config/state pair before persistence and reuse `config.V2ReleasePairPersister`. The persister creates durable pair-recovery evidence before unsafe replacement, records pending evidence before each target rename, verifies bytes before confirmation, verifies the complete intended pair before closing evidence, and can next-process restore exact prior bytes/modes/existence or close an already-complete intended pair. Ambiguous or corrupt evidence requires manual recovery. | initialization use cases, migration planner/execution, `V2ReleasePairPersister`, `.neko/release.pair-recovery.json` | focused new/update/migration, pair evidence/schema/policy, next-process recovery, temp-create/write, first/second replace, exact restore, restore-failure, cleanup, byte, and mode tests |
| INV-28 | Validate, CI release-context validation, history, contributors, and plugin-index check/render queries receive only command-owned read capabilities and do not mutate release files, Git worktree/index/refs, journals, environment, or plugin state. V1 validate still resolves its token through the requirements read, and legacy history retains suppressed Git failures. | `validationQueryUseCase`, `releaseContextValidationUseCase`, `historyQueryUseCase`, `contributorsQueryUseCase`, `pluginIndexQueryUseCase` | parser/use-case/handler stop-point tests plus config/state/tree and real-Git worktree/index/ref immutability contracts |
| INV-29 | Plugin-index output mode builds the complete stable JSON bytes before resolving one output target. Relative targets are repository-root-relative to the explicit root; absolute targets remain supported for CI/temp artifacts. Repository-contained targets cannot overwrite protected release state/evidence, Git internals, or plugin manifest inputs, and target/parent symlink behavior is explicit. New parents/files use `0755`/`0644`; overwrite is allowed for permitted file targets and preserves an existing target mode; returned write/replace failures preserve the old target and clean temporary files. | `resolvePluginIndexOutputTarget`, `jsonPluginIndexOutputBuilder`, `atomicPluginIndexOutputPersister`, `config.AtomicFileReplacement` | exact pretty/compact schema tests plus path normalization, traversal/protected/symlink tests, multiple-root isolation, creation, replacement, mode, injected write/replace, original-preservation, unrelated-file, and cleanup tests |
| INV-30 | Production selects V1 or V2 once from canonical `SourceFormat`; active V1 uses a fixed executor catalog and active V2 alone builds the V2 execution context. | `releaseApplicationPathSelector`, `HandleReleaseWithV1Executors`, `releaseStartOperation` | source-selector, fixed-catalog, production-composition, and architecture tests |
| INV-31 | V1 patch/minor/major planning is pure and deterministic. The typed plan owns exact next version, `v` tag, release commit metadata, `.release.neko.json`, executor identity, and materialized files without infrastructure dependencies. | `PlanV1Release`, `V1ReleasePlan` | deterministic planner table and infrastructure-free architecture guard |
| INV-32 | GoReleaser, JReleaser, and release-it preserve their distinct command/config/push/publication ownership and warning-only dry-run behavior through replaceable ports. Executor outputs/errors redact the legacy token while preserving underlying causes. | concrete `Run` methods, executor system adapters, `RedactV1ProcessResult` | command-order/failure/ownership tests, injected adapter tests, clock test, and sentinel secret tests |
| INV-33 | Migration reads canonical V1 data but cannot import V1 execution, executor, Git mutation, or rollback internals. V1 executors do not implement the inactive V2-local transaction or inspect source format. | migrate imports, concrete executor orchestration | migration-direction and executor-orchestration architecture tests |
| INV-34 | Dispatched V2 context validation is strictly local and read-only: it requires one valid unblocked V2 pair, exact unit/version/tag/commit/HEAD/tag-target agreement, performs no token/network/mutation/fetch, and maps output only at the command boundary. | `releaseContextValidationUseCase`, `filesystemReleaseContextSourceReader`, `releaseContextGitAdapter`, `MapValidatedReleaseContext` | application/real-Git/command/output tests plus architecture guards |
| INV-35 | Release V2 integration diagnostics are read-only. Default Doctor is offline/token-free and never invokes the optional remote capability. Explicit `--verify-remote` injects only a package-private, exact-identity GitHub GET reader plus lazy typed token resolution at the command boundary; it preserves local/shared-workflow scope, distinguishes definite failure from unresolved access, never reads journals or receives mutation/Git/store/process/dispatch/publication capabilities, and maps stable readiness/facts/remote-summary/diagnostics only at the command boundary. | `ResolveInspectionRepositoryRoot`, `integrationDoctorInspectionUseCase`, focused source/workflow/file/origin readers and inspectors, `integrationDoctorGitHubRemoteInspector`, `integrationDoctorGitHubReadClient`, `mapIntegrationDoctorResult` | command/source/parser/local-inspector/dogfood/safety/presentation/explicit-root tests; fake-server repository/workflow/variable/secret/policy/release/tag/asset/run/auth/rate/partial/redaction tests; architecture, exact-identity, no-mutation, and naming guards |
| INV-36 | Release V2 unit inventory is local and read-only: every config/state unit remains visible in deterministic order, current versions come only from state, canonical version/tag/unit policies are reused, expected source and row findings use structured exit `1`, and no workflow parser, Doctor orchestration, Git/network/token/store/writer/planner capability reaches the use case. | `HandleUnitsAt`, `unitOverviewInspectionUseCase`, `filesystemLocalV2SourceReader`, `mapUnitOverviewResult` | source/unit/tag/exit/presentation/root/isolation/no-mutation tests plus architecture and naming guards |

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
- Validate, CI release-context validation, history, contributors, and plugin-index have typed command boundaries with command-owned read capabilities and deterministic mappers.
- Plugin-index discovery, JSON output construction, and atomic single-file persistence are distinct owners with focused failure seams.
- Manifest, routes, docs, workflows, V2 self-release state, and plugin index scripts have cross-file contract tests.

## Concrete hotspots and mixed abstraction levels

### `releaseStartOperation` and the active release use case

`HandleReleaseWithV1Executors` is the production presentation/composition entry: parse a typed request, invoke one starter, and map one typed outcome/failure with an injected clock. `releaseStartOperation` loads the repository and delegates the one source-format decision to `releaseApplicationPathSelector`. V1 then owns unit resolution and its typed preview/execution use cases; V2 alone builds the V2 execution context. For active V2 GitHub Actions execution, `GitHubActionsReleaseRunner.Run` remains a facade over the readable named-operation use case. `HandleRelease` and `newReleaseStartOperation` retain registry-backed composition only as direct compatibility facades.

### Resume composition

`HandleResume` is a typed presentation boundary over `resumeReleaseUseCase`. Discovery, assessment, context compatibility, pure policy resolution, and one named continuation operation are separate responsibilities. Production composition reuses the active release tag, dispatch-preparation, push, workflow-dispatch, and handoff capabilities with the same coordinator, journal stores, typed token, and clock; it retains conservative no-inference/no-retry policy.

### Parallel transaction paths

`ReleaseTransaction` is no longer a future V2 local production path. It remains as a deprecated compatibility wrapper that rejects local execution directly, without private preparation, rollback, or executor-invocation scaffold. No concrete V1 executor plugs into it, and the former JReleaser V2 source-format bypass was removed. retired-path cleanup removed `GitReleaseCoordinator.Coordinate`; the active use case continues to call focused coordinator methods directly through named operations that interleave journal transitions. This is not an active second orchestration path.

### Init and configuration persistence

`HandleInit` and `HandleUnitAdd` are command boundaries: each parses one distinct typed request, invokes one focused use case, and maps one typed result or failure. Raw flags stop in `command_request.go`; pure normal/plugin unit construction, file-presence policy, and complete pair creation/append are separate. retired-path cleanup removed the private `buildV2InitConfigFromFlags` bridge after tests moved to typed parser/constructor coverage.

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

`evidence` is a read-only evidence inspection and archival query across the explicit evidence families: release-execution journals, dispatch journals, migration journals, V1 compensation evidence, and V2 pair-recovery evidence. It returns typed redacted records and diagnostics instead of raw JSON, keeps corrupt/unsupported/conflicting files visible without dumping their content, and orders records deterministically. Its response mapper preserves complete existing `items`, typed `evidence`, and `diagnostics` data while adding transport-only Core human-table metadata. The Release Plugin owns summary field order and priority: family, state, classification, resume eligibility, and manual recovery are essential; unit, version, tag, pending action, automatic continuation, and lifecycle are optional. Full identity, digest, owner, path, guidance, and timestamps remain detail-only. Core owns width detection, Unicode/ANSI display width, optional-column fitting, table-versus-vertical selection, wrapping, and `wide` mechanics; public JSON excludes the presentation declaration.

`evidence --identity <prefix>` selects a complete redacted detail record only after family and unit filters. Prefixes are 8-64 lowercase hexadecimal characters, are not case-normalized, and must match exactly one typed record; zero and ambiguous matches fail instead of selecting the first record. Detail human output uses the existing property/value response shape, and detail JSON keeps the established three data keys with the complete typed record. The operation receives no writer, archiver, resume capability, or lifecycle mutation. `evidence-archive` remains the only evidence inspection and archival lifecycle operation. It supports `archive-completed` only for completed release-execution, completed V1 compensation, and completed V2 pair-recovery evidence, requires family, an exact full identity, current digest, and explicit confirmation, re-observes the evidence, writes and verifies an exact private archive, then removes the completed source. Dispatch and migration evidence remain inspect-only because accepted dispatches and migration journals can still be needed for handoff audit or owner-specific recovery.

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
- evidence query and archive use cases with explicit family/unit filters, read-only identity-prefix detail selection, and separate exact identity/digest/confirmation archival inputs;
- plugin-index `pluginIndexSourceReader`, query/builder/persister command ports, and persistence-specific directory/stat/atomic-replacement operations;
- `gitCommandRunner` inside the single active `GitReleaseCoordinator` and shared journal common-dir mechanics.
- `GitHubActionsWorkflowDispatchClient` and injected HTTP transport.
- `GitHubActionsDispatchTokenResolver`.
- `ReleaseClock` across response mapping, active journal stores, runner, and dispatcher.
- `VersionMaterializer`.
- `transactionExecutor` in the deprecated `ReleaseTransaction` compatibility wrapper.
- V1 preview/execution plan builders, requirements, preflight repository, config store, fixed executor catalog, reporter, release Git writer, compensation evidence store/policy/named operations, bounded GitHub Release client, per-executor process/config/file/environment/token/clock ports, and shared binary/file/redaction adapters.
- package variables `refreshVersionTags` and `latestVersionTag` for V1 version-guard tests.

Important missing seams include:

- process-global workspace/current-directory compatibility paths and inactive release transaction factories;
- direct compatibility callers can still reach the legacy best-effort `V1ReleaseRollback`; its behavior is not the active V1 recovery protocol;
- registry and version-evidence globals remain mutable only for compatibility tests/direct callers;
- a command-decoding policy for wrong flag types; the command presentation extraction parsers deliberately preserve silent defaults because rejection would be a new public behavior.

## Bounded post-refactor limitations, prioritized

1. Active V1 compensation is interruption-safe for supported local actions, but deliberately requires manual recovery for pending/uncertain remote actions, pending revert, corrupt evidence, and uncertain executor outcomes; direct legacy rollback callers remain best-effort.
2. Pair and migration crash recovery is evidence-driven for supported config/state and archival windows, but it still refuses corrupt, externally edited, unsupported, or owner-ambiguous evidence and does not claim cross-file atomicity.
3. V2 local delivery is unsupported for executable V2 releases; production rejects invalid configs before local executor composition. The former `GitReleaseCoordinator.Coordinate` convenience path was removed in retired-path cleanup.
4. Process-global workspace selection and compatibility current-directory facades still limit parallel in-process command execution.
5. Completed V2 release behavior after journal exclusion and subsequent planning remains less directly characterized than the primary release/recovery matrix.
6. Explicit absolute plugin-index output remains a supported CI/temp-artifact exception; repository-contained outputs follow the protected-path and symlink policy above.

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

Typed release progress reporting resolved the prior bounded presentation deviation: active V2 application and focused operation code now emits typed progress through `ReleaseProgress`, while terminal rendering is isolated in adapter files. This does not mix response construction into application code or create a second orchestrator; it keeps the completed refactor ledger closed and records ongoing architecture decisions in [post-refactor-review.md](post-refactor-review.md) and [architecture-evolution.md](architecture-evolution.md).

- Completed stages: 9 / 9
- Remaining stages: 0
- Release Plugin refactor: completed
- Completed capability records: V1 compensation interruption safety — Make V1 compensation interruption-safe; V2 pair and migration crash recovery — Make pair and migration crash recovery explicit; evidence inspection and archival — Add evidence-safe journal inspection and lifecycle support; V1 compatibility policy — Decide and deprecate V1 compatibility surfaces; retired-path cleanup — Retire superseded and inactive release paths; typed release progress reporting — Isolate release progress reporting; explicit-root composition — Make command roots explicit for embedders; generated-output path policy — Clarify generated-output path policy; release plan inspection — Add read-only release plan inspection; V2 local delivery evaluation — Evaluate V2 local delivery; GitHub Actions workflow scaffolding — Generate an idempotent create-only release workflow; Release V2 unit overview — Add deterministic read-only unit inventory; Release V2 pipeline inspection — Add local configured-stage inspection; Release V2 pipeline runtime inspection — Correlate local journals, local Git evidence, recovery, and resume/retry safety without mutation; Release V2 pipeline verification facts — Reuse local and explicit remote Doctor facts without changing lifecycle status
- Deferred pipeline capability: durable workflow-run and publication-completion inspection (not implemented)

V1 compensation interruption safety, V2 pair and migration crash recovery, evidence inspection and archival, V1 compatibility policy, retired-path cleanup, typed release progress reporting, explicit-root composition, generated-output path policy, release plan inspection, GitHub Actions workflow scaffolding, Release V2 unit overview, configured and runtime Release V2 pipeline inspection, and later architecture decisions are maintained in [architecture-evolution.md](architecture-evolution.md). Capability records are not refactor stages; the historical refactor ledger remains closed.
