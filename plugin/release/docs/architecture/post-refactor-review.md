# Release Plugin Architecture

## Status

This document describes the architecture established by the Release Plugin
code-quality refactor completed in July 2026: ten planned structural commits,
corrective follow-up Commit 11, and final corrective follow-up Commit 12. The
final sequence contains twelve commits. This is the concise review entry point.
[current-state.md](current-state.md) remains the detailed command,
disk, and wire-contract reference;
[maintainability-policy.md](maintainability-policy.md) defines the controls for
future changes; and [compatibility-notes.md](compatibility-notes.md) records the
preserved surfaces.

The architecture is a pragmatic hybrid. Stable public packages and compatibility
facades remain in place, independent command capabilities live in focused
`internal` packages, shared neutral facts live in infrastructure-free leaf packages,
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
presentation, journals, or HTTP. The JReleaser and release-it configuration
owners additionally provide focused local config-file load/save codecs; that
intentional filesystem I/O contains no lifecycle, HTTP, journal, or presentation
policy. Extracted command capabilities never import root release orchestration.
Response mapping receives typed outcomes and owns no lifecycle decision.

## Responsibility map

| Package | Responsibility | Boundary |
| --- | --- | --- |
| `plugin/release` | Decode/encode the plugin protocol, resolve one repository root, construct fresh V1 executors, and route commands | Composition only; no domain model, lifecycle, or persistence |
| `internal/releasetool` | Common tool identity, ordered configuration candidates, and static V1 tool-behavior facts | Pure leaf facts |
| `internal/releasetool/goreleaser` | GoReleaser configuration parsing, invocation classification, and artifact-contract facts | Pure byte/model facts; no filesystem, Doctor, HTTP, journals, or presentation |
| `internal/releasetool/jreleaser` | Canonical JReleaser configuration codec and project-version rewrite | Focused local config reads/writes; no HTTP or lifecycle policy |
| `internal/releasetool/releaseit` | Canonical release-it configuration codec and default configuration | Focused local config reads/writes; no HTTP or lifecycle policy |
| `internal/releaseworkflow` | Canonical workflow name/file/ref/inputs and repository-target facts | Pure leaf facts |
| `internal/githubdispatch` | One bounded GitHub workflow-dispatch POST transport and response sanitization | HTTP mutation leaf; no lifecycle policy |
| `internal/releasesource` | Tolerant, local, read-only V1/V2 source classification | Read-only leaf |
| `internal/doctor` | Doctor inspection plus Doctor-owned severity, messages, remediation, fact mapping, presentation, and optional bounded GET verification | Default local/offline; remote path GET-only |
| `internal/unitoverview` | Local V2 unit inventory and its response presentation | Read-only command capability |
| `internal/contextvalidation` | Local/CI release-context validation and response presentation | Read-only command capability |
| `internal/workflowinit` | Canonical workflow preview and create-only persistence | One focused filesystem mutation |
| `internal/legacyrequirements` | Source-format V1 token and configuration-file validation retained for the public V1 facade and Validate | Compatibility leaf; execution-context requirements remain distinct |
| `pkg/config` | V1/V2 models, loading, normalization, validation, path rules, atomic writes, and V2 pair persistence/recovery | Canonical configuration/state owner |
| `pkg/release` | V1 planning/compatibility, V2 planning, explicit release orchestration, journals, Git coordination, dispatch handoff, resume, and public facades | Authoritative release lifecycle owner |
| `pkg/release/tool/*` | Behavior-preserving V1 execution adapters and compatibility surfaces for GoReleaser, JReleaser, and release-it | Process/filesystem/Git compatibility adapters |
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

## Refactor metrics

The baseline is `cd2e590`, immediately before the twelve-commit code-quality
sequence. Production files and lines count direct non-test Go files in
`plugin/release/pkg/release`; internal packages count Go packages below
`plugin/release/internal`; moved files are Git-detected production Go renames.

| Measure | Before | After |
| --- | ---: | ---: |
| `pkg/release` production files | 110 | 88 |
| `pkg/release` production lines | 18,939 | 11,486 |
| Release internal packages | 0 | 12 |
| Moved production files | 0 | 47 |

