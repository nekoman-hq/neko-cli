# Release Plugin Architecture Evolution

## Purpose and scope

This document records durable Release Plugin architecture decisions after the completed behavior-preserving refactor. It is not a project-management roadmap and does not track task status.

Detailed runtime behavior and disk contracts live in [current-state.md](current-state.md). Dependency direction, debt ranking, and compatibility inventory live in [post-refactor-review.md](post-refactor-review.md). The closed refactor history lives in [refactor-history.md](refactor-history.md). V1 compatibility support decisions live in [v1-compatibility-policy.md](v1-compatibility-policy.md).

Every future implementation remains subject to `plugin/release/RULES.md`.

## Completed architecture records

### V1 compensation interruption safety

Active V1 release execution writes strict private evidence before local mutation and persists pending intent before each supported compensation effect. Supported repeatable local work can continue on the next invocation; uncertain remote work, unsupported evidence, corrupt evidence, pending non-repeatable operations, and executor ambiguity fail closed with manual-recovery guidance.

The active V1 GitHub Release client is injected, root-aware, bounded by timeout and response size, verifies deletion through an observable not-found result, and unwraps its token only while constructing the HTTP request.

### V2 pair and migration crash recovery

Init, unit-add, and migration share the V2 config/state pair persister. The persister creates durable pair-recovery evidence before replacing either file, records pending and confirmed target replacement, verifies exact bytes, validates the complete intended pair, and either closes already-complete evidence or restores exact prior bytes, modes, and absence when supported.

Migration owns a separate worktree journal. It persists the complete V2 target pair before archiving the V1 source, verifies exact target bytes and strict V2 validity before the archive, verifies the byte-identical backup after archive, and refuses owner-ambiguous pair evidence.

These guarantees are crash-recoverable, not cross-file atomic. Corrupt, externally edited, unsupported, hash-conflicting, mode-conflicting, or owner-ambiguous evidence requires manual recovery.

### Evidence inspection and archival

`neko release evidence` is read-only and token-free. It reports redacted typed summaries and diagnostics for release execution journals, dispatch journals, migration journals, V1 compensation evidence, and V2 pair-recovery evidence. Its query receives canonical Git-common-dir locations through `ResolveReleaseEvidenceLocations`; it never receives the mutation-capable execution, dispatch, or V1 compensation stores.

`neko release evidence-archive` supports only completed release-execution, completed V1 compensation, and completed V2 pair-recovery evidence. It requires family, identity, current digest, and explicit confirmation; then it re-observes the evidence, writes and verifies an exact private archive, and removes the completed source. Dispatch and migration evidence remain inspect-only.

No repair command, remote reconciliation, unsafe completion inference, or schema migration exists without a separate evidence-specific design.

### Responsive Evidence presentation

Evidence summaries opt in to Core's small typed `presentation.Table` transport declaration. The Release Plugin owns the ordered labels and essential/optional meaning; Core owns only actual-writer width detection, Unicode- and ANSI-aware visible-cell measurement, optional-column fitting, table-versus-vertical choice, wrapping, and truthful `wide` behavior. Width-unavailable output uses deterministic vertical records. Commands without the declaration retain legacy rendering.

Presentation metadata does not alter `Data` and is excluded from public JSON and raw JSON. Existing unfiltered Evidence JSON retains the complete `items`, typed `evidence`, and `diagnostics` contract. Full forensic values are not summary columns and remain available through JSON and read-only detail inspection.

Identity detail accepts only 8-64 lowercase hexadecimal characters, applies family and unit filters before matching, and requires exactly one result. It neither normalizes uppercase nor chooses a first match. Prefix matching is inspection-only: Evidence archival still requires the exact 64-character identity, current digest, supported completed lifecycle, and explicit confirmation. Classification, resume, automatic-continuation, manual-recovery, lifecycle, redaction, ordering, and secret policy remain Evidence-owned and unchanged.

### V1 compatibility policy and retired release paths

The V1 compatibility policy documents which exported surfaces are kept, deprecated, deferred, removed, or still dependent on downstream evidence. Production composition uses explicit V1 executor values and bypasses mutable registry lookup.

Retired internal bridges, inactive response helpers, one-call coordinator convenience paths, and unused raw Git helpers are guarded against reintroduction. Retained compatibility surfaces remain direct delegates or deliberately bounded facades; new production code must not depend on mutable registries, version-evidence globals, inactive V2-local scaffolding, or raw destructive Git helpers.

### Typed release progress reporting

