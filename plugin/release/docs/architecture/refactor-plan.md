# Release Plugin Behavior-Preserving Refactor Plan

## Intent

This plan starts from the current implementation documented in `current-state.md`. It does not assume an earlier refactor, package layout, branch, or issue description exists.

The plan establishes the smallest boundaries needed to make unsafe release ordering explicit, provide failure-injection seams, reduce duplicated parsing/response mapping, and support later read-only inspection and CI features. It does not prescribe a generic architecture framework or a large package move.

No stage may silently activate V2 local execution, add standalone dispatch/retry, change V1 semantics, or change production release behavior unless a later feature task explicitly requests that behavior.

## Global invariants for every stage

Every stage must preserve the current contracts in `plugin/release/RULES.md`, especially:

- V2 config/state ownership and selected-unit version updates;
- exact version/tag calculation;
- clean-worktree and exact known-file staging rules;
- release commit message and exact contents;
- lightweight tag target;
- commit push before tag push;
- execution pending markers before mutations and phase confirmation afterward;
- dispatch journal preparation before push and request-started before HTTP;
- accepted/rejected/unknown classification;
- no blind retry for uncertain push or dispatch;
- dry-run/recovery read-only behavior;
- stable error and response contracts;
- no secret disclosure;
- no destructive V2 rollback;
- V1 compatibility.

If a stage discovers that current code or tests contradict this plan, current behavior wins. Update the audit and revise the stage before changing code.

## Validation baseline

Use focused tests while iterating. Before every stage commit, run:

```bash
git diff --check
GOCACHE=/private/tmp/neko-cli-go-build go test ./plugin/release/...
GOCACHE=/private/tmp/neko-cli-go-build go test ./...
golangci-lint run --config .golangci.yml ./...
```

The repository Make targets are `make plugin-release-test`, `make test`, `make lint`, and `make verify`. Do not run a formatting target without first checking whether it would modify unrelated user files. Record exact environmental limitations rather than claiming success.

For documentation changes, also verify every referenced path/symbol and every relative Markdown link.

## Target boundaries

The intended boundaries are deliberately small:

1. **Command presentation** decodes flags into typed requests and maps typed outcomes to stable `plugin.Response` values.
2. **Release use cases** own visible operation ordering and policy for start, dry-run, and resume.
3. **Release decisions/models** retain current canonical repository, unit, tag, materialization, known-file, and journal types.
4. **Adapters** isolate Git, config/state/filesystem, materialization persistence, journals, token/environment, clock, executor subprocesses, and GitHub dispatch.

The target does not require one package per layer. Boundaries can begin as types and functions in existing packages, then move only if a later change shows clear value.

## Clean-code completion gate for production refactors

This gate applies to every production refactor stage in this plan and to any future production milestone derived from it. Behavior preservation is necessary but not sufficient. Every milestone must demonstrate that:

- it introduces no new god function or procedural playbook;
- it introduces no generic state machine, workflow interpreter, transition engine, or large phase/status switch;
- it introduces no boolean parameter that selects a workflow;
- every extracted function has one clearly nameable responsibility and one primary reason to change;
- every function operates at one abstraction level;
- infrastructure dependencies are explicit, narrowly scoped, and replaceable where the operation needs a test seam;
- application use cases represent focused user-visible intentions rather than vague services, managers, coordinators, or processors;
- no oversized dependency container or generic step pipeline hides responsibilities or safety order;
- plugin response mapping remains at the command/presentation boundary, outside application and business logic;
- safety-critical operation ordering is readable through focused named calls rather than callbacks, generic step lists, shared mutation, or low-level detail inlining;
- tests prove both preserved behavior and the intended architectural boundary, including that unrelated real infrastructure is not required.

> A refactor milestone is incomplete when behavior is preserved but the resulting code still violates the single-responsibility, abstraction-level, dependency, or control-flow rules.

Moving a large function into several equally mixed functions is not progress under this plan. A stage is also incomplete if extraction only relocates direct infrastructure construction, response mapping, phase branching, or shared mutable workflow state.

## Stage 1: Characterize active command and failure contracts

### Goal

Pin the active V2 release and resume behavior that lower-level tests do not currently prove, before production extraction.

### Current problem

`GitHubActionsReleaseRunner.Run` has happy-path integration tests, while failure semantics are inferred from component tests. `HandleRelease` and `HandleResume` expose many stable codes and ordered response items without one command-contract suite.

### Scope

- Add table-driven command contract tests for V2 patch/minor/major dry-run and active execution failures.
- Add active runner tests for unresolved-journal blocking and accepted/rejected/unknown dispatch results.
- Add resume tests for exact phase/pending-action decisions that can be exercised safely without changing orchestration.
- Assert response item order, metadata command, renderer hint, and secret absence.

Likely files:

- `pkg/release/dry_run_test.go`
- `pkg/release/github_actions_release_runner_test.go`
- `pkg/release/resume_test.go`
- a focused new `pkg/release/command_contract_test.go` if clearer

### Non-goals

- No production refactor.
- No new behavior or error code.
- No network call to GitHub.
- No exhaustive failure injection that requires new adapters; record those cases for Stage 3.

### Behavioral invariants

Preserve all global invariants, with special attention to stable response contracts, token-before-mutation, terminal dispatch behavior, and resume not calculating a new version.