## Responsibilities moved to `plugin/release/internal`

The following is the complete responsibility transfer made by this refactor:

1. shared release-tool identity, ordered configuration candidates, and static
   V1-behavior facts moved to `internal/releasetool`;
2. reusable GoReleaser config/artifact parsing and invocation facts moved to
   `internal/releasetool/goreleaser`;
3. canonical JReleaser config load/save, rewriting, and version facts moved to
   `internal/releasetool/jreleaser`;
4. canonical release-it config load/save and defaults moved to
   `internal/releasetool/releaseit`;
5. workflow names, files, refs, canonical inputs, and repository-target facts
   moved to `internal/releaseworkflow`;
6. the bounded GitHub workflow-dispatch POST and response sanitization moved to
   `internal/githubdispatch`;
7. tolerant local V1/V2 source inspection moved to `internal/releasesource`;
8. Doctor inspection, severity, messages, remediation, fact mapping,
   presentation, optional GET-only remote verification, and its white-box tests
   moved to `internal/doctor`;
9. Unit Overview inspection and presentation moved to `internal/unitoverview`;
10. dispatched Context Validation inspection, Git reads, diagnostics, and
    presentation moved to `internal/contextvalidation`;
11. Workflow Init source selection, canonical rendering, create-only policy,
    filesystem persistence, and presentation moved to `internal/workflowinit`;
12. the source-format V1 token/config-file requirement contract shared by the
    public requirements facade and Validate moved to `internal/legacyrequirements`.

## Responsibilities intentionally retained in `pkg/release`

| Retained responsibility | Why it belongs here |
| --- | --- |
| V1/V2 source selection and release command start | It selects the authoritative release application exactly once and establishes the release identity. |
| immutable V2 execution context and active `ReleasePlan` construction | These are the canonical inputs and facts consumed by the lifecycle, journal identity, and command result. |
| V2 preflight and exact named mutation ordering | `githubActionsReleaseUseCase.Run` is the sole active coordinator and must keep the safety order locally auditable. |
| known-file materialization and state transactions | They define the exact local mutations and bounded restoration allowed before commit uncertainty. |
| release Git coordination and verification | Commit, tag, targeted staging, ordered pushes, and evidence checks are lifecycle operations reused by execution and resume. |
| execution and dispatch journal application policy | Pending/confirmed transitions, immutable identities, and terminal classifications are authoritative lifecycle state. |
| workflow-request preparation, typed token boundary, result classification, and handoff | These surround the internal POST transport with lifecycle identity, evidence, and accepted-handoff policy. |
| resume assessment, recovery policy, and named continuation operations | Resume must reuse the same authoritative operations rather than implement a second state machine. |
| V1 intent, planning, execution coordination, and compensation | V1 remains a supported release lifecycle with its own characterized safety/evidence contract. |
| release command request/outcome/progress and response mapping | These are the supported public release boundary; mapping remains policy-free and adjacent to the lifecycle result it exposes. |
| public Doctor, Units, Context Validation, and Workflow Init facades | These are compatibility aliases/forwarders only; their implementations and policy live in the focused internal packages. |
| deprecated V1 and inactive V2-local compatibility surfaces | They preserve supported historical Go contracts, are explicitly quarantined, and are not selected by active production composition. |

No retained root responsibility owns reusable tool parsing, Doctor diagnostics,
Unit Overview inspection, Context Validation inspection, Workflow Init policy,
workflow static facts, or dispatch HTTP.

## Final `pkg/release` cohesion assessment

The assessment examined declarations, imports, direct I/O, mutation capability,
and call direction rather than accepting filenames as evidence:

- `github_actions_release_use_case.go` contains only the authoritative ordered
  V2 coordinator and its consumer-owned ports; named planning, local-file, Git,
  and workflow effects live in their subject files;
- `v2_release_plan.go` owns only the active immutable `ReleasePlan` derivation;
  no active planning declaration remains in a compatibility file;
- execution-journal model/transitions, store I/O, recovery assessment, and
  dispatch-journal model/store remain separate subjects even where the model is
  necessarily large because its transition validation is authoritative;
