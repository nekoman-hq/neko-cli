# Release Plugin Current Architecture

> **Audience:** Contributors changing Release Plugin code, package boundaries, lifecycle behavior, or I/O policy.
>
> **Purpose:** Describe the Release Plugin as it exists: runtime topology, canonical owners, data flow, side effects, invariants, and bounded limitations.

Code, manifests, workflow/configuration files, and focused tests are the final
authority. Public user behavior is documented from the
[Release overview](../../../../docs/release/overview.md); this document owns the
implementation map.

## Runtime topology

```text
Core Cobra command
  -> installed manifest command/flag registration
  -> pkg/plugin.Request
  -> pkg/dispatcher subprocess execution
  -> plugin/release main router
  -> focused command handler or pkg/release lifecycle
  -> pkg/plugin.Response
  -> Core validation and rendering
  -> explicit final process exit
```

Core owns command composition, global presentation flags, subprocess transport,
response validation, rendering, GitHub command-file persistence, and final
process status. The Release executable owns domain commands, data, presentation
declarations, domain failures, and explicit exit requests.

The request carries command, positional arguments, command flags, working
directory, user, and verbose intent. It carries no output format, describe
flag, token, credential, or authorization value. A valid response is rendered
once. Presentation-only declarations stay out of public JSON.

## Composition boundary

`plugin/release/main.go` decodes one request, resolves one explicit repository
root, constructs fresh V1 executor adapters, routes the command, and encodes one
response. It does not implement lifecycle policy, read repository state for a
second time, or render terminal output.

The root `plugin/release` package contains routing and compatibility-facing
composition. Focused command capabilities live in `internal/*` or `pkg/*`.
Doctor, Unit Overview, Context Validation, and Workflow Init root declarations
are thin aliases, wrappers, or forwarders. Their implementation owners are
`internal/doctor`, `internal/unitoverview`, `internal/contextvalidation`, and
`internal/workflowinit`.

Pipeline composition is the deliberate read-only root seam. It reads
authoritative lifecycle evidence, bounded local Git state, recovery policy, and
neutral Doctor verification facts, then supplies immutable facts to
`internal/pipelineinspection`. It owns no lifecycle writer, transition engine,
dispatch client, retry, or presentation policy.

## Package ownership

| Package | Responsibility |
| --- | --- |
| `plugin/release` | Protocol boundary, explicit root, fresh V1 executor composition, routing |
| `internal/releasetool` | Executor identities and format-specific local facts |
| `internal/releaseworkflow` | Workflow identity, dispatch inputs, repository target, consumer-operation facts |
| `internal/githubdispatch` | One bounded workflow-dispatch POST transport and sanitized result |
| `internal/releasesource` | Tolerant read-only V1/V2 source classification |
| `internal/doctor` | Local inspection, opt-in bounded GET verification, diagnostics, presentation |
| `internal/unitoverview` | Read-only V2 unit inventory and presentation |
| `internal/pipelineinspection` | Immutable lifecycle/evidence/verification projection |
| `internal/contextvalidation` | Local dispatched-context validation and presentation |
| `internal/workflowinit` | Canonical workflow planning, preview, and create-only persistence |
| `internal/legacyrequirements` | V1 requirement/configuration checks shared by Validate and compatibility APIs |
| `pkg/config` | V1/V2 models, strict loading, validation, normalization, paths, pair writes/recovery |
| `pkg/release` | Authoritative planning, lifecycle, Git, journals, handoff, Resume/recovery, public facades |
| `pkg/release/tool/*` | V1 process, filesystem, Git, and executor compatibility adapters |
| `pkg/init` | V2 initialization and unit addition |
| `pkg/migrate` | V1-to-V2 migration plan, persistence, and recovery |
| `pkg/evidence` | Redacted evidence query and guarded completed-evidence archival |
| `pkg/validate` | Read-only full repository validation |
| `pkg/history`, `pkg/contributors` | Read-only unit-scoped Git queries |
| `pkg/pluginindex` | Deterministic index bytes, check/output policy, optional atomic persistence |
| `pkg/workspace` | Explicit repository roots and bounded cwd compatibility helpers |

The more concise dependency view is in
[Package Ownership](package-ownership.md).

## Source and model authority

Source resolution selects one authority:

- V1: nearest `.release.neko.json` when root V2 configuration is absent;
- V2: root `.neko/release.config.json` plus `.neko/release.state.json`;
- mixed root V1/V2 or incomplete V2: error.

`config.ReleaseRepository` is the normalized repository model.
`config.ReleaseUnit` is the unit ownership boundary. V1 becomes one virtual
`default` unit; V2 retains configured units. `ReleaseExecutionContext` binds
root, unit, current/next version, tag, release kind, executor, delivery,
workflow, source format, and dry-run intent for one lifecycle.

