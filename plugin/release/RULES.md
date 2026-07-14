# Release Plugin Engineering Rules

## 1. Scope and authority

These rules apply to all work under `plugin/release`.

Before modifying Release Plugin files:

1. Read `plugin/release/AGENTS.md` and this file.
2. Read `plugin/release/docs/architecture/current-state.md` for the affected flow.
3. Read `plugin/release/docs/architecture/refactor-plan.md` when implementing a planned boundary.
4. Inspect the current code and tests. Plans and documents do not override current behavior.
5. Inspect `git status --short --branch` and preserve all pre-existing work.

Correctness and release safety take precedence over architectural purity. A behavior-preserving refactor must preserve observed release semantics, including awkward legacy behavior, unless the task explicitly authorizes a behavior change.

Keep changes inside `plugin/release` unless inspecting or changing a shared contract is explicitly required. Do not use a Release Plugin task as a reason for a repository-wide rewrite. Prefer incremental migration of one flow or boundary at a time.

New features must use improved boundaries that already exist for the affected flow. Do not add another branch to a known orchestration hotspot when the requested work can first establish a small typed use case or adapter seam.

## 2. Current compatibility contract

Treat the following as stable until a task explicitly changes them and updates tests/documentation:

- public commands and flags in `plugin/release/manifest.json`;
- stdin `plugin.Request` and stdout `plugin.Response` behavior;
- response status, metadata command, renderer hint, data keys, item order, error code, error details, and exit behavior;
- V1 compatibility behavior;
- V2 config/state ownership and unit selection;
- version calculation and tag formatting;
- release commit message and exact file allowlist;
- lightweight tag target;
- commit push before tag push;
- execution- and dispatch-journal schemas, identities, locations, permissions, states, transitions, and pending-action semantics;
- dry-run and recovery read-only guarantees;
- uncertain-operation and resume restrictions;
- `GITHUB_TOKEN` handling and secret non-disclosure;
- V2 local execution remaining blocked unless activation is explicitly requested.

If a change may affect one of these contracts, add characterization tests before extraction. Do not infer permission to “clean up” the behavior.

## 3. Command boundaries

A command handler should have one presentation-boundary responsibility:

1. parse command input;
2. reject invalid request shape or flag types;
3. construct one typed application request;
4. invoke one use case;
5. map its typed result or classified failure to the established plugin response.

Handlers may select a use case by command name or release kind. They must not directly coordinate unrelated Git, filesystem, config/state mutation, journal transitions, token resolution, network dispatch, and response construction.

The handler must not:

- call `os.Chdir`, `exec.Command`, `http.Client`, or unsafe persistence directly;
- calculate a release version independently of the canonical planner;
- construct or transition journals;
- decide rollback or retry behavior;
- format terminal logs inside business decisions;
- return an unclassified Go error when the command has an established structured error response;
- silently treat a wrong flag type as the default when the request contract requires that flag.

Keep manifest metadata and route registration synchronized. A public command or flag change requires manifest, routing, command-contract tests, and user documentation in the same change.

## 4. Application orchestration

Each user-visible use case must have explicit input and output types. Inputs contain only the facts required to execute the use case; outputs contain domain/application results, not `plugin.Response` or terminal formatting.

Safety-critical ordering must be visible in the use case through clearly named steps. For the active V2 GitHub Actions flow, preserve named operations equivalent to:

1. resolve token before mutation;
2. plan materialization and known release files;
3. validate Git and unresolved-journal preflight;
4. prepare the execution journal;
5. record pending action, perform materialization, confirm phase;
6. record pending action, write selected-unit state, confirm phase;
7. record pending action, stage the exact allowlist, confirm phase;
8. record pending action, create and verify the release commit, confirm phase;
9. record pending action, create and verify the unit tag, confirm phase;
10. prepare the dispatch journal and confirm its execution-journal phase;
11. record pending action, push commit, confirm phase;
12. record pending action, push tag, confirm phase;
13. record dispatch request start before HTTP and classify the result;
14. confirm handoff only for an accepted dispatch.

Do not hide safety order in generic middleware, callback chains, deferred functions, or a broad transaction helper whose call site does not show the steps.

An application use case must not create `plugin.Response`, render tables, color text, or call terminal-formatting helpers. Presentation mapping belongs at the command boundary.

All orchestration dependencies that perform I/O, observe time/environment, or can fail at a release boundary must be replaceable in tests. Prefer one dependency interface per cohesive adapter, not one interface per function and not one all-purpose service.