### Tests required before modification

Existing `dry_run_test.go`, runner, journal, dispatcher, and resume suites must pass unchanged. Capture any current response that is surprising instead of correcting it in this stage.

### Implementation steps

1. Inventory every error code and response row emitted by `HandleRelease` and `HandleResume`.
2. Add reusable assertions for ordered response rows and forbidden fake secret values.
3. Add characterization cases without altering production dependencies.
4. Document any still-unreachable failure point in `current-state.md`.

### Acceptance criteria

- The characterized command matrix records every currently reachable V2 release/resume outcome without modifying production Go files.
- Assertions cover response ordering, metadata, error codes, dry-run immutability, terminal dispatch handling, and secret absence.
- Unreachable unsafe boundaries are listed explicitly for Stage 3 rather than represented by weakened tests.

### Clean-code acceptance

- Characterization tests assert externally observable contracts without treating current god-function structure as desirable architecture.
- Test helpers and fixtures have focused responsibilities; no generic workflow harness or shared mutable phase interpreter is introduced.
- The tests make command/presentation, application, and infrastructure seams observable enough for Stages 2 and 3 to prove their boundaries.

### Validation

Run the full baseline. Also run:

```bash
GOCACHE=/private/tmp/neko-cli-go-build go test ./plugin/release/pkg/release -run 'Test.*(CommandContract|Runner|Resume|DryRun)'
```

### Expected commit message

`test(release): characterize v2 release command contracts`

### Risks

Tests may accidentally assert nondeterministic timestamps or temporary paths. Normalize only nondeterministic values; do not weaken semantic assertions.

### Rollback strategy

This is test-only. Revert the commit if the characterization is wrong; no production rollback is needed.

### Dependencies

None. This is the first executable milestone.

### Completion record (2026-07-14)

Stage 1 is complete as a test-only milestone with no production Go changes. The retained and added characterization covers:

- V2 patch/minor/major dry-run planning, selected-unit behavior, repository immutability, token independence, ordered response rows, metadata, and renderer hints;
- request/repository failures for unit selection, malformed or mismatched config/state, V1 resume, local delivery, and dirty worktrees;
- active-runner unresolved-journal blocking, commit/tag creation failures, commit-before-tag push failures, and accepted/rejected/unknown dispatch outcomes;
- execution/dispatch pending markers and confirmed metadata at the currently injectable Git/dispatch boundaries, including `request-started` before the outbound dispatch call;
- resume discovery, assessment ordering, corrupt/conflicting/config-drift handling, supported continuation from `commit-created`, `tag-created`, and `tag-pushed`, completed-journal exclusion, ambiguous-push blocking, no push-state inference, and terminal dispatch no-retry behavior;
- sentinel-token absence from characterized command responses, runner results/errors, execution journals, and dispatch journals.

The active runner still constructs materialization/state transactions and journal stores internally, and the handlers construct their runner/resume dependencies. Consequently, materialization/state/store-write failures, post-side-effect confirmation failures, captured-log redaction, and a fresh successful resume HTTP dispatch remain unreachable without the narrow production seams planned for later stages. These gaps are recorded in `current-state.md`; Stage 1 does not emulate them with weakened tests.

## Stage 2: Establish typed command requests and response mapping

Status: completed on 2026-07-14 by `refactor(release): isolate release command presentation`.

### Goal

Make `patch`, `minor`, `major`, and `resume` handlers presentation boundaries without changing their use-case internals yet.

### Current problem

`HandleRelease` and `HandleResume` mix untyped flag access, repository/use-case decisions, error classification, response timestamps, and table construction. Response helpers are scattered and use direct `time.Now`.

### Scope

- Introduce typed command request values for release start and resume.
- Introduce typed command outcomes/failures at the handler boundary.
- Centralize stable release/resume response mapping and inject a response clock.
- Keep the existing runner and resume functions behind compatibility facades.

Likely files/symbols:

- `pkg/release/handler.go`
- `pkg/release/resume.go`
- `githubActionsReleaseResponse`
- `commandErrorResponse`
- new narrowly named files in `pkg/release` for command requests/results/mapping

### Non-goals

- No side-effect reordering.
- No runner/resume extraction.
- No package move or public symbol rename.
- No response/error cleanup beyond exact consolidation.

### Behavioral invariants

Every existing status, error code, message/details, metadata command, renderer hint, data key, and item order remains byte-for-byte equivalent after timestamp normalization.

### Tests required before modification

Stage 1 command contracts plus all existing manifest and dry-run tests.

### Implementation steps

1. Define strict parsing functions that produce typed release/resume requests while preserving current defaults and accepted inputs.
2. Define typed outcomes matching current dry-run, V1 success, V2 handoff, resume assessment, and classified failure shapes.
3. Extract pure response mappers with an injected timestamp.
4. Route existing orchestration results through the mappers one branch at a time.
5. Keep compatibility wrappers for current exported handlers.

### Acceptance criteria

- Release and resume handlers parse, invoke one application boundary, and map its typed result without directly ordering Git, filesystem, journal, state, or network work.
- Characterization tests show no stable response field, item order, code, or timestamp semantics changed.
- Existing exported command entry points and manifest routes remain unchanged.
- The result passes the global clean-code completion gate; extraction has not merely moved mixed handler responsibilities into a vague service.

### Clean-code acceptance

