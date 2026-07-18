# Release V2 Bootstrap Product Boundary

This page defines the product boundary for bootstrapping a repository into
Release V2. It separates what Neko owns from what GitHub Actions, build-system
adapters, and consumer repositories must own.

It is a product planning document, not an implementation status tracker.
Current capabilities and future capabilities are called out explicitly.

## Product objective

The Release V2 bootstrap product should take a repository from Neko
installation to a release-ready V2 integration:

```text
Neko installation
  -> V2 release config
  -> release-unit config
  -> local validation and inspection
  -> GitHub Actions integration
  -> CI release-context validation
  -> consumer-owned build and publication
```

The bootstrap must work for:

- single-unit repositories;
- multi-unit repositories;
- build-system-neutral consumers;
- Gradle consumers;
- custom publication logic.

## Product decision

Preferred integration model:

```text
CLI-first release policy with thin, generated or documented GitHub Actions integration.
```

Neko CLI and the Release Plugin own release policy, V2 config/state validation,
version planning, release commit/tag/push, workflow dispatch, durable evidence,
and recovery rules. GitHub Actions integration owns repeatable workflow wiring
that installs or invokes Neko and calls stable CLI contracts in CI. Consumer
repositories own their build, test, signing, artifact, publication, credential,
and deployment steps.

This is intentionally a combination model:

- not Action-first, because release policy, version authority, tag calculation,
  journal semantics, and recovery rules must not be reimplemented in YAML;
- not CLI-only, because a committed workflow and CI validation contract are the
  difference between a valid local V2 config and a release-ready integration.

There is no new generic release execution command in this boundary.
`patch`, `minor`, and `major` remain the user-facing release execution commands.
`ci-validate-context` is a validation/introspection contract, not another way
to start a release.

## Ownership principles

- Neko Core owns plugin installation, plugin dispatch, common CLI rendering,
  version/update behavior for the CLI product, and plugin registry consumption.
- The Release Plugin owns Release V1 compatibility, Release V2 config/state,
  unit selection, planning, materialization, Git coordination, workflow
  dispatch, evidence, and recovery.
- GitHub Actions integration owns workflow shape, checkout, CI environment
  setup, and calling stable Neko release-context validation before publishing.
- Build-system adapters own mapping a validated release unit to build-system
  concepts such as modules, tasks, versions, target matrices, and publication
  commands.
- Consumer repositories own product-specific build, test, artifact, publication,
  credential, signing, deployment, and environment policy.
- No layer may duplicate an upstream owner when a stable contract exists.

## Ownership matrix

| Responsibility | Primary owner | Input contract | Output contract | Public API | Provider-neutral | Build-system-neutral | Must not duplicate | Consumer-specific remainder |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Unit config and selection | Release Plugin | `.neko/release.config.json`, `--unit` | normalized selected unit | V2 schema and CLI output | yes | yes | workflow-side unit parsing rules beyond validation | unit naming and path ownership choices |
| Current version state | Release Plugin | `.neko/release.state.json` | selected unit version | V2 state schema | yes | yes | plugin maps or build files as version authority | deciding initial versions |
| Next-version planning | Release Plugin | current version, requested change | next SemVer | `plan`, dry-run, release commands | yes | yes | workflow or adapter bump logic | selecting patch, minor, or major |
| Tag calculation | Release Plugin | unit `tagPrefix`, next version | unit tag | V2 tag strategy | yes | yes | workflow tag formatting except validation | tag-prefix conventions |
| Generated release files | Release Plugin or adapter-specific materializer | selected unit and executor needs | declared known files | materialization plan and known files | yes | partly | hidden workflow file rewrites | source files outside declared materialization |
| Release commit and tag | Release Plugin | known files, selected unit, tag | deterministic commit and lightweight tag | Git coordination contract | yes | yes | executor-owned commit/tag for V2 | none |
| Commit and tag push | Release Plugin | selected upstream remote | pushed release commit, then pushed tag | Git coordination contract | provider-specific today | yes | workflow pushing release refs | remote and branch setup |
| Workflow dispatch | Release Plugin | validated GitHub.com remote and workflow | one journaled dispatch request | dispatch contract | no, GitHub.com only today | yes | standalone retry or workflow-created dispatch | Actions permissions |
| Workflow input contract | Release Plugin | unit, version, tag, release SHA | four deterministic inputs | dispatch contract | no, GitHub Actions today | yes | executor, delivery, paths, or secrets as inputs | workflow descriptions and concurrency naming |
| Dispatch-context validation | GitHub Actions integration through stable CLI contract | checked-out commit, local tag history, and dispatch inputs | validated release context and machine outputs | `ci-validate-context` | GitHub Actions first | yes | ad hoc shell or JSON parsing | extra product policy checks |
| Workflow YAML scaffolding | GitHub Actions integration | existing V2 config/state and exact workflow selection | deterministic create-only contract-version-1 workflow | `github-workflow-init` | no, GitHub Actions only | yes | release policy, build, or publication implementation | replacing the failing extension point or maintaining a manual workflow |
| Build-system version assignment | Build-system adapter | validated release context and V2 state | build-system version value | adapter contract | yes | no | SemVer bumping or state writes | project-specific version propagation |
| Unit-scoped build selection | Build-system adapter | selected unit paths or adapter metadata | tasks and target matrix | adapter contract | yes | no | Release Plugin path ownership | module grouping and target metadata |
| Artifact construction | Consumer repository | validated release context and build outputs | archives, images, packages, checksums | consumer workflow contract | yes | no | Neko-owned release identity | artifact layout and names |
| Publication command | Consumer repository | artifacts, credentials, release context | remote publication | consumer workflow contract | provider-specific | no | Neko release commit/tag/push | exact release tooling invocation |
| Provider credentials | Consumer repository | repository secrets | scoped CI credentials | CI provider secret model | no | yes | storing secrets in Neko config, journals, or inputs | token names and secret rotation |
| Remote publication | Consumer repository | publication command and credentials | GitHub Release, package, image, or deployment | provider/tool contract | no | no | Neko evidence claims for publish result | provider-specific publication semantics |
| Recovery evidence | Release Plugin | local execution and dispatch journals | safe recovery assessment | evidence and resume commands | partly | yes | workflow-side journal mutation | manual remote inspection after uncertainty |
| Execution diagnostics | Release Plugin and GitHub Actions integration | release progress and CI logs | human-readable diagnostics | CLI output and workflow logs | partly | yes | secrets or raw remote bodies | product-specific troubleshooting notes |
| Consumer-specific deployment | Consumer repository | successful publication artifacts | deployment, promotion, release notes | consumer workflow contract | no | no | Neko release-policy changes | all deployment policy |