Active V2 progress reporting is behind the `ReleaseProgress` port. Application and operation code emits safe typed `ReleaseProgressEvent` values. Terminal rendering lives in adapter files, Git diagnostics are separate, and command responses remain owned by response mappers.

The reporter is synchronous and infallible. It cannot choose release policy, retry behavior, journal state, Git or network effects, command success, or response schema. Unknown events render nothing.

### Explicit repository-root composition

The plugin executable resolves one `workspace.RepositoryRoot` at the command boundary and routes production commands through explicit-root handlers. Production command routing no longer mutates process cwd.

Explicit-root entry points exist for init, unit-add, release, resume, validate, CI release-context validation, GitHub workflow scaffolding, history, contributors, evidence, evidence archive, migration, and plugin-index. Existing cwd-based facades remain compatibility surfaces. Production migration keeps its legacy Git-root discovery facade to preserve nested-V1 behavior, while embedders can call the explicit-root migration handler.

Isolation tests prove two in-process repositories can be validated and indexed without changing process cwd or leaking root-specific data. CI context validation additionally resolves nested invocations to the explicit root and keeps both repositories' Git/config facts isolated.

### Typed dispatched-context validation

`neko release ci-validate-context` now owns the reusable local validation
boundary between four dispatched strings and one `ValidatedReleaseContext`.
The use case depends only on strict V2 source reads and five local Git facts:
object format, object type, HEAD commit, tag presence, and peeled tag commit.
Canonical unit resolution, state version, and `TagSpec` policy remain the
authorities; no parallel CI policy model was introduced.

The application boundary has no token, network, dispatch, runner, executor,
persister, materializer, journal/evidence writer, recovery mutator, or response
transport capability. Command mapping provides deterministic typed JSON and
ordered human properties. Domain-neutral Core transport metadata declares ten
ordered scalar GitHub outputs; Core alone owns explicit command-file selection,
safe multiline encoding, and output errors. This boundary is reused by the
canonical generated workflow and by the local integration doctor.

### GitHub Actions workflow scaffolding

`neko release github-workflow-init` is a focused GitHub-Actions-only command,
not a provider registry or generic workflow DSL. Untyped flags stop in the
command parser. Target resolution consumes only an existing structurally valid
V2 config/state pair and selects exactly one configured workflow through the
unique default, `--unit`, or exact `--path` rules. Shared paths remain one
workflow scope; distinct paths never trigger implicit multi-file generation.

One typed canonical specification and focused template own workflow contract
version `1`. The documentation contract test compares the Golden Path snippet
byte-for-byte with the renderer. The plan is read-only and classifies create,
unchanged, or conflict before the execution use case receives a narrow output
creator. Missing output is published target-locally with atomic no-clobber
semantics and mode `0644`; identical output is never rewritten; different or
older content is preserved as a conflict. No managed update, force overwrite,
partial YAML merge, or arbitrary consumer command exists.

The generator has no Git mutator, network client, token resolver, dispatcher,
journal/evidence writer, state persister, release runner, or provider
credential capability. Preview uses the same plan and renderer, performs no
write, and maps summary plus complete YAML through transport-only preformatted
human output while public JSON retains stable typed data. Existing manual
workflows are compatible because scaffolding is opt-in and create-only.

The generated file owns only Actions integration: exact four-input dispatch,
minimal read permission, safe concurrency, exact-SHA checkout with full tags,
pinned CLI/plugin installation, canonical context validation, and a
deliberately failing consumer extension point. Builds, publication,
credentials, GitHub Release creation, release notes, and deployment remain
consumer-owned.

### Generated-output path policy

Generated-output path policy is explicit per output family rather than shared through a universal path manager.

`plugin-index --output` owns the generated integration artifact path. Relative paths are clean repository-root-relative paths resolved from the explicit `workspace.RepositoryRoot`, independent of process cwd, CLI nesting, or embedder mode. Explicit absolute paths remain supported for CI and temporary artifact targets, including the plugin release workflows' runner-temp output. Repository-contained targets cannot overwrite release config/state, recovery or migration evidence, legacy release config/backup, Git internals, or the plugin manifest inputs present in the generated index. Existing target directories and target symlinks are rejected before writing. Existing repository-relative parent symlinks are allowed only when their physical target remains inside the resolved repository root. Missing parents are created only by the persister, with mode `0755`; new files use `0644`, existing file mode is preserved, and replacement remains target-local atomic.