- Each handler can be described as parse/validate, create request, invoke one focused use case, and map one result, with no repository, unit, remote, journal, token, execution-context, or phase coordination.
- Request parsing, application invocation, and response mapping are separate focused functions at one abstraction level; response mappers are pure apart from their explicit clock.
- No generic command framework, broad result wrapper, boolean workflow selector, or vague `ReleaseService`/`ResumeManager` boundary is added.

### Validation

Run Stage 1 focused contracts, manifest tests, then the full baseline.

### Expected commit message

`refactor(release): isolate release command presentation`

### Risks

The greatest risk is accidental code/message/item-order drift. Golden or structured comparison tests must compare the old and new mapping during migration.

### Rollback strategy

One atomic commit. Revert it to restore the existing handlers; no disk/journal schema migration is involved.

### Completed implementation

- `ParseReleaseCommandRequest` and `ParseResumeCommandRequest` exclusively translate plugin flag maps into typed requests. Wrongly typed values retain the characterized empty/false defaults; no new parser failure code was introduced.
- `releaseCommandHandler` and `resumeCommandHandler` each parse, invoke one focused application interface exactly once, and map one sealed typed outcome or `CommandFailure`.
- Release preview/completion, V2 preview, GitHub Actions handoff, and resume assessment are represented by command-specific outcome types rather than a generic result wrapper.
- `command_response.go` is the single release/resume presentation mapper. Stable status, metadata, details, renderer, data keys, row values, and row order are constructed with an explicit timestamp supplied by an injected `responseClock`.
- Exported `HandleRelease`, `HandleResume`, and `V2ExecutionUnavailableResponse` signatures remain intact. `releaseStartOperation`, `resumeReleaseOperation`, `GitHubActionsReleaseRunner.Run`, and `resumeJournal` are compatibility facades around the unchanged execution behavior.
- Focused parser, handler-boundary, failure-mapping, fixed-timestamp, and ordered-row tests supplement the unchanged Stage 1 command/runner/resume contracts.

### Residual deferrals

- `releaseStartOperation` still performs repository/unit/context selection and constructs `GitHubActionsReleaseRunner`; the active V2 execution behind that facade is extracted in Stage 3, while broader start-operation selection remains compatibility wiring.
- The resume application path originally retained duplicated continuation policy and boolean mode parameters after Stage 2; Stage 4 has since replaced that compatibility path with the explicit resume use case and named operations recorded below.
- V1 `Service` and tool orchestration remain compatibility code. No V1/V2 consolidation, response cleanup outside release/resume, validation policy change, or new malformed-flag behavior was introduced.

Next exact stage: **Stage 4: Model resume policy and reuse the active release steps.**

### Dependencies

Stage 1.

## Stage 3: Extract the active V2 release use case with replaceable dependencies

Status: completed on 2026-07-14 by `refactor(release): extract github actions release use case`.

### Goal

Turn the active GitHub Actions release procedure into a testable use case with named ordered steps while preserving `GitHubActionsReleaseRunner` as the production facade.

### Current problem

`GitHubActionsReleaseRunner.Run` constructs stores/adapters internally and mixes policy, persistence, Git, files, dispatch, logging, and result creation. Failure after a side effect but before journal confirmation cannot be injected comprehensively.

### Scope

- Supply focused dependencies to the operations that need them; compose production wiring at the facade without an oversized dependency container.
- Wrap existing production implementations; do not rewrite them.
- Extract focused named operations for preflight, prepare/apply/confirm, commit/tag, journal handoff, pushes, dispatch, and handoff without introducing a generic step abstraction.
- Make a fake-driven failure matrix possible.

Likely files/symbols:

- `pkg/release/github_actions_release_runner.go`
- `pkg/release/release_execution_journal_store.go`
- `pkg/release/dispatch_journal_store.go`
- `pkg/release/git_release_coordinator.go`
- `pkg/release/state_transaction.go`
- `pkg/release/materialization_transaction.go`
- a new `pkg/release/github_actions_release_use_case.go`

### Non-goals

- No journal schema/state/identity change.
- No new retry, rollback, or resume behavior.
- No V2 local execution.
- No consolidation with V1.
- No replacement of real adapter implementation.

### Behavioral invariants

The exact active order and pending/confirmed journal boundaries remain unchanged. Token resolution remains before journal/repository mutation. Commit and tag pushes remain separate and ordered. Handoff occurs only after accepted dispatch.

### Tests required before modification

- Stage 1 command and outcome contracts.
- Existing journal state-machine tests.
- Existing Git coordinator and runner happy paths.
- Add an initial call-order characterization around the current happy path if not completed in Stage 1.

### Implementation steps

1. Define narrow ports around existing operations: token, planning/materialization, Git, execution journal, dispatch journal/dispatcher, state/files, and clock where needed.
2. Build production adapters from current types.
3. Extract one cohesive decision or side effect at a time from `Run`; every extracted function must have one verb-phrase responsibility and one abstraction level.
4. Keep `Run` responsible only for facade validation, explicit production wiring, invocation of the focused start-release use case, and returning its typed result.
5. Add fake-driven tests for failure before and after every unsafe operation listed in `RULES.md`.
6. Assert last phase, pending marker, stopped calls, local/remote evidence policy, and secret absence for every case.

### Acceptance criteria