Do not use boolean parameters when they select materially different workflows. Replace flags such as “load only,” “already pushed,” or “perform dispatch” with typed requests, named operations, or separate functions.

## 5. Domain and state modeling

Reuse the canonical models before creating new ones:

- `config.ReleaseRepository` and `config.ReleaseUnit` for normalized repository facts;
- `config.V2ReleaseConfig` and `config.V2ReleaseState` for disk ownership;
- `config.TagSpec` for tag formatting and parsing;
- `ReleaseExecutionContext` or its deliberately evolved replacement for execution facts;
- `MaterializationPlan` and `KnownReleaseFiles` for the exact file set;
- journal state and identity types for persisted release progress.

Do not introduce a parallel release metadata model that duplicates unit ID, version, tag, executor, delivery, workflow, or plugin manifest facts without an explicit conversion boundary and a documented reason.

Use typed states, phases, pending actions, outcomes, and result categories where invalid combinations would be unsafe. State transitions must be explicit and validated. Do not add undocumented string states or compare raw state strings throughout nested conditionals.

Reject invalid states explicitly. Do not “repair” a corrupt or conflicting journal by guessing, skipping a phase, clearing a pending action, deleting evidence, or overwriting immutable fields.

Do not spread a state machine across handlers, adapters, and response mappers. The state model owns allowed transitions; the use case owns when to request them; the store owns persistence.

Small value types and named functions are preferred over speculative domain hierarchies. Do not introduce generic Clean Architecture, hexagonal, or domain-driven package trees unless a concrete Release Plugin dependency or test seam requires them.

## 6. Config and state ownership

For V2:

- `.neko/release.config.json` is architecture/configuration.
- `.neko/release.state.json` is the authoritative mutable unit-version state.
- a release updates only the selected unit's state entry;
- a tag is derived from the selected unit's validated `TagSpec` and next version;
- release execution does not silently rewrite config;
- configured plugin metadata is the canonical source for plugin manifest selection; do not extend hard-coded unit maps.

Any operation that writes config and state as a pair must define and test its pair-failure behavior. Single-file atomic replacement alone does not make a two-file operation atomic.

For V1, preserve `.release.neko.json` compatibility until a task explicitly removes it. Do not route V1 through V2 writes as a refactor shortcut.

## 7. Side effects and adapters

### Git

Use a Release Plugin Git adapter with replaceable command execution. Git methods must reveal intent, such as preflight, stage exact files, create verified release commit, create verified unit tag, push exact commit, or push exact tag.

Do not pass arbitrary Git argument slices across application boundaries. Validate repository root, ref, tag, and path inputs before invoking Git.

V2 Git adapters must not add automatic `reset --hard`, `clean -fd`, tag deletion, remote deletion, force push, or GitHub Release deletion. Preserve evidence after uncertain operations.

### Filesystem

Use atomic writes for durable single-file state and journal updates. Preserve relevant file mode and exact snapshot bytes when a bounded restore is part of the contract.

Validate that release-owned paths remain inside the repository or intended Git common directory after symlink resolution where applicable. A function that writes, renames, removes, or restores a file must make that side effect clear in its name.

### Release config and state

Keep loading/validation separate from mutation. Application orchestration should receive a repository/state abstraction rather than performing ad hoc `os.ReadFile`/`os.WriteFile` calls.

Validate the complete target state before writing and validate the persisted repository after writing when the current contract does so.

### Release file materialization

Planning must be read-only and return before/after content, hashes, reason, mode, repository-relative path, and whether the file is required in the release commit.

Applying a plan must write only declared changes. The Git commit allowlist must be derived from the validated plan and state file, never from `git add -A`, `git add .`, or a directory-wide glob.

### Execution journals

Write execution journals below the Git common directory with restrictive permissions. Identity filenames must remain safe hashes, not raw unit/tag/path values.

Persist a pending marker before an unsafe action. Confirm the next phase only after the action succeeds and its result is verified. If the post-action journal write fails, report uncertainty and preserve evidence; do not repeat or roll back the action automatically.

### Dispatch journals and workflow dispatch

Prepare the dispatch journal from an immutable request tied to the verified release commit and tag. Record `request-started` before the HTTP request.

Accepted, rejected, and unknown outcomes must remain distinct. A timeout, redirect, transport interruption, unexpected response, or server error is not proof of rejection. Do not automatically retry terminal or uncertain dispatch journals.