- materialization and state transactions each own one bounded local mutation and
  restoration contract; Git staging, commit, tag, push, and query behavior are
  separated by operation subject;
- resume discovery/policy and named continuation operations remain lifecycle
  code because they consume authoritative evidence and reuse active operations;
- V1 planning, execution, adapters, compensation evidence, compensation store,
  compensation policy, and named compensation operations are separate by safety
  responsibility while remaining in the supported V1 lifecycle owner;
- request parsing, typed command outcomes, response mapping, progress facts, and
  terminal rendering are distinct; response code imports no journal, Git, or
  HTTP mutation policy;
- Doctor, Unit Overview, Workflow Init, and Context Validation root files contain
  only direct internal calls, type aliases, constant aliases, or mapping
  forwarders, enforced structurally;
- every `*_compatibility.go` declaration is explicitly inventoried as legacy,
  deprecated, alias, wrapper, or forwarding code; active declarations fail the
  compatibility architecture guard.

The remaining large production files are cohesive lifecycle schemas,
authoritative transition owners, safety stores, or explicit use cases. They are
reviewed by complexity and function-length controls; none is retained merely to
avoid a file move, and no maximum-file-count guard substitutes for responsibility
review.

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

`internal/releasetool` owns common identity, ordered configuration candidates,
and static V1 tool-behavior facts. Each format package owns only its reusable
format-specific facts.

- GoReleaser configuration parsing, invocation classification, and artifact
  contracts are canonical under `internal/releasetool/goreleaser`; GoReleaser
  candidate paths remain with the common owner.
- The JReleaser configuration codec and project-version rewrite are canonical
  under `internal/releasetool/jreleaser`.
- The release-it configuration codec and default configuration are canonical
  under `internal/releasetool/releaseit`.
- `pkg/release/tool/*` retains V1 execution adapters, environment/process/Git
  effects, compensation integration, and compatibility surfaces; it delegates
  reusable configuration facts inward.
- `internal/doctor` alone owns diagnostic severity, messages, remediation, and
  mapping of shared facts into Doctor results.

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

## V2 restoration boundary

The State transaction first writes `.neko/release.state.json` only after the
execution journal has confirmed materialization. At that point zero or more
planned version files may contain their postimages; the State snapshot and every
materialization preimage have already been captured.

Before a Git commit can be attempted, failure handling uses the existing
`StateTransaction` and `MaterializationTransaction` directly:

- a materialization-confirmation failure restores materialized files;
- a State write or State-confirmation failure restores State and materialized
  files;
- a staging pending/store, staging, or staging-confirmation failure restores
  State and materialized files and unstages the known release files;
- failure to persist the pending commit action restores the same local files and
  index because Git commit execution has not started.

Cleanup failures are joined to the original operation error, so callers retain
the original cause and receive every failed restoration detail. The unresolved
execution journal remains durable; restored preimages intentionally conflict
with its postimage evidence, so recovery fails closed rather than treating a
rolled-back attempt as resumable.

Once `create-release-commit` is durably pending and
`GitReleaseCoordinator.Commit` has started, automatic rollback stops. A commit
command error can be ambiguous, and a later HEAD or commit-content verification
error can occur after Git has created the commit. State, materialized files, and
index/commit evidence are therefore preserved. Without a confirmed
`ReleaseCommitSHA`, Resume refuses automatic continuation and a new Release is
blocked by the preserved index and/or unresolved journal. Read-only Plan or
Doctor may observe the persisted filesystem version, but they cannot authorize
mutation or clear the evidence.

After the commit is confirmed, tag preparation/creation and every remote or
dispatch failure also preserve the committed State. Resume reuses the confirmed
commit path; no post-commit rollback exists. Focused real-repository tests assert
State and materialized bytes, index state, HEAD/commit/tag objects, journal
state, recovery assessment, and resume selection at each boundary.

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

`ExecutionPhase`, `MutationTracker`, and `ReleaseTransactionResult` remain
quarantined compatibility types in `v2_local_transaction_compatibility.go` and
are not used by active production composition. `ReleasePlan` and
`BuildReleasePlan` are active planning declarations owned by
`v2_release_plan.go`. There is no operation registry,
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