- One production V2 GitHub Actions execution path remains. Its focused start-release use case shows safety-critical order through named application-level calls while delegating each validation, decision, and side effect.
- Tests can inject a failure on both sides of every unsafe boundary and prove the resulting durable journal state and absence of later calls.
- Persisted files, journal schemas, Git commands, dispatch inputs, responses, and dry-run behavior match the characterized baseline.
- The result passes the global clean-code completion gate; no broad runner replacement, dependency bag, callback pipeline, workflow interpreter, or mixed helper becomes the new god function.

### Clean-code acceptance

- The start-release use case, facade, and every extracted operation each have one responsibility and one abstraction level.
- Operations receive only their explicit required dependencies; store/client construction and plugin response mapping do not occur inside application logic.
- Safety order remains visible without a generic step list, transition engine, boolean workflow selector, large phase switch, or shared result mutation used as control flow.

### Validation

Run focused runner/use-case, journals, Git coordinator, state/materialization, dispatch tests, then the full baseline.

### Expected commit sequence

```text
test(release): characterize v2 release failure boundaries
refactor(release): extract github actions release use case
```

Use one refactor commit only if tests and extraction are inseparable and still reviewable.

### Risks

An overly broad interface could merely hide the same god function. An overly fine interface could obscure the safety order. Keep named steps visible in one use-case method and adapters cohesive.

### Rollback strategy

The runner facade and persisted schemas remain stable, so reverting the extraction commit restores the old orchestration. Do not ship a partial cutover with two active execution paths.

### Dependencies

Stages 1 and 2.

### Completion record

- `GitHubActionsReleaseRunner.Run` now performs facade validation, request logging, explicit production composition, and one use-case invocation.
- `githubActionsReleaseUseCase.Run` exposes the exact token, plan, preflight, execution-journal, materialization, state, stage, commit, tag, dispatch-journal, commit-push, tag-push, dispatch, and accepted-handoff order through named calls.
- Consumer-owned operation interfaces make token, planning, preflight, execution journal, materialization/state transactions, Git mutations, dispatch journal/request construction, workflow dispatch, and handoff confirmation replaceable without a generic dependency bag or step pipeline.
- Each of the eight unsafe local/remote mutations owns its pending marker, side effect, and confirmed phase. Existing stores, transactions, Git coordinator, dispatcher, journal schemas, Git commands, response mapping, and recovery policy remain unchanged.
- Fake-driven tests cover the full call order, every use-case dependency failure, cleanup stopping rules, rejected dispatch, token absence from captured logs/results, and failures before and after every journaled mutation. Existing real-repository runner tests remain the behavior baseline.
- `BuildReleaseDispatchRequest` retains direct construction of a read-only `GitReleaseCoordinator` for tag/committed-state verification. Stage 3 places the builder behind a focused seam; replacing that internal verifier is deferred to later adapter consolidation.
- The characterization and extraction tests depend on the new operation seams, so Stage 3 is delivered as one reviewable refactor commit as allowed by the stage plan.

Next exact stage: **Stage 4: Model resume policy and reuse the active release steps.**

## Stage 4: Model resume policy and reuse the active release steps

Status: completed on 2026-07-14 by `test(release): characterize existing tag resume behavior` and `refactor(release): centralize release resume orchestration`.

### Goal

Represent resume as an explicit transition policy and reuse Stage 3 operations instead of duplicating tag/push/dispatch orchestration.

### Current problem

`resumeJournal` branches directly on persisted state, duplicates runner operations, mutates the in-memory journal manually, and uses boolean mode parameters. Successful later-phase continuation lacks comprehensive tests.

### Scope

- Define a small pure resolver from journal plus local assessment to one supported named operation or one explicit refusal.
- Separate read-only assessment from mutating continuation.
- Implement focused operations such as resume from commit-created, tag-created, or tag-pushed, completed handoff, ambiguous-push rejection, and uncertain-dispatch rejection.
- Reuse Stage 3 tag, dispatch-journal, push, dispatch, and handoff capabilities without a generic transition engine.
- Remove `loadOnly` and `pushed` workflow booleans.

Likely files/symbols:

- `pkg/release/resume.go`
- `pkg/release/release_execution_recovery.go`
- `prepareDispatchJournalForResume`
- `dispatchRequestForResume`
- Stage 3 use-case/ports

### Non-goals

- No automatic retry for pending pushes or terminal dispatch journals.
- No remote-state probe unless a later task explicitly designs and authorizes one.
- No journal repair or schema migration.
- No universal resume workflow processor, large phase switch, or generic state-machine framework.

### Behavioral invariants

Resume remains tied to exactly one unresolved journal; no fresh version/tag/identity is generated. Corrupt, conflicting, and ambiguous states remain blocked. Dry-run remains read-only and token-free.

### Tests required before modification

Add a table covering every confirmed execution state crossed with every relevant pending action. Add successful current-behavior tests for `commit-created` with a missing tag, `tag-created`, and `tag-pushed`. Pin current behavior when the expected tag already exists.

### Implementation steps

1. Resolve the current state and assessment to one typed supported operation/refusal without encoding execution in the resolver.
2. Test the decision table as a pure function.
3. Route dry-run directly through assessment and presentation.
4. Route each allowed case to its focused named use case using only that case's required Stage 3 capabilities.
5. Remove duplicated side-effect functions only after all call sites use the shared operations.