### Environment variables and tokens

Resolve credentials only at the adapter/use-case boundary that needs them. Pass them in memory only. Never store a token in config, state, identity, journal, result metadata, logs, error messages, response details, snapshots, fixtures, or golden files.

Sanitize third-party error text before it crosses an adapter boundary. Tests must use unmistakable fake secrets and assert their absence from every persisted and user-visible surface.

### Clock and generated identifiers

Inject a clock when timestamps affect persisted bytes, response snapshots, recovery policy, or deterministic tests. Generate identifiers from explicit canonical facts. Do not mix wall-clock values into an identity intended to be idempotent.

## 8. Unsafe-operation policy

An unsafe operation is any action whose completion may be externally visible, irreversible, ambiguous after interruption, or destructive to evidence. In this plugin that includes at least:

- creating a commit or tag;
- pushing a commit or tag;
- deleting or moving release state;
- dispatching a workflow;
- creating/deleting a GitHub Release;
- starting a release executor that can commit, tag, push, or publish;
- overwriting durable journal state after its matching side effect;
- destructive Git cleanup.

A function performing an unsafe operation must make it obvious through:

- its name;
- a narrow adapter dependency;
- an explicit typed input/result;
- a journal or recovery boundary where required;
- tests for success, definitive failure, and uncertain completion.

Do not wrap multiple unrelated unsafe operations in one adapter method. The application use case must retain control of their order.

## 9. Errors and responses

Use typed or centrally classified application failures. Preserve the original cause with `%w` or an equivalent cause field. Do not flatten an error to a string until the presentation or persistence boundary requires it.

Stable error codes and response semantics must not change silently. Before extracting a handler, characterize:

- status;
- error code;
- message and details that callers rely on;
- metadata plugin/version/command;
- renderer hint;
- data keys;
- item ordering;
- whether the handler returns a response or a Go error to `main`.

Response mapping belongs at the command/presentation boundary. Business orchestration and adapters must not import `pkg/plugin` merely to create responses.

Response item ordering must be deterministic. Sort map-derived output explicitly. Do not rely on Go map iteration.

Never include secrets in messages, details, metadata, logs, or test snapshots. Limit and sanitize remote response bodies before storing or returning them.

Machine-readable output contracts must have an explicit versioning decision and tests before incompatible change. Journal `schemaVersion`, plugin response shapes, and `plugin-index.json` are separate contracts; do not conflate their versions.

## 10. Dry-run and recovery

Dry-run is a read-only use case, not a boolean shortcut through mutating execution. It may load/validate configuration, inspect local files/Git, calculate plans, and render exact intended steps. It must not:

- resolve a token;
- create/update journals;
- write config, state, manifests, or executor files;
- fetch or mutate Git;
- invoke executors;
- send network requests;
- run rollback;
- commit, tag, push, dispatch, publish, or delete.

Resume must use persisted immutable intent. It must not calculate a fresh version, create a new release identity, infer remote completion without evidence, clear ambiguous pending actions, or blindly retry push/dispatch.

A completed/handoff-ready release must not be redispatched. Starting a later release is a separate intent calculated from current state.

## 11. Testing requirements

### Characterization before extraction

Add characterization tests before a risky extraction when the affected behavior is not already pinned. Keep those tests in a separate atomic commit when they establish a meaningful baseline.

Risky extraction includes code that controls version/tag selection, exact commit contents, side-effect order, journals, retry/resume, error codes, or secrets.

### Pure decisions

Test version bumps, tag parsing, unit selection, capability/delivery resolution, state transitions, outcome classification, and response mapping as table-driven pure tests where possible.

### Use cases with fakes

Application use-case tests must replace Git, filesystem/state, journals, token, clock, and dispatch dependencies. Assert the exact call order and stop point. Fakes must be able to fail before and after each unsafe side effect.

### Adapter tests

Keep real temporary Git repository tests for Git commands and commit/ref verification. Keep temp filesystem tests for atomic writes, modes, and symlink/path confinement. Keep injected-transport HTTP tests for headers, payloads, redirects, limits, token redaction, and response classification.

### Command contract tests

Every public command needs success and failure contract coverage. Assert stable codes, renderer hints, command metadata, deterministic item order, and no secret leakage. Keep manifest/routes/docs synchronization tests.

### Failure injection

For the active V2 release and resume flows, test failure around every unsafe step:

- journal prepare/update;
- materialization apply/confirm;
- state write/confirm;
- stage/confirm;
- commit/confirm;
- tag/confirm;
- dispatch-journal prepare/confirm;
- commit push/confirm;
- tag push/confirm;
- request-started write;
- HTTP request;
- terminal dispatch write;
- handoff confirmation.

Each test must assert what did happen, what did not happen, the last durable phase, pending action, preserved local/remote evidence, response classification, and secret absence.

### Determinism and regressions

Inject time and generated IDs where they affect output. Every fixed release bug gets a regression test that fails without the fix. Do not weaken assertions, delete coverage, broaden expected output, add sleeps, or skip tests merely to make a change pass.

## 12. Maintainability rules

Give each function one clearly named responsibility. Extraction is indicated when a function:

- performs multiple unrelated side effects;
- spans several release phases;
- mixes command parsing with execution;
- mixes response construction with business decisions;
- repeats branching on state strings;
- uses booleans to choose different workflows;
- requires real Git, filesystem, network, environment, or wall-clock access in a unit test;
- duplicates canonical unit/version/tag/workflow/plugin metadata;
- duplicates safety ordering already implemented elsewhere.

Responsibility and testability, not an arbitrary line-count limit, decide whether to extract. A long explicit state transition table can be clearer than several tiny indirections; a short function with three unrelated side effects still needs separation.

Delete an obsolete parallel path only after production call sites, characterization tests, and documentation prove which path is authoritative. Do not leave two implementations of release ordering active indefinitely.

Avoid package moves and public symbol renames during behavioral extraction unless the task explicitly requires them. First establish seams in place, then move code in a separate reviewable change if movement still helps.

Do not add production dependencies when the standard library and existing repository packages provide the required boundary.

## 13. Documentation rules

Update documentation in the same change when its contract changes:

- update `docs/architecture/current-state.md` when responsibilities, dependencies, states, or production call paths change;
- update `docs/architecture/refactor-plan.md` when a milestone is completed, reordered, or invalidated;
- update `RULES.md` when engineering policy changes;
- update `plugin/release/manifest.json`, `docs/plugins/release.md`, and `docs/release/*` when public behavior changes;
- update tests that demonstrate every documented contract.

Documentation must name real files, symbols, states, commands, and evidence. Do not describe an intended boundary as implemented before production call sites use it.

## 14. Validation

Run checks proportional to the change, then run the canonical full validation before committing completed work.

For documentation-only changes:

- verify every referenced repository path and named symbol;
- check relative Markdown links;
- run `git diff --check`;
- run the Release Plugin tests;
- run the full Go test suite when practical;
- run the configured linter when installed.

For Go changes, also verify formatting with `gofmt`/`go fmt` and run focused tests while iterating.

Canonical repository commands discovered in this checkout are:

```bash
make plugin-release-test
make lint
make test
make verify
```

Equivalent direct commands may be used to avoid mutating Make targets or to set a writable build cache:

```bash
GOCACHE=/private/tmp/neko-cli-go-build go test ./plugin/release/...
GOCACHE=/private/tmp/neko-cli-go-build go test ./...
golangci-lint run --config .golangci.yml ./...
```

Do not install new global tools merely to satisfy a documentation task. If a canonical command cannot run, record the exact command, output, and environmental reason. Never claim a skipped or failed command passed.

## 15. Commit policy

Codex automatically commits completed work without waiting for separate confirmation when the requested milestone is fully implemented and validated.

- Give each coherent behavior-preserving refactor its own commit.
- Give each independently usable feature its own commit.
- Characterization tests may be committed separately before a risky refactor when they establish a meaningful safety baseline.
- Do not mix unrelated cleanup with a refactor or feature.
- Documentation and tests belong in the same commit as the change they describe unless they are a standalone baseline deliverable.
- Never stage or commit unrelated user changes.
- Never amend, squash, rebase, reset, push, tag, release, dispatch, publish, or otherwise modify remote state unless a later prompt explicitly requests that exact operation.

Use Conventional Commit-style messages with the `release` scope. Preferred forms are:

```text
test(release): characterize <behavior>
refactor(release): <clear structural change>
feat(release): <user-visible capability>
fix(release): <corrected behavior>
docs(release): <documented contract>
```

Commit messages describe the completed change, not a task name or issue title.

Before committing:

1. inspect the complete diff;
2. confirm production behavior changed only as authorized;
3. confirm tests and documentation match the result;
4. confirm no unrelated paths are staged;
5. record validation results.