`ReleasePlan` is the read-only calculation result. Materialization, state, Git,
execution journal, dispatch journal, and recovery each use distinct typed
models. No general state machine or provider abstraction mediates them.

## Command capability boundaries

### Configuration and migration

`init` and `unit-add` parse requests in the command layer, build a complete V2
pair, validate it, and persist through config-owned atomic pair handling. They
do not create workflows, manifests, source directories, or executor config.

`migrate` has a separate plan/persist/recovery lifecycle. It accepts only a
root V1 source, writes one V2 `default` unit, and archives V1. Mixed sources,
nested sources, conflicting targets, or untrusted recovery evidence fail
closed.

### Read-only queries

`validate`, `history`, `contributors`, `units`, `plan`, default Doctor, default
Pipeline, Evidence query, and Context Validation are read-only. Their handlers
receive readers or immutable facts, not lifecycle mutation ports.

Evidence Archive is a different command and an explicit guarded local
mutation. Plugin Index raw output and `--check` are read-only; only
`--output-file` enables its atomic persistence path.

### Doctor

`internal/doctor` owns workflow parsing, local repository inspection,
credentials-reference analysis, GoReleaser/installer/publication checks,
diagnostics, readiness, and presentation. Default Doctor constructs no GitHub
client, resolves no token, invokes no Git command, and performs no write.

Explicit `--verify-remote` uses one bounded GitHub GET reader with lazy optional
read-token resolution. The endpoint set is closed. It never dispatches,
publishes, uploads, changes repository settings, requests secret values, or
repairs local files.

### Unit Overview

`internal/unitoverview` reads the strict V2 pair and emits declared unit facts.
It does not calculate the next version, inspect Git tags, read journals, or
call Doctor. Selection is presentation filtering, not lifecycle execution.

### Pipeline inspection

Pipeline is a read-only projection, not a lifecycle engine or state machine.
Root composition gathers the configured operation list, execution/dispatch
journal evidence, bounded local Git evidence, recovery assessment, and neutral
verification facts. `internal/pipelineinspection` converts those immutable
inputs into configured, runtime, verification, progress, resume-eligibility,
and retry-safety views.

Remote verification does not complete a lifecycle stage, change recovery
classification, authorize retry, or prove workflow/publication success.

### Context validation

`internal/contextvalidation` validates the dispatched unit, version, tag,
release SHA, checked-out `HEAD`, and peeled local tag target. It reads local Git
without fetch, token, network, or mutation and emits the stable GitHub output
contract.

#### GitHub Actions workflow scaffolding

`internal/workflowinit` owns request parsing, configured-target resolution,
canonical workflow rendering, comparison, presentation, and atomic no-clobber
creation. `githubWorkflowGenerationPlanner` returns an immutable plan. Dry-run
and conflict paths receive no writer. The write path can create one missing
target, accept byte-identical content, or fail on differing content; it cannot
update or merge an existing workflow.

## Release lifecycle ownership

`pkg/release` keeps the responsibilities that share release identity and safety
evidence: source selection, V1 planning/execution, V2 context and plan,
materialization/state ordering, targeted Git coordination, journal policy,
handoff classification, and Resume/recovery. Splitting these policies across a
generic framework would introduce competing lifecycle authority.

### V1 compatibility execution

V1 selects a fresh GoReleaser, JReleaser, or release-it adapter. Compatibility
code retains established requirement checks, version behavior, executor
invocation, environment mapping, Git/GitHub effects, compensation evidence,
and recovery messages. V1 does not use V2 central state or V2 workflow journals.

### V2 planning

V2 planning resolves one unit, calculates the next SemVer and exact unit tag,
validates executor requirements at the unit root, plans required file
materialization, calculates known release files, and describes Git/workflow
handoff. `plan` and release dry-run do not resolve tokens or perform writes,
Git mutations, executor invocation, network requests, or journal creation.

#### V2 GitHub Actions execution

The non-dry-run operation order is fixed:

```text
token preflight
materialization plan
Git and unresolved-journal preflight
execution journal preparation
materialization apply
state write
targeted stage
release commit
unit tag
dispatch journal preparation
commit push
tag push
workflow dispatch
accepted-handoff confirmation
```

Token preflight precedes mutation. The release commit contains only state and
materialized files marked as required. Commit push precedes tag push. Dispatch
uses the existing unit tag and four inputs: unit, version, tag, release SHA.
An accepted response records `handoff-ready`; it does not mean publication
completed.