### Acceptance criteria

- Every persisted execution phase/pending-action combination produces one explicit resume action or one explicit refusal.
- Dry-run assessment performs no token lookup or mutation, and mutating resume reuses the active release operations without boolean workflow selectors.
- Existing journals remain readable and all uncertain push/dispatch cases remain blocked.
- The result passes the global clean-code completion gate; no generic resume engine or phase-switch god function replaces `resumeJournal`.

### Validation

Run all resume, recovery, journal, dispatcher, runner, and command contract tests, then the full baseline.

### Expected commit sequence

```text
test(release): characterize resume transition policy
refactor(release): centralize release resume orchestration
```

### Risks

Persisted local evidence may be insufficient for a desired continuation. The safe result is to preserve a block, not infer success.

### Rollback strategy

Revert the resume refactor commit. No persisted schema changes are permitted, so existing journals remain readable by the prior implementation.

### Completion record

- `resumeReleaseUseCase` now coordinates discovery, local recovery assessment, dry-run return, typed policy resolution, context compatibility, one named operation selection, and invocation without Git/HTTP/response details.
- `resolveResumeRecovery` covers every confirmed execution state and relevant pending action as one typed supported operation or refusal. `resolveResumeDispatch` separately permits a fresh immutable dispatch, reuses accepted dispatch, or refuses request-started/rejected/unknown retry.
- `resumeFromCommitCreatedOperation`, `resumeFromTagCreatedOperation`, `resumeFromTagPushedOperation`, and `returnCompletedReleaseHandoffOperation` own the supported recovery intentions. The expected-tag-already-present block remains pinned to its pre-refactor behavior.
- Resume composition reuses Stage 3 tag creation, dispatch-journal preparation, commit push, tag push, workflow dispatch, and accepted-handoff confirmation. No resume flag, load-only mode, pushed mode, generic transition engine, workflow pipeline, or dependency bag was introduced.
- Dry-run returns before context reconstruction and effects. Corrupt/conflicted recovery and ambiguous pending pushes refuse before continuation. Config compatibility retains its established failure contract.
- Token resolution is confined to a fresh-dispatch operation. Accepted dispatch reuse and completed handoff require no token, do not redispatch, and only confirm the existing accepted handoff where required.
- `resumeJournal`, `prepareDispatchJournalForResume`, and `dispatchRequestForResume` were removed. The loaded journal is no longer manually mutated as orchestration control state.
- Pure policy, selector, use-case, named-operation, push-boundary, token/dispatch/handoff, accepted-reuse, completed-handoff, architecture, and real-repository continuation tests supplement all retained Stage 1-3 tests.
- Retained limitations remain deliberate: no remote-state probe, automatic retry, journal repair/schema migration, pre-commit continuation, or inference from ambiguous external effects.

Next exact stage: **Stage 5: Consolidate canonical release files, Git, journals, tokens, and clocks.**

### Dependencies

Stage 3.

## Stage 5: Consolidate canonical release files, Git, journals, tokens, and clocks

### Goal

Reduce duplicated side-effect access and make canonical V2 metadata drive release-file behavior.

### Current problem

Git, token, clock, and filesystem access use multiple mechanisms. Plugin materialization duplicates validated config in a hard-coded unit map. Journal stores duplicate Git common-dir and atomic JSON plumbing.

### Scope

- Make plugin materialization use `ReleaseUnit.PluginManifestPath`/`IsPlugin`.
- Consolidate Git common-directory resolution and safe journal persistence without changing paths/bytes/permissions.
- Use one V2 token abstraction and injected clocks for persisted/output timestamps.
- Route active V2 Git access through the existing tested coordinator adapter or a narrowly extracted successor.

Likely files:

- `pkg/release/plugin_manifest_materializer.go`
- `pkg/release/release_execution_journal_store.go`
- `pkg/release/dispatch_journal_store.go`
- `pkg/release/github_actions_dispatch_client.go`
- `pkg/release/git_release_coordinator.go`
- response mapper files from Stage 2

### Non-goals

- No V1 Git/tool rewrite yet.
- No journal schema/version change.
- No plugin-index schema change.
- No new output-path policy for plugin-index.

### Behavioral invariants

Existing `plugin-release` and `plugin-ui` manifest paths and exact materialized bytes remain unchanged. Journal identities/locations/permissions and dispatch token messages remain unchanged.

### Tests required before modification

- Add arbitrary validated plugin-unit materialization tests.
- Add byte-for-byte journal fixtures or serialization assertions.
- Add deterministic clock tests and repository-root/worktree path tests.

### Implementation steps

1. Switch plugin materialization from the map to normalized unit metadata under characterization.
2. Remove the map only after self-release and dry-run tests pass.
3. Extract shared Git common-dir/path and secure JSON-store primitives while keeping store-specific validation.
4. Route clocks/tokens through explicit dependencies from Stage 3.
5. Remove duplicated active V2 Git/environment paths only when unused.

### Acceptance criteria

- Any validated plugin unit materializes through its configured manifest path while existing plugin manifests remain byte-identical.
- Execution and dispatch journal locations, names, permissions, schemas, identities, and validation remain compatible.
- Active V2 code has one replaceable boundary each for Git, token lookup, and generated time, with deterministic tests.
- The result passes the global clean-code completion gate; adapter consolidation does not create a broad infrastructure interface or dependency container.