GitHub workflow scaffolding has a deliberately narrower create-only path
policy: the output must be an exact V2-configured direct child of
`.github/workflows/` with lowercase `.yml` or `.yaml`; absolute, traversal,
nested, protected, unsupported, and symlink-escaping targets are rejected.
Missing parents use `0755`, new files use `0644`, and target-local atomic
publication cannot replace a target that appears after planning.

User-declared materialized release files are owned by V2 release planning and materialization. Plugin manifest paths come only from validated plugin unit metadata, and JReleaser materialization owns only `jreleaser.yml` below the unit root. V2 `workingDirectory` must be lexically and physically inside the resolved repository root, materialized target symlinks are rejected, and `KnownReleaseFiles` keeps staging identity repository-relative to the same root. `MaterializationTransaction` continues to snapshot and restore exact bytes/modes before commit uncertainty.

Operational state remains separately owned by its existing components: config/state/pair recovery by config persistence, execution and dispatch journals below the Git common dir, V1 compensation evidence, migration journals/backups, and evidence archives. These files are not folded into generated-output handling merely because they are files.

Check, render, and planning modes remain write-free. The plugin-index JSON schema/order and command response schema are unchanged.

### Release plan inspection

`neko release plan` provides token-free, read-only facts about selected source/unit, current and next version, requested version change, tag, planned materialization, known release files, local blockers, and explicit limitations without starting execution.

The command has its own typed request, read-only use case, typed result, command-boundary mapper, and manifest entry. It uses canonical release source selection, V1 pure planning through `PlanV1Release`, V2 context construction through `BuildV2ReleaseExecutionContext`, and the shared V2 planning facts used by dry-run. It does not run release execution in a no-op mode.

The inspection use case receives no token resolver, remote client, journal writer, evidence writer, Git mutator, executor runner, state persister, materialization transaction, dispatcher, or recovery capability. It does not inspect execution journals, dispatch journals, recovery evidence, remote tags, remote releases, workflow runs, token availability, provider authorization, or publication readiness.

The command-boundary mapper keeps the established machine-readable
`data.items` projection and complete typed limitation records. Its
transport-only property declaration gives every limitation a Release-owned
human-readable title and direct presentation value. Core remains
Release-neutral: it owns actual-width resolution, bounded label/value layout,
ANSI/Unicode visible-cell measurement, wrapping, continuation indentation,
separator bounds, and deterministic vertical fallback. Public JSON, raw JSON,
GitHub output, planning facts, and read-only behavior are unchanged.

Existing `patch`, `minor`, and `major --dry-run` behavior remains a separate compatibility surface. Dry-run keeps its established response order and progress output while sharing V2 planning facts below the presentation boundary.

V1 inspection is supported as a local planning subset: it reports the legacy source, virtual `default` unit, current and next version, tag, executor, planned `.release.neko.json` materialization, and limitations. It does not use the old cwd-based latest-tag evidence facade.

### Release V2 integration doctor

`neko release doctor` is a read-only GitHub Actions integration inspection
rather than an execution mode, validation alias, diagnostics framework, unit
overview, or pipeline inspector. The flat command accepts optional `--unit`
and explicit `--verify-remote`, and supports human or JSON output. Without the
remote flag it remains offline and token-free.

The executable uses a doctor-specific inspection-root resolver so incomplete
or conflicting source files can be diagnosed. The typed handler parses one
request, invokes one use case, and maps one result. The use case composes a
strict local source reader, canonical config/state validation, deterministic
unit/workflow scope, a physically confined workflow reader, focused YAML-node
inspectors, and typed diagnostic aggregation. Shared workflow paths retain all
configured unit identities even when one unit is selected.

The workflow checks reuse the centralized four-input dispatch contract and
the canonical workflow renderer. Exact generated bytes are recognized, while
the deliberately failing consumer placeholder remains an error until replaced.
Manual workflows are supported through structural equivalence; optional extra
inputs and unrelated verification triggers remain allowed. Supported repository
workflows additionally expose typed local facts for consumer structure, focused
GoReleaser configuration, installation/artifact identity, credential wiring,
and publication/registry identity. Unknown shapes remain `unsupported` rather
than being guessed.

The same small fact model represents three explicit boundaries: remote workflow
identity and repository-variable values are `remote`; exact dispatch
authorization is `mutation_required`. Consumer execution, installation,
credential issuance, and publication acceptance retain only genuine runtime or
remote uncertainty. Every limitation belongs to a focused predicate; there is
no unconditional limitation registry or loop.

