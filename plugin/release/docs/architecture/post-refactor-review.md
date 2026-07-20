# Release Plugin Architecture

## Status

This document describes the architecture established by the ten-commit Release
Plugin code-quality refactor completed in July 2026. It is the concise review
entry point. [current-state.md](current-state.md) remains the detailed command,
disk, and wire-contract reference;
[maintainability-policy.md](maintainability-policy.md) defines the controls for
future changes; and [compatibility-notes.md](compatibility-notes.md) records the
preserved surfaces.

The architecture is a pragmatic hybrid. Stable public packages and compatibility
facades remain in place, independent command capabilities live in focused
`internal` packages, shared facts live in infrastructure-free leaf packages,
and the active V2 lifecycle remains an explicit straight-line use case in
`pkg/release`. It is deliberately not a universal layer tree, service framework,
pipeline, or second state machine.

## Dependency direction

```text
plugin protocol and compatibility facades
                    |
                    v
focused command capabilities and release orchestration
                    |
                    v
consumer-owned ports and canonical facts
                    |
                    v
concrete filesystem, Git, process, and HTTP adapters
```

Composition happens at `plugin/release/main.go`, public compatibility facades,
or command boundaries. Canonical facts never depend upward on Doctor,
presentation, journals, or HTTP. Extracted command capabilities never import
root release orchestration. Response mapping receives typed outcomes and owns no
lifecycle decision.

## Responsibility map

| Package | Responsibility | Boundary |
| --- | --- | --- |
| `plugin/release` | Decode/encode the plugin protocol, resolve one repository root, construct fresh V1 executors, and route commands | Composition only; no domain model, lifecycle, or persistence |
| `internal/releasetool` | Canonical identity, display, command, configuration-candidate, and V1 behavior facts for supported release tools | Pure leaf facts |
| `internal/releasetool/goreleaser` | GoReleaser config parsing, artifact-contract parsing, and invocation facts | Pure/local reads; no Doctor, HTTP, journals, or presentation |
| `internal/releasetool/jreleaser` | Canonical JReleaser configuration parsing/rewriting and version normalization | Pure/local reads |
| `internal/releasetool/releaseit` | Canonical release-it configuration parsing | Pure/local reads |
| `internal/releaseworkflow` | Canonical workflow name/file/ref/inputs and repository-target facts | Pure leaf facts |
| `internal/githubdispatch` | One bounded GitHub workflow-dispatch POST transport and response sanitization | HTTP mutation leaf; no lifecycle policy |
| `internal/releasesource` | Tolerant, local, read-only V1/V2 source classification | Read-only leaf |
| `internal/doctor` | Doctor inspection, diagnostic policy, presentation, and optional bounded GET verification | Default local/offline; remote path GET-only |
| `internal/unitoverview` | Local V2 unit inventory and its response presentation | Read-only command capability |
| `internal/contextvalidation` | Local/CI release-context validation and response presentation | Read-only command capability |
| `internal/workflowinit` | Canonical workflow preview and create-only persistence | One focused filesystem mutation |
| `internal/legacyrequirements` | V1 token, tool, and required-file validation retained for release and Validate | Compatibility leaf |
| `pkg/config` | V1/V2 models, loading, normalization, validation, path rules, atomic writes, and V2 pair persistence/recovery | Canonical configuration/state owner |
| `pkg/release` | V1 planning/compatibility, V2 planning, explicit release orchestration, journals, Git coordination, dispatch handoff, resume, and public facades | Authoritative release lifecycle owner |
| `pkg/release/tool/*` | Behavior-preserving V1 GoReleaser, JReleaser, and release-it executor adapters | Process/filesystem/Git compatibility adapters |
| `pkg/init` | Initialize a V2 pair, add a unit, and expose static init options | Focused config mutation |
| `pkg/migrate` | Discover, plan, journal, execute, verify, and recover V1-to-V2 migration | Separate migration lifecycle |
| `pkg/evidence` | Query redacted journals/evidence and archive completed evidence | Read-only query plus guarded archive mutation |
| `pkg/validate` | Read-only V1/V2 validation query and presentation | No root release dependency |
| `pkg/history` | Read-only V1/V2 tag and history query | Consumer-owned Git reads |
| `pkg/contributors` | Read-only V1/V2 contributor query | Consumer-owned Git reads |
| `pkg/pluginindex` | Discover, validate, deterministically build, and optionally persist the plugin index | Query/build/persist boundaries remain distinct |
| `pkg/workspace` | Resolve and validate explicit roots; retain isolated cwd compatibility helpers | Root/path boundary |
| `pkg/git` | Retained legacy and read-only Git helpers used below focused ports | Compatibility infrastructure |
| `pkg/metadata` | Plugin identity and version | Static facts |

## Root release responsibility

The root package `pkg/release` has a bounded responsibility:

1. preserve supported release-facing Go contracts and compatibility facades;
2. select V1 versus V2 once from the canonical source;
3. own V1 release planning/execution coordination;
4. own V2 planning, the authoritative mutation order, journals, handoff, and
   resume/recovery policy;
5. compose focused internal command capabilities behind existing public
   handlers.

It does not own reusable tool parsing, workflow constants, dispatch HTTP,
Doctor diagnostics, Unit Overview inspection, Context Validation inspection, or
workflow scaffolding. A new root production file must satisfy one of the five
responsibilities above and pass the architecture review in
[maintainability-policy.md](maintainability-policy.md).

## Release-tool ownership

`internal/releasetool` is the shared identity owner. Each concrete fact package
owns exactly the format-specific facts reusable by Doctor, planning, validation,
and the V1 executor compatibility adapter.