### Validation

Run materializer, self-migration, dry-run, journals, dispatch, runner, and full baseline tests.

### Expected commit sequence

```text
refactor(release): use configured plugin materialization paths
refactor(release): consolidate v2 release adapters
```

### Risks

Journal serialization drift can strand recovery data. Compare exact schema fields and identity hashes before and after. Symlink/path validation must not weaken.

### Rollback strategy

Each adapter consolidation is a separate commit. Revert the affected commit; persisted contracts must remain backward-readable throughout.

### Dependencies

Stages 3 and 4. The plugin metadata sub-step may follow Stage 1 independently if kept isolated.

### Completion record

Stage 5 was completed on 2026-07-14.

- Plugin materialization now uses validated `ReleaseUnit.IsPlugin` and `ReleaseUnit.PluginManifestPath`; the production unit-to-path map was removed, and arbitrary validated plugin metadata is characterized alongside byte-identical self-release manifests.
- `releaseJournalFiles` owns only Git common-directory resolution, fixed execution/dispatch locations, canonical indented JSON serialization, private directory creation, and atomic private writes. Execution and dispatch stores retain their distinct schemas, validation, mutations, lookup rules, and error messages.
- Active release and resume composition create one `GitReleaseCoordinator` and expose it through consumer-owned capabilities. Dispatch-request building and recovery no longer construct Git infrastructure; they receive focused read-only verification/tag-inspection capabilities.
- `GitHubActionsDispatchToken` is the single active V2 secret-bearing value from `GITHUB_TOKEN` resolution through the dispatcher/client boundary, with redacted formatting and no static re-wrapping resolver.
- `ReleaseClock` supplies release/resume response timestamps and all active V2 persisted execution/dispatch timestamps. Deterministic end-to-end timestamp tests cover the composed runner.
- Existing public constructors remain compatibility entry points with production Git, environment-token, and system-clock defaults. V1 tools, inactive V2 transaction/convenience paths, and their direct adapters were not rewritten.
- Exact journal byte fixtures, modes, linked-worktree paths, injected atomic-write failures, focused verifier injection, token source/redaction, architecture boundaries, and Stage 1-4 compatibility are covered without schema, output, or release-order changes.

Next exact stage: **Stage 6: Extract init and unit-add use cases with paired persistence.**

## Stage 6: Extract init and unit-add use cases with paired persistence

### Goal

Move request parsing/presentation out of V2 initialization logic and define safe config/state pair-write behavior.

### Current problem

`pkg/init/handler.go` mixes all concerns, and `writeV2Files` can leave config written when state write fails.

### Scope

- Typed init and add-unit requests/results.
- A V2 config/state repository with explicit create, replace, and append operations.
- Characterized pair-write/restore behavior.
- Correct command metadata through shared response mapping.

Likely files:

- `pkg/init/handler.go`
- `pkg/init/handler_test.go`
- `pkg/config/atomic_writer.go`
- a new init application/repository file within existing packages

### Non-goals

- No new init flags, executor scaffolding, workflow generation, or multi-unit migration.
- No change to `--force` or V1 conflict policy.
- No generic repository transaction framework.

### Behavioral invariants

Existing config/state JSON shape, defaults, validation, plugin rules, force behavior, duplicate protection, response codes, and next-step output remain stable. Existing files are never partially replaced without a tested recovery contract.

### Tests required before modification

Inject failure at directory creation, config temp/write/rename, state temp/write/rename, and any restore. Assert exact pre-existing bytes and modes after failure. Pin `unit-add` error metadata command before deciding whether it is a compatibility bug or a later fix.

### Implementation steps

1. Extract typed parsing and pure config/state construction.
2. Introduce paired persistence with snapshots or another explicit bounded recovery design.
3. Extract init and add-unit use cases.
4. Reduce handlers to parse/invoke/map.
5. Update current architecture with the chosen pair-write contract.

### Acceptance criteria

- Init and unit-add handlers are presentation boundaries and the application operations expose typed requests/results.
- Failure injection proves the documented config/state pair outcome at every write and restore boundary, including exact preservation of pre-existing bytes and modes.
- JSON schemas, defaults, response contracts, duplicate/force behavior, and command routes remain unchanged.
- The result passes the global clean-code completion gate; init and unit-add remain separate focused intentions rather than methods on a generic configuration service.

### Validation

Run all init/config/workspace/manifest tests and the full baseline.

### Expected commit sequence

```text
test(release): characterize v2 config pair failures
refactor(release): isolate v2 initialization use cases
```

### Risks

Trying to guarantee cross-file atomicity with renames alone can create a false safety claim. Document exactly which failures restore and which require recovery.

### Rollback strategy

Keep disk schema unchanged. Revert the use-case/persistence commits together if pair behavior regresses.

### Dependencies

Stage 2 response patterns are helpful but not mandatory. Do not duplicate Stage 2 mapping abstractions.

## Stage 7: Extract read-only query use cases and plugin-index output persistence

### Goal

Give validate, history, contributors, and plugin-index consistent typed command boundaries and replaceable read/query adapters.

### Current problem

These handlers duplicate flag parsing and response construction. Git failures and plugin-index Go-error fallback behavior are inconsistent. Plugin-index writes are non-atomic.

### Scope