The explicit remote decision adds one optional inspector port to the existing
use case. Production composition supplies a package-private GitHub reader and
the existing typed dispatch-token resolver at the command boundary; the use
case invokes neither unless `VerifyRemote` is true. The reader is deliberately
not a provider abstraction or general REST framework: it owns only exact
repository, default-branch workflow, workflow-state, recognized-variable,
referenced custom-secret-name, Actions-policy, exact release/tag/asset, and
exact durable workflow-run GET operations. There are no collection/latest/
fuzzy discovery operations, no redirects, no automatic retries, a 12-second
timeout, and a 1 MiB response cap. Because Doctor currently owns no durable run
ID, the run operation is not invoked.

Repository identity is anonymous-first. Only an ambiguous missing or
unauthorized repository identity can trigger one authenticated identity lookup;
protected Actions metadata then reuses the same token resolved at most once.
Public workflow, release, and tag observations remain anonymous. The result
never contains a token, authorization header, response body, secret value, or
arbitrary variable/secret collection. Exact remote workflow bytes are compared
with the repository-confined local bytes, and release/tag/asset identities come
only from focused locally verified installation and publication contracts.

The additive remote summary closes explicit observation as `not_requested`,
`complete`, `partial`, or `unavailable`. Fact states additionally distinguish
`not_attempted`, `unavailable`, `unauthorized`, and `rate_limited`. Definite
missing, mismatched, disabled, or invalid configuration is an error; access and
service uncertainty remains unresolved evidence. Successful remote facts
replace or narrow their matching offline limitations without claiming future
runner success, credential-value validity, publication acceptance, or exact
dispatch authorization.

The result closes severity and readiness policy without a generic state
machine: any error is `not_ready`; warnings without errors are
`ready_with_warnings`; recommendations and not-verifiable facts alone are
`ready`. Facts have stable subject/category/state/evidence/references and
optional unit/workflow/limitation-class fields. Diagnostics have stable severity/scope/unit/workflow/code/message/
remediation fields and deterministic ordering. Human-readable output is summary-first:
a titled responsive readiness/count summary precedes a titled compact
severity/code index, then complete responsive diagnostic records headed by
severity and code. The index admits optional target and scope fields by width;
complete workflow identity, message, and remediation remain in the ordered
details. The response mapper assigns closed semantic roles without emitting
ANSI or inspecting terminal capability. Core maps those roles only for
interactive terminal human output; a non-empty `NO_COLOR` and non-terminal
destinations disable color. JSON, raw JSON, and GitHub output keep the stable
typed result projection, exclude every presentation-only value, and remain
ANSI-free. Styling cannot affect diagnostic ordering, readiness, or exit
policy.

The existing response fields could not express a property summary, table, and
separate responsive detail values in one result because human renderers were
exclusive and responsive tables could only read a machine-data list. Core
therefore extends `presentation.Table` with optional transport-only `rows`, `details`,
and neutral titles, and adds bounded semantic roles/headings to the existing
presentation declaration types. They reuse row maps and `presentation.Properties`; nil and
empty zero values are omitted and preserve every established table response.
Core composes only generic property/table/property rendering and contains no
Doctor terminology, diagnostic renderer, document model, layout DSL, theme
engine, registry, provider abstraction, or state machine.
Public JSON and raw JSON already exclude the complete `presentation.Table` declaration.

No writer, Git command or mutator, dispatcher, journal store, Evidence writer,
release runner, executor, registry, workflow DSL, provider abstraction, generic
remote framework, or repair capability reaches the use case. Default Doctor
does not invoke its optional network/token port. Explicit Doctor receives only
the focused GET reader and lazy token resolver; it cannot dispatch, publish,
upload, write settings, execute a process, or mutate local or remote state. The
only recovery-related read is the existing V2 pair-recovery readiness check
required to decide whether local config/state facts are trustworthy.

### Release V2 unit overview

`neko release units` is the active local Release V2 inventory command. It is a
flat command with no selector or verification modes. Its focused flow is typed
request parsing, explicit inspection-root resolution, strict local V2 source
loading, deterministic unit-row and issue derivation, summary calculation, and
command-boundary response mapping.

The shared `filesystemLocalV2SourceReader` owns only V1/V2 presence, canonical
strict config/state decoding, and the existing pair-recovery readiness check.
The Doctor source facade delegates to that reader, but the overview never calls
Doctor workflow orchestration, workflow readers, or YAML inspectors. Valid
pairs use canonical config/state structural validation and normalization.
Parseable schema-2 pairs with parity or unit validation findings use the union
of config and state identities so config-only, state-only, and invalid units
remain visible.