- GoReleaser: candidate paths, YAML parsing, artifact shapes, and invocation
  arguments are canonical under `internal/releasetool/goreleaser`.
- JReleaser: config interpretation, version normalization, and V2 rewriting are
  canonical under `internal/releasetool/jreleaser`.
- release-it: config interpretation is canonical under
  `internal/releasetool/releaseit`.
- `pkg/release/tool/*` retains execution, environment, compensation, and legacy
  public method shapes; it delegates configuration facts inward.

Production composition constructs the three V1 executors explicitly. The
mutable global registry is retained only for direct compatibility callers and
the opt-in `pkg/release/tool` aggregator.

## Authoritative V2 release flow

`githubActionsReleaseUseCase.Run` is the only active V2 mutation coordinator.
Named operation files make the order visible without introducing a dynamic
stage registry:

1. resolve the typed dispatch token;
2. plan materialization and the exact known-file allowlist;
3. run Git, workflow, and unresolved-journal preflight;
4. prepare the execution journal;
5. apply local file materialization and confirm it;
6. write selected-unit state and confirm it;
7. stage only known files and confirm it;
8. create and verify the release commit, then confirm it;
9. create and verify the unit tag, then confirm it;
10. prepare the dispatch journal and confirm it;
11. push the release commit and confirm it;
12. push the unit tag and confirm it;
13. mark the workflow request started, submit it, and classify the result;
14. confirm handoff only after an accepted submission.

The files `release_operation_plan.go`, `release_operation_local_files.go`,
`release_git_staging.go`, `release_git_commit.go`, `release_git_tag.go`,
`release_git_push.go`, and `release_operation_workflow.go` contain the named
operations. `github_actions_release_use_case.go` alone orders them.

Dry-run stops after planning and resolves no token, creates no journal, writes no
state, mutates no Git, invokes no release tool, and calls no HTTP. Active V2
never invokes GoReleaser, JReleaser, or release-it locally.

## Lifecycle and state-machine review

The persisted authoritative state owners are intentionally separate:

- `ReleaseExecutionJournalStore` owns V2 execution phases and pending-action
  evidence;
- `DispatchJournalStore` owns workflow-request evidence and result
  classification;
- the V1 compensation store owns V1-only compensation evidence;
- `pkg/migrate` owns its worktree migration journal;
- `pkg/config` owns V2 config/state pair-recovery evidence.

These stores share only narrow serialization or secure-file mechanics where
appropriate. They do not share a transition engine. Resume reads execution and
dispatch evidence, applies one pure continuation policy, and reuses the same
named operations as active execution. Transition, retry, and uncertainty policy
is not copied into response mapping or transports.

`ExecutionPhase` and `MutationTracker` remain quarantined compatibility types and
are not used by active production composition. There is no operation registry,
middleware chain, event loop, mutable pipeline context, dynamic step graph, or
generic pipeline executor. No Pipeline Inspection capability was added.

## Read-only and mutation boundaries

- Doctor is local and offline by default. Explicit remote verification injects
  a bounded GET-only reader; it cannot reach workflow dispatch.
- Plan, Units, Validate, History, Contributors, Evidence query, and Context
  Validation receive read capabilities only.
- Workflow Init owns only create/unchanged/conflict behavior and cannot update an
  existing workflow.
- GitHub dispatch owns one POST and no retry, journal, token-resolution, or
  lifecycle policy.
- Plugin Index keeps discovery, deterministic byte building, output-path policy,
  and atomic persistence separate.
- Init and Migration reuse the canonical V2 pair persister instead of
  reimplementing pair-write policy.

## Terminology glossary

| Term | Meaning |
| --- | --- |
| command routing | Selecting one command at the plugin protocol boundary |
| release orchestration | The explicit ordered composition of named release operations |
| tool facts | Neutral identity, configuration, artifact, version, or invocation facts |
| tool invocation | Starting a V1 release-tool process through its compatibility executor |
| release commit | The exact commit containing the known release files |
| unit tag | The selected unit's canonical `TagSpec` result |
| workflow request | The immutable GitHub Actions workflow-dispatch request |
| dispatch | GitHub workflow-dispatch compatibility concepts only |
| publication | Observable artifact/release publication performed by the workflow |
| handoff | Accepted workflow submission after both pushes |
| resume | Continuing a known persisted release identity from proven evidence |
| recovery | Classifying incomplete or uncertain effects and selecting a safe action |
| inspection | Read-only collection of facts |
| verification | Evidence that a requested effect or contract is present |
| diagnostic mapping | Doctor-owned conversion of inspection facts to stable diagnostics |
| compatibility facade | A retained supported or deprecated entry point that directly delegates to the canonical owner |

Avoid vague production names such as manager, processor, engine, generic
handler outside a request boundary, generic adapter, or unqualified state/result.
`ReleaseExecutionContext` is retained because it is an immutable typed release
input.

## Architecture controls

Static tests enforce root composition limits, inward dependency direction,
tool-fact isolation, Doctor read-only boundaries, the single dispatch POST,
workflow ownership, read-only query boundaries, compatibility quarantine,
operation-order structure, root isolation, and tracked-artifact naming. Lint
adds production function-length, cognitive-complexity, and nesting review with
documented exceptions for straight-line safety transactions and static mappings.

The controls are structural tripwires, not substitutes for behavioral tests.
Every release change still runs focused contract tests, the complete Release
Plugin suite, the repository suite, and lint with module downloads disabled when
dependencies are already present.