- Typed use cases for validate, history, contributors, and plugin-index generation.
- Shared read-only repository/Git query dependencies where behavior aligns.
- Explicit plugin-index output writer with atomic single-file persistence.
- Preserve current structured-vs-fatal error behavior until tests and an explicit compatibility decision allow centralization.

### Non-goals

- No plugin-index schema or publication workflow change.
- No new inspection feature in this stage.
- No change to whether V1 validation requires a token without explicit feature authorization.

### Behavioral invariants

Unit/path/tag selection, sort order, raw JSON formatting, response shapes, and manifest/workflow contracts remain unchanged.

### Tests required before modification

Characterize every Git/filesystem failure mapping, item order, empty result, cancellation, and plugin-index output write failure.

### Implementation steps

1. Extract pure query results and response mappers command by command.
2. Reuse canonical unit/tag models.
3. Inject Git query and filesystem readers.
4. Introduce an atomic plugin-index output adapter.
5. Keep command commits independent so each remains reviewable.

### Acceptance criteria

- Each query handler parses, invokes one typed use case, and maps one typed result while preserving its current structured-versus-fatal error boundary.
- Read-only use cases can run with fake Git/filesystem dependencies and retain exact selection, sorting, and response behavior.
- Plugin-index file output is atomically persisted without changing its schema or stdout behavior.
- The result passes the global clean-code completion gate; shared query code is extracted only for demonstrated cohesive behavior, not into a generic query framework.

### Validation

Run history, contributors, validate, config/tag, plugin-index, manifest/workflow script, and full baseline tests.

### Expected commit sequence

```text
refactor(release): isolate release query use cases
refactor(release): isolate plugin index output persistence
```

### Risks

Centralizing errors may inadvertently convert a handler Go error to a structured response or vice versa. Preserve the current boundary until an explicit contract change.

### Rollback strategy

One commit per query group/output adapter. Revert independently.

### Dependencies

Stage 2 mapping patterns; Stage 5 canonical adapter conventions.

## Stage 8: Type and isolate migration recovery

### Goal

Make migration phases and filesystem operations explicit and failure-injectable while preserving the root single-unit migration contract.

### Current problem

Migration is relatively cohesive but its journal stage is an unvalidated string and direct filesystem/Git operations make failure boundaries difficult to test.

### Scope

- Typed migration stages with explicit transition/validation rules.
- Replaceable Git-root and filesystem operations.
- Failure injection for every journal/write/archive/validate/remove boundary.
- Keep serialized stage strings and journal schema version compatible.

Likely files:

- `pkg/migrate/migration.go`
- `pkg/migrate/migration_test.go`
- existing config atomic writer

### Non-goals

- No multi-unit migration.
- No nested V1 conversion.
- No journal relocation or schema bump.
- No inferred plugin metadata.
- No generic transition engine or reusable workflow framework for migration phases.

### Behavioral invariants

Dry-run remains read-only. Backup remains byte-identical. Existing planned content hashes and recovery conflict checks remain authoritative. The journal is removed only after final validation.

### Tests required before modification

Add corrupt/unknown stage cases and injected failures at every persisted boundary, including failure to remove the final journal.

### Implementation steps

1. Wrap current stage strings in a validated type without changing JSON.
2. Extract a migration filesystem/Git adapter.
3. Test exact recovery plans from every durable boundary.
4. Split planning from execution while keeping `Run` as facade.

### Acceptance criteria

- Migration stage transitions are typed in code but serialize to the existing strings and schema.
- Tests inject every journal/write/archive/validate/remove failure and prove the exact recoverable disk state.
- Dry-run, backup bytes, planned hashes, conflict behavior, and root-only migration scope remain unchanged.
- The result passes the global clean-code completion gate; typed phases validate persisted data but do not become a generic state-machine executor.

### Validation

Run migration, config atomic/V1/V2, validate, and full baseline tests.

### Expected commit sequence

```text
test(release): characterize migration failure boundaries
refactor(release): type migration recovery phases
```

### Risks

Rejecting a journal previously accepted by accident is a behavior change. Characterize malformed-but-currently-accepted cases before tightening validation.

### Rollback strategy

Keep serialized format unchanged so reverting production code does not strand journals.

### Dependencies

Stage 5 adapter conventions are helpful but not required.

## Stage 9: Isolate the V1 compatibility subsystem

### Goal

Contain legacy local executor behavior behind explicit adapters without changing its semantics, then make its future support/removal decision independent from V2.

### Current problem

V1 uses global tool registration, direct subprocess/environment/network access, fatal error helpers inside preflight, and destructive rollback helpers. It shares package surfaces with inactive V2 local preparation.

### Scope

- Characterize every V1 executor's command order, config update, push/publish ownership, and rollback flags.
- Introduce replaceable subprocess, Git, token, and GitHub Release adapters.
- Replace fatal exits inside orchestration with classified failures mapped at the command boundary while preserving output/exit contracts.
- Clearly mark inactive V2 local code and either route it through the established use-case boundary or remove it only under separate proof.

Likely files:

- `pkg/release/service.go`
- `pkg/release/preflight.go`
- `pkg/release/tool.go`
- `pkg/release/registry.go`
- `pkg/release/tool/*`
- `pkg/git/repository.go`
- `pkg/release/release_transaction.go`

### Non-goals