## Single-unit golden path

Single-unit repositories should be bootstrapped so the only release unit can be
selected implicitly by local commands, while CI still validates the explicit
unit identity supplied by the dispatch input.

Current path:

1. Install Neko CLI and the bundled Release Plugin.
2. Create one V2 unit with `neko release init`.
3. Commit `.neko/release.config.json` and `.neko/release.state.json`.
4. Add or retain the referenced workflow file below `.github/workflows/`; when
   a structurally valid V2 pair exists and the configured target is missing,
   use `neko release github-workflow-init`.
5. Add the executor config and any product-specific build or publication files.
6. Run `neko release validate --show`.
7. Inspect with `neko release plan --change patch`.
8. Run `neko release patch`, `minor`, or `major`.
9. Let GitHub Actions run `ci-validate-context` before build and publish.
10. Use `neko release resume --dry-run` only for unresolved local evidence.

The scaffolder is create-only and separate from `init`: it creates a missing
configured target, recognizes byte-identical output, and refuses different
consumer content. A future integration doctor remains separate.

## Multi-unit golden path

Multi-unit repositories make the release unit explicit in every unit-scoped
operation. One central workflow may handle many units when it derives behavior
from checked-out V2 config and adapter metadata; per-unit workflows remain valid
when the repository wants isolated publication logic.

Current path:

1. Create the first V2 unit with `neko release init`.
2. Append additional units with `neko release unit-add`.
3. Ensure each unit has a non-overlapping `tagPrefix`, path ownership, existing
   `workingDirectory`, supported executor, and canonical workflow path.
4. When a configured target is missing, scaffold one exact path with
   `neko release github-workflow-init --unit <unit>` or `--path <path>`; units
   sharing one path use one central workflow.
5. Commit config/state, workflow files, executor config, and consumer build
   support files.
6. Run `neko release validate --show`.
7. Inspect one unit with `neko release plan --change patch --unit <unit>`.
8. Release one unit with `neko release patch --unit <unit>`.
9. Let the workflow pass `unit`, `version`, `tag`, and `release_sha` to
   `ci-validate-context` and consume its validated outputs.

Future product path:

1. Add or import units.
2. Inspect a unit overview before release planning.
3. Run an integration doctor for every release unit.
4. Use pipeline inspection to explain local and CI readiness before execution.

## Build-system-neutral consumers

A build-system-neutral consumer should be able to use Release V2 without a
build adapter. The workflow validates the release context, then runs
repository-owned shell commands.

The stable contract is:

- Neko provides unit, version, tag, release SHA, workflow path, executor, and
  delivery through checked-in config/state plus dispatch inputs.
- The workflow validates those facts before build and publish.
- The repository decides which commands build, test, package, sign, and publish.
- Neko does not infer artifact names, provider credentials, or deployment
  targets from generic folder structure.

## Gradle consumers

Gradle support belongs in a build-system adapter, not in Release Plugin core.

A Gradle adapter may:

- read a stable Neko Release V2 contract;
- map a release unit to Gradle projects, tasks, and publication targets;
- assign the release version to Gradle project versions;
- verify alignment between Gradle project versions and V2 state;
- emit deterministic target metadata for CI matrices.

A Gradle adapter must not:

- calculate the next release version;
- write `.neko/release.state.json`;
- create commits, tags, pushes, dispatches, journals, or GitHub Releases;
- infer release safety from remote publication state;
- parse private Neko internals when a stable CLI or schema contract exists.

## Custom publication logic

Custom publication logic remains consumer-owned. It may use the validated CI
release context and still decide its own:

- test and build commands;
- artifact layout;
- signing process;
- package registry or GitHub Release command;
- deployment and promotion steps;
- provider credentials and secret names.

The consumer workflow must treat a failed context validation as a hard stop
before publishing.

## CI release-context contract

The GitHub Actions workflow-dispatch input contract is intentionally small:

```text
unit
version
tag
release_sha
```

Authoritative values:

| Fact | Supplied by Neko dispatch | Recomputed or read in CI | Failure condition |
| --- | --- | --- | --- |
| Unit id | `unit` input | selected unit in `.neko/release.config.json` | input is unknown or mismatched |
| Version | `version` input | `.neko/release.state.json` selected unit version | values differ or version is invalid |
| Tag | `tag` input and checkout ref | unit `tagPrefix + version` | values differ or tag points elsewhere |
| Release SHA | `release_sha` input | checked-out `HEAD` and tag target | either SHA differs |
| Workflow path | V2 config | checked-out config | selected unit does not own current workflow |
| Executor and delivery | V2 config | checked-out config | unsupported or unexpected for workflow |
| Known release files | release plan/commit contract | checked-out commit and local validation | state/materialized files are inconsistent |

`ci-validate-context` is token-free and publication-free. It produces ordered
human properties, deterministic JSON, and safely encoded GitHub step outputs.
It validates the complete local V2 pair, selected unit, exact version and tag,
full local commit object, checked-out HEAD, and peeled local tag target. Missing
tag history fails rather than fetching. Workflow identity is returned from V2
config for downstream integration policy; the command does not inspect ambient
GitHub runtime state.

## Current supported capabilities

Supported today:

- V1 compatibility and root V1-to-V2 migration.
- V2 config/state creation for one unit through `init`.
- V2 unit append through `unit-add`.
- V2 validation with `validate --show`.
- Unit-aware `history` and `contributors`.
- Token-free `plan` inspection and dry-run release planning.
- Token-free, network-free `ci-validate-context` with human, JSON, and explicit
  GitHub command-file output.
- Token-free, network-free, create-only `github-workflow-init` with exact
  target selection, deterministic YAML, dry-run, idempotent recognition, and
  fail-closed conflicts.
- GitHub Actions-delivered `patch`, `minor`, and `major` releases.
- Neko-owned state/materialization, release commit, unit tag, commit push, tag
  push, execution journal, dispatch journal, and workflow dispatch.
- `resume` and `resume --dry-run` for unresolved V2 GitHub Actions evidence.
- Plugin unit metadata and `plugin-index` generation.
- Plugin registry publication from Neko plugin release workflows.
- V2 local delivery rejection.

Not supported today:

- workflow generation as an implicit side effect of `init` or `unit-add`;
- managed workflow updates, arbitrary YAML merging, or force overwrite;
- executor config scaffolding from `init` or `unit-add`;
- a first-class integration doctor;
- a release unit overview command;
- release pipeline inspection;
- build-system adapters in Neko CLI;
- V2 local non-dry-run execution;
- standalone dispatch or retry commands.

## Future product capabilities

Future Release V2 bootstrap work should add capabilities in this order:

1. Read-only integration doctor.
2. Release unit overview.
3. Release pipeline inspection.
4. Build-system adapter contract and a Gradle adapter.
5. GitHub Actions packaging decision after the generated workflow contract is
   proven in consumers.

Build-system adapter work can start after the stable CI validation contract is
defined, but it must not block build-system-neutral workflow support.

## Explicit non-goals

- No V2 local executor execution.
- No universal build or deployment engine inside Neko.
- No generic `neko release execute` command.
- No standalone dispatch or retry command.
- No automatic remote-state inference for uncertain publication.
- No moving provider credentials into release config, state, journals, or
  dispatch inputs.
- No build-system adapter parsing private implementation files when stable
  schemas or CLI JSON output can provide the contract.
- No generated workflow that publishes without validating the dispatch context.
- No removal of V1 compatibility as part of bootstrap work.

## Compatibility principles

- Preserve V1 compatibility until a separate compatibility-removal decision is
  made.
- Preserve V2 schema ownership: config owns release architecture and state owns
  versions.
- Preserve `patch`, `minor`, and `major` as the release execution commands.
- Preserve the workflow input names `unit`, `version`, `tag`, and `release_sha`.
- Preserve GitHub Actions handoff ordering: release commit, unit tag, commit
  push, tag push, dispatch.
- Preserve dry-run and plan inspection as token-free and write-free.
- Preserve conservative recovery: no blind retry for ambiguous push or
  uncertain workflow dispatch.
- Preserve consumer ownership of build, publication, credentials, and
  deployment.