V2 does not start the configured executor locally. The consumer workflow owns
build and publication from the pushed identity.

### Resume and recovery

The execution journal records intent before local mutation and confirms
monotonic boundaries. The dispatch journal records one request through
prepared, request-started, accepted, rejected, or unknown. Both live below the
Git common directory and contain hashes/metadata rather than file bytes or
credentials.

Recovery assesses journals, local `HEAD`, tag, known files, state metadata,
index state, and recorded push markers. It makes no remote query. Resume
continues only an exact unresolved execution and blocks ambiguous push or
dispatch outcomes. It never creates another release intent or acts as a
standalone retry command.

## I/O, network, token, and mutation map

| Capability | Filesystem/Git reads | Writes or mutation | Network/token |
| --- | --- | --- | --- |
| Help and manifest overview | Manifest only | none | none |
| Validate, Units, Plan, History, Contributors | Repository/Git reads as required | none | none |
| Doctor default | Config, state, workflow, executor/installer files | none; no Git command | none |
| Doctor `--verify-remote` | Same local reads | none | bounded GitHub GET; optional lazy read token |
| Pipeline default | Config/state, journals, bounded local Git facts | none | none |
| Pipeline `--verify-remote` | Same local reads | none | Doctor-owned bounded GET boundary |
| Context Validation | Config/state and local Git/tag facts | none | none |
| Workflow Init dry-run/conflict | Config/state and target comparison | none | none |
| Workflow Init create | Same reads | atomic create of one missing workflow | none |
| Plugin Index raw/check | Config/state/manifests | none | none |
| Plugin Index output file | Same reads | explicit atomic artifact write | none |
| Evidence | Journal reads | none | none |
| Evidence Archive | Eligible journal reads | guarded local archive mutation | none |
| V2 release dry-run | Config/state, requirements, local Git/tag planning | none | none |
| V2 GitHub Actions release | Preflight reads | materialize, state, journal, stage, commit, tag, push, dispatch | Git remote plus one GitHub POST; required token |
| Resume dry-run | Journal/local Git assessment | none | none |
| Resume | Same assessment | exact eligible continuation only | only when the recorded operation requires it |

## Output and exit boundary

Handlers return `plugin.Response` with stable machine data, safe human
presentation declarations, optional logs, and an explicit exit request.
Release commands use exits `0` and `1`. Successful negative observations such
as Pipeline blocked, Doctor warning-only, or unsafe Resume dry-run remain exit
`0`; invalid evidence, not-ready Doctor, missing Resume journal, and execution
failures use exit `1`.

Core owns transport/decode/render failures. `--describe` changes human
visibility and `--verbose` exposes safe chronological phases; neither flag
adds token, network, filesystem, or mutation reachability. JSON stays stable
machine data. Human output uses repository-relative paths or safe labels.

## Architecture invariants

- One repository root is explicit through command execution.
- No command handler calls another command handler to reuse policy.
- Read-only handlers receive no writer or mutation port.
- Response mapping reads no Git, HTTP, journal store, token, or recovery policy.
- Terminal width, TTY, and color dependencies remain outside domain/application logic.
- Compatibility files contain only classified legacy/deprecated declarations, aliases, wrappers, or forwarders.
- Production composition does not use the mutable V1 registry.
- No generic lifecycle engine, second state machine, stage registry, provider hierarchy, dependency bag, workflow DSL, or second renderer exists.
- Unknown remote effects fail closed and are not retried blindly.
- Tests use temporary roots and self-owned loopback listeners; they do not read real credentials or contact real external APIs.

## Current bounded limitations

- Executable V2 local delivery is unsupported.
- GitHub dispatch accepts GitHub.com remotes, not GitHub Enterprise Server.
- No public standalone dispatch or retry command exists.
- Dispatch acceptance is not durable workflow-run or publication-completion state.
- Doctor cannot prove runtime credentials, exact dispatch authorization, runner outcome, or publication success without performing forbidden mutation.
- Unknown push/dispatch results require operator inspection.
- V1 may require manual recovery when executor or remote effects are uncertain.
- Retained cwd, registry, service, tool, and executor surfaces remain compatibility-only until their removal gates are satisfied.

## Related architecture controls

- [Package ownership](package-ownership.md)
- [Architecture decisions](architecture-decisions.md)
- [Maintainability policy](maintainability-policy.md)
- [Compatibility notes](compatibility-notes.md)
- [V1 compatibility policy](v1-compatibility-policy.md)

## Historical context

Completed and superseded implementation rationale is isolated in the
[numbered Release documentation history](../history/README.md). History is not
required to understand or change the product and does not override this
current-state description.