- No V1 behavior redesign or removal.
- No activation of V2 local execution.
- No new executor.
- No destructive rollback expansion.

### Behavioral invariants

Current V1 clean/main/upstream/token/version rules, config timing, executor commands, warnings, and guarded rollback behavior remain stable. No V1 destructive operation may become reachable earlier.

### Tests required before modification

Build fake-driven executor and rollback matrices first. Include no-mutation guard failures, commit/tag/push/publish failures, GitHub deletion, revert fallback, reset, and cleanup behavior.

### Implementation steps

1. Characterize tool registry and each executor independently.
2. Introduce adapters around direct side effects.
3. Extract V1 application orchestration from fatal error/response helpers.
4. Decide in a separate change whether inactive V2 local preparation remains useful.

### Acceptance criteria

- V1 executor command sequences, release ownership, warnings, failures, and guarded rollback effects are pinned by fake-driven tests.
- V1 orchestration can be tested without real subprocess, Git remote, network, token, or wall-clock access.
- No V2 local path is activated and no V1 destructive operation becomes reachable under a broader condition.
- The result passes the global clean-code completion gate; compatibility facades do not become permanent god services or hide destructive ordering behind generic steps.

### Validation

Run all V1 config/requirements/version/tool/rollback tests, full Release Plugin tests, then the full baseline.

### Expected commit sequence

```text
test(release): characterize legacy executor recovery
refactor(release): isolate v1 release compatibility
```

### Risks

Legacy rollback can modify remote state. Tests must use fakes or isolated repositories and must never contact a real remote.

### Rollback strategy

Keep compatibility facades and one commit per executor/core extraction. Revert the affected extraction without touching user repositories or remotes.

### Dependencies

Stages 2 and 3 establish the preferred command/use-case patterns. Perform this after the active V2 path is stable.

## Atomic commit sequence across the plan

The expected high-level sequence is:

1. `test(release): characterize v2 release command contracts`
2. `refactor(release): isolate release command presentation`
3. `test(release): characterize v2 release failure boundaries`
4. `refactor(release): extract github actions release use case`
5. `test(release): characterize resume transition policy`
6. `refactor(release): centralize release resume orchestration`
7. `refactor(release): use configured plugin materialization paths`
8. `refactor(release): consolidate v2 release adapters`
9. `test(release): characterize v2 config pair failures`
10. `refactor(release): isolate v2 initialization use cases`
11. independent query/plugin-index/migration commits from Stages 7 and 8
12. V1 characterization followed by its isolated refactor commits

Do not squash characterization into later behavior changes when retaining a standalone baseline materially improves rollback and review.

## Rollback boundaries

- Tests/documentation are independent baseline commits.
- Command mapping can revert independently because handler signatures remain stable.
- Active runner extraction must cut over atomically; never leave two production orchestrators selectable.
- Resume extraction can revert independently while journal schemas remain unchanged.
- Adapter consolidations are one concern per commit and must preserve persisted bytes.
- Init pair-persistence changes are one recovery contract boundary.
- Query commands are independently revertible.
- Migration changes must preserve old journal readability at every commit.
- V1 executor changes are isolated per adapter/executor after characterization.

No rollback for this refactor plan may use `git reset --hard`, rewrite published history, delete user journals, or mutate a release remote.

## First three executable milestones

1. **Characterize V2 release command contracts (Stage 1).** One focused task, normally one test commit. Pin active response/error order, unresolved-journal blocking, dispatch terminal outcomes, resume restrictions, and secret absence.
   - **Clean-code acceptance:** Test externally visible behavior rather than current function structure; keep fixtures focused and expose the seams needed to reject mixed handler/application/infrastructure responsibilities later.
2. **Isolate release command presentation (Stage 2, completed 2026-07-14).** One focused refactor with typed request/result mapping and an injected response clock. No side-effect code moves.
   - **Clean-code acceptance:** Handlers only parse/validate, create a typed request, invoke one focused use case, and map one result. No broad service, generic command framework, boolean workflow selector, or infrastructure access is introduced.
3. **Extract the active GitHub Actions release use case (Stage 3, completed 2026-07-14).** The runner is a facade over one ordered use case; focused operation ports provide failure injection around every journaled mutation in one atomic cutover.
   - **Clean-code acceptance:** Each operation has one responsibility and one abstraction level, dependencies are explicit and narrowly scoped, response mapping stays outside application logic, and safety order is visible without a god function, dependency bag, generic step pipeline, or state-machine engine.

## When new features can begin

Read-only inspection features that only consume existing canonical config/state/journal models can begin after Stages 1 and 2 if they use typed query/use-case boundaries and do not extend the active runner.

CI-facing read-only planning or validation features can begin after Stage 3 if they reuse the extracted planning dependencies and remain token-free and non-mutating.

Features that change active release execution, journal/recovery semantics, dispatch, or retry must wait until Stages 3 and 4 are complete and the full failure matrix passes.

Features that consume arbitrary validated plugin units may now reuse the Stage 5 metadata-driven materializer; changing plugin-index policy or adding release behavior still requires its own authorized stage.

V2 local execution must not begin until Stages 3 through 5 are complete and a separate design explicitly resolves executor ownership, state-in-commit guarantees, unsafe-operation journaling, and recovery. Release-it remains blocked unless that design proves the root state can be included safely.