Current versions come only from state through canonical SemVer validation.
Invalid raw versions remain explicitly configured values without becoming
canonical versions. Tag prefix validation and the version-independent tag
shape reuse `TagSpec`; the configured current tag is derived only when the
state version validates and never claims Git evidence. Executor, delivery, and
workflow-path validation remain config-owned. The overview neither plans a
next version nor opens configured workflow files.

Alignment is the closed derived set `aligned`, `config_only`, `state_only`, and
`invalid`; overall status is `valid`, `has_issues`, or `source_invalid`.
Expected findings are structured nil-Go-error responses, and only `valid`
requests exit `0`. Human-readable output opts into Core's responsive `presentation.Table` with
Unit, Version, and Status as essential columns. Machine output contains stable
typed summary, ordered rows, ordered issues, lexical workflow paths, and an
optional source issue; transport-only presentation metadata remains outside
the data contract.

No workflow parser, Doctor workflow inspector, Git reader or mutator, network
or GitHub client, token resolver, file writer, state persister, build-system
reader, planner, release executor, dispatcher, journal/Evidence store, state
machine, registry, provider abstraction, generic inventory framework, or
repair capability reaches the use case. Runtime and static guards preserve
source/workflow bytes, modes and mtimes, cwd/environment isolation, sequential
repository isolation, JSON isolation, and Doctor behavior.

### V2 local delivery evaluation

V2 local delivery is deliberately unsupported for executable V2 releases. `github-actions` is the only supported V2 delivery mode for committed V2 release units.

The decision rejects local JReleaser activation because the existing JReleaser executor is not a publish-only adapter. Its local `full-release` flow can create tags, publish releases, and observe remote services from inside the executor process. Neko CLI cannot prove the external publication result after interruption, cannot safely retry without remote-state inference, and cannot compensate remote publication with the guarantees required by the V2 journal and recovery model. GoReleaser and release-it have the same or stronger ownership conflict for local V2 execution: the current local executor contracts can own publication and, for release-it, commit/tag/push as well.

For V2, local delivery means "the executor process would be launched on the current machine"; it does not mean offline or no remote side effects. Because no supported executor currently exposes a safe publish-only boundary, V2 config validation rejects `delivery: "local"` and missing delivery defaults no longer imply local execution. Init and unit-add must present only `github-actions` for executable V2 releases, require a workflow path, and keep V1 local compatibility unchanged.

Planning and dry-run remain token-free and non-mutating. Existing invalid V2 configs containing `local` are rejected before planning or execution with a clear unsupported-delivery failure. The active V2 GitHub Actions path remains unchanged: Neko CLI owns version materialization, V2 state, the release commit, the unit tag, commit push, tag push, execution journals, dispatch journals, and workflow dispatch; the workflow owns build and publication from the pushed tag.

The retained inactive V2 local transaction scaffold is no longer a future production path. Any public compatibility wrapper that remains must refuse local execution directly and must not retain private preparation, rollback, or executor-invocation scaffolding. A future local delivery feature would need a fresh executor-specific design with publish-only execution, typed evidence, explicit crash windows, and compensation limits.

## Pending architecture decisions

Release-pipeline inspection remains a separate future capability. It must not
be inferred from or folded into the unit overview or integration doctor.

Release V2 bootstrap planning is product capability planning, not a reopened
architecture refactor stage. The current product boundary for GitHub Actions
bootstrap, CI release-context validation, build-system adapters, and
consumer-owned publication is maintained in
[Release V2 Bootstrap Product Boundary](../../../../docs/release/bootstrap-product-boundary.md).

## Preserved invariants

- No blind retry for ambiguous push or uncertain workflow dispatch.
- No remote-state inference that converts observation into proof of safe retry.
- No generic workflow pipeline, universal state-machine engine, dependency bag, or shared journal repository.
- No V1 removal without fresh consumer evidence, replacement paths, deprecation policy, and a separately approved removal.
- No V2 local execution without executor-specific ownership and recovery proof.
- No journal repair or schema migration without an inspection-first evidence design.
- No claim of full concurrent safety for retained cwd compatibility facades.
- No repository-relative generated output may infer its base from process cwd.
- No generated integration artifact may overwrite protected release state, evidence, Git internals, or plugin manifest inputs.

## Implementation guidance

Future work should use descriptive capability names in code, tests, documentation, and commits. Characterization remains the first step for behavior-sensitive changes, but permanent artifacts must describe the behavior or architectural boundary rather than internal project-management labels.
