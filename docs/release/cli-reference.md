# Release CLI Reference

> **Audience:** Release users, operators, automation authors, and contributors verifying public command behavior.
>
> **Purpose:** Provide the canonical Release command reference for commands, flags, sources, I/O, network, tokens, mutation, output, and exits.

## Canonical ownership

This is the canonical Release command reference and authoritative command,
flag, output, exit, source, network,
token, and mutation contract. There are 20 manifest commands and one
Core-owned overview path. Other Release pages summarize a specific workflow
and link here instead of owning another full matrix.

The safety shorthand is exact: Doctor is read-only and never repairs;
Workflow Init is create-only; Pipeline is read-only and is not a lifecycle
engine; `--output-file` writes Plugin Index schema-v1 bytes only for the
`plugin-index` command.

## Release V1 versus Release V2

Release V1 is a supported compatibility surface, not the preferred setup for a
new repository. Its authority is the legacy root `.release.neko.json`, its
unit model is one virtual `default` unit, and its configured legacy tool is
GoReleaser, JReleaser, or release-it. V1 remains supported by `patch`, `minor`,
`major`, `plan`, `history`, `contributors`, `validate`, and the relevant
Evidence families. `--unit default` is accepted; another V1 unit is rejected.
V1 lifecycle compatibility keeps its established tool-owned commit, tag, push,
and publication boundaries.

Release V2 is the canonical active architecture. The immutable unit structure
lives in `.neko/release.config.json`; current unit versions live in
`.neko/release.state.json`. V2 owns explicit multi-unit selection, unit tag
prefixes, materialization, GitHub Actions workflow handoff, execution and
dispatch journals, Evidence, recovery, Doctor, Units, Pipeline, workflow
initialization, dispatched-context validation, Resume, and Plugin Index.
Executable V2 release delivery is `github-actions`; V2 `local` is rejected.

V1 and V2 must not remain active as competing authorities. Canonical source
loading selects one generation once. A complete valid V2 pair is preferred as
the V2 source, but coexistence with V1 is a conflict; partial, malformed,
schema-invalid, config/state-mismatched, or recovery-blocked V2 sources are
reported rather than merged. Merging would make version, unit, tag, file, and
recovery ownership ambiguous. Use `neko release migrate --dry-run`, complete a
verified migration, then validate V2 before starting a V2 lifecycle command.

| Generation | Authoritative files | Unit model | Setup/status |
| --- | --- | --- | --- |
| V1 compatibility | `.release.neko.json` | One virtual `default` unit | Supported existing source; not created by current `init` |
| V2 canonical | `.neko/release.config.json`, `.neko/release.state.json` | Explicit one or many units | Active setup and lifecycle architecture |
| V2 recovery evidence | `.neko/release.pair-recovery.json` plus journals below the Git common directory | Exact persisted intent | Blocks unsafe source use until canonical recovery policy resolves it |

## Release support-status matrix

`Manifest outputs` records the plugin-declared output vocabulary. Core also
accepts `wide` as the extended table renderer for every structured response.
Successful `github` command-file output exists only for the context validator.

<!-- release-support-matrix:start -->
| Command | Support | Required files | Read or mutate | Network | Token | Git | Filesystem | Manifest outputs | Default | Describe | Verbose | Exit |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `neko release` | Core overview | Installed `manifest.json` | Read-only overview | Offline | None | None | Manifest inspect | `Core text` | Static command inventory | No-op | No-op | `0` |
| `neko release init` | V2 only | No active source; or replaceable V2 pair | Guarded mutation | Offline | None | None | Create/replace V2 pair and recovery evidence | `table, json` | Concise pair result | Complete configuration/write/validation facts | Safe write phases | Success `0`; invalid/conflict/write failure `1` |
| `neko release unit-add` | V2 only | Complete valid V2 pair | Guarded mutation | Offline | None | None | Append pair through recovery writer | `table, json` | Concise unit result | Complete unit/comparison/write facts | Safe append phases | Success `0`; invalid/duplicate/write failure `1` |
| `neko release init-options` | V2 only | None | Read-only | Offline | None | None | None | `table, json` | Complete choice inventory | No-op | No-op | `0` |
| `neko release migrate` | V1 to V2 migration | Root `.release.neko.json` | Dry-run or guarded migration | Offline | None | Inspect root only | Create V2 pair/journals; archive V1 | `table, json` | Migration result | Source/artifact/plan/write facts | Safe migration phases | Success/dry-run `0`; refusal/failure `1` |
| `neko release patch` | Shared V1/V2 | Valid V1 or complete V2 pair | Dry-run or lifecycle mutation | Execution uses configured Git/provider remotes | `GITHUB_TOKEN` required for execution; none for dry-run | Execution may commit, tag, push | Materialize/write journals and release files | `table, json` | Lifecycle result/preview | Complete safe lifecycle facts | Chronological phases | Success/dry-run `0`; invalid/refusal/failure `1` |
| `neko release minor` | Shared V1/V2 | Valid V1 or complete V2 pair | Dry-run or lifecycle mutation | Same as patch | Same as patch | Same as patch | Same as patch | `table, json` | Lifecycle result/preview | Complete safe lifecycle facts | Chronological phases | Same as patch |
| `neko release major` | Shared V1/V2 | Valid V1 or complete V2 pair | Dry-run or lifecycle mutation | Same as patch | Same as patch | Same as patch | Same as patch | `table, json` | Lifecycle result/preview | Complete safe lifecycle facts | Chronological phases | Same as patch |
| `neko release plan` | Shared V1/V2 | Valid V1 or complete V2 pair | Read-only | Offline | None | Local evidence only; no mutation | Inspect planned files | `table, json` | Local plan summary | Complete plan facts | No-op | Ready/blocked observation `0`; invalid `1` |
| `neko release doctor` | V2 only | V2 inspection source/workflow | Read-only | Offline default; explicit bounded GitHub GET | Optional `GITHUB_TOKEN` only with remote verification | None | Inspect config/state/workflow | `table, json` | Readiness/actionable findings | Complete diagnostics | No-op | Ready/warning `0`; not ready `1` |
| `neko release units` | V2 only | V2 inspection source | Read-only | Offline | None | None | Inspect config/state/recovery marker | `table, json` | Complete unit inventory | Complete unit facts | No-op | Valid `0`; issues/source invalid `1` |
| `neko release pipeline` | V2 only | Complete valid V2 pair/workflow | Read-only | Offline default; explicit bounded GitHub GET | Optional `GITHUB_TOKEN` only with remote verification | Inspect local branch/objects/refs/index/worktree | Inspect journals/config/state/workflow | `table, json` | Summary/actionable findings | Complete pipeline facts | No-op | Valid observation including blocked `0`; invalid `1` |
| `neko release ci-validate-context` | V2 only | Complete valid V2 pair and exact local commit/tag | Read-only domain check | Offline | None | Inspect only; never fetch | Core may append explicit GitHub command file | `table, json, github` | Validated context/contradictions | Complete checks/context | No-op | Valid `0`; contradiction/failure `1` |
| `neko release github-workflow-init` | V2 only | Complete valid V2 pair/configured workflow | Dry-run or create-only guarded write | Offline | None | None | Create one missing workflow; never overwrite different bytes | `table, json` | Create/current/preview result | Comparison/input/write facts | Safe inspect/write phases | Success/identical/dry-run `0`; conflict/failure `1` |
| `neko release resume` | V2 only | One matching unresolved execution journal | Dry-run or journaled continuation | Dry-run offline; continuation may push/dispatch | None for dry-run; `GITHUB_TOKEN` only for fresh dispatch | May tag/push; never plans new version | Read/update existing journals/evidence | `table, json` | Recovery decision | Complete journal/Git/recovery facts | Continuation/refusal phases | Safe dry-run/completion `0`; no journal/refusal/failure `1` |
| `neko release history` | Shared V1/V2 | Valid V1 or complete V2 pair | Read-only | Offline | None | Inspect history/tags | Config/state inspect | `table, json` | Complete history | No-op | No-op | Valid/legacy empty `0`; structured failure `1` |
| `neko release contributors` | Shared V1/V2 | Valid V1 or complete V2 pair | Read-only | Offline | None | Inspect shortlog | Config/state inspect | `table, json` | Complete contributors | No-op | No-op | Valid `0`; structured failure `1` |
| `neko release validate` | Shared V1/V2 | V1 source or V2 inspection pair | Read-only | Offline | V1 requires `GITHUB_TOKEN` compatibility read; V2 none | None | Inspect source/config/state/tool file | `table, json` | Validation result | Safe validation facts | No-op | Valid `0`; invalid `1` |
| `neko release evidence` | Shared V1/V2 | Supported evidence locations | Read-only | Offline | None | None | Inspect redacted evidence | `table, json` | Evidence classifications | Complete safe evidence | No-op | Valid/empty/malformed diagnostic `0`; invalid filter `1` |
| `neko release evidence-archive` | Shared V1/V2 | Supported completed evidence | Guarded mutation | Offline | None | None | Write/verify private archive; remove exact source | `table, json` | Guarded archive result | Limited safe write facts | Guarded phases | Success `0`; guard/failure `1` |
| `neko release plugin-index` | V2 only | Complete V2 plugin units/state/manifests | Raw/check read-only; explicit persist mutation | Offline | None | None | Persist only explicit output file | `json, table` | Raw schema-v1 or concise mode result | Raw no-op; check/persist complete facts | Raw no-op; check/persist safe phases | Success `0`; check/build/persist failure `1` |
<!-- release-support-matrix:end -->

## Global and inherited flag reference

These flags are not manifest-local Release request fields. Core consumes
`--describe`, `--output`, and `--github-output-file`; only verbose intent is
transported as `Request.Context.Verbose`. Scalar flags may be repeated; the
last occurrence wins.

<!-- release-global-flag-inventory:start -->
| Flag | Owner | Required | Default | Accepted values | Repeat | Sent to plugin | Restriction |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `--help, -h` | Cobra | no | `false` | boolean | Last wins | no | Render manifest-derived help and stop |
| `--describe` | Core | no | `false` | boolean | Last wins | no | Human detail only; never enables reads or effects |
| `--verbose, -v` | Core | no | `false` | boolean | Last wins | context only | Captured phases where command-owned |
| `--output` | Core | no | `table` | `table`, `json`, `wide`, `github` | Last wins | no | Renderer selector; never a file path |
| `--github-output-file` | Core | with GitHub output | `empty` | explicit path | Last wins | no | Used only with `--output github`; destination is never inferred |
<!-- release-global-flag-inventory:end -->

Core accepts exactly `table`, `json`, `wide`, and `github`. `wide` is Core's
extended human table mode and need not be repeated in manifest `outputs`.
Only `ci-validate-context` declares successful GitHub command-file output.
Other successful responses fail safely instead of inventing output fields.
`--describe` cannot be combined with `--output github`. `--github-output-file`
is ignored outside GitHub mode and must name an explicit available command file
inside it.

## Command-local flag reference

The following 66 rows are the complete Release manifest-local flag inventory.
`Required` is the manifest/Cobra presence requirement; conditional domain
requirements are stated separately. Every flag is scalar and uses pflag's
last-value-wins repeat behavior.

<!-- release-local-flag-inventory:start -->
| Command | Flag | Type | Required | Default | Accepted values and restrictions | Repeat | Owner |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `neko release init` | `--unit` | string | false | `cli` | Valid unit ID; plugin kind must start `plugin-` | Last wins | Release manifest |
| `neko release init` | `--display-name` | string | false | selected unit ID | Non-empty string | Last wins | Release manifest |
| `neko release init` | `--version` | string | false | `0.1.0` | SemVer without leading `v` | Last wins | Release manifest |
| `neko release init` | `--executor` | string | true | none; required | `goreleaser`, `jreleaser`, `release-it` | Last wins | Release manifest |
| `neko release init` | `--delivery` | string | true | none; required | `github-actions` only | Last wins | Release manifest |
| `neko release init` | `--workflow` | string | false | empty | Required for GitHub Actions; configured `.github/workflows` YAML path | Last wins | Release manifest |
| `neko release init` | `--tag-prefix` | string | false | `v` | Valid non-overlapping tag prefix; plugin kind requires unit prefix plus `/v` | Last wins | Release manifest |
| `neko release init` | `--working-directory` | string | false | `.` | Repository-confined relative unit directory | Last wins | Release manifest |
| `neko release init` | `--paths` | string | false | `**` | One comma-separated path-glob string | Last wins | Release manifest |
| `neko release init` | `--kind` | string | false | `release` | `release` or `plugin` | Last wins | Release manifest |
| `neko release init` | `--plugin-name` | string | false | empty | Only with `--kind plugin`; required there | Last wins | Release manifest |
| `neko release init` | `--plugin-manifest` | string | false | empty | Repository-relative manifest; only with plugin kind and required there | Last wins | Release manifest |
| `neko release init` | `--plugin-asset-prefix` | string | false | empty | Only with plugin kind; required and equal to unit ID | Last wins | Release manifest |
| `neko release init` | `--plugin-binary-name` | string | false | empty | Only with plugin kind and required there | Last wins | Release manifest |
| `neko release init` | `--force` | bool | false | `false` | Boolean; replaces permitted V2 config/state, never V1 | Last wins | Release manifest |
| `neko release unit-add` | `--unit` | string | true | none; required | Valid new unit ID; existing unit is rejected | Last wins | Release manifest |
| `neko release unit-add` | `--display-name` | string | false | selected unit ID | Non-empty string | Last wins | Release manifest |
| `neko release unit-add` | `--version` | string | false | `0.1.0` | SemVer without leading `v` | Last wins | Release manifest |
| `neko release unit-add` | `--executor` | string | true | none; required | `goreleaser`, `jreleaser`, `release-it` | Last wins | Release manifest |
| `neko release unit-add` | `--delivery` | string | true | none; required | `github-actions` only | Last wins | Release manifest |
| `neko release unit-add` | `--workflow` | string | false | empty | Required for GitHub Actions; configured `.github/workflows` YAML path | Last wins | Release manifest |
| `neko release unit-add` | `--tag-prefix` | string | false | `v` | Valid non-overlapping tag prefix; plugin kind requires unit prefix plus `/v` | Last wins | Release manifest |
| `neko release unit-add` | `--working-directory` | string | false | `.` | Repository-confined relative unit directory | Last wins | Release manifest |
| `neko release unit-add` | `--paths` | string | false | `**` | One comma-separated path-glob string | Last wins | Release manifest |
| `neko release unit-add` | `--kind` | string | false | `release` | `release` or `plugin` | Last wins | Release manifest |
| `neko release unit-add` | `--plugin-name` | string | false | empty | Only with `--kind plugin`; required there | Last wins | Release manifest |
| `neko release unit-add` | `--plugin-manifest` | string | false | empty | Repository-relative manifest; only with plugin kind and required there | Last wins | Release manifest |
| `neko release unit-add` | `--plugin-asset-prefix` | string | false | empty | Only with plugin kind; required and equal to unit ID | Last wins | Release manifest |
| `neko release unit-add` | `--plugin-binary-name` | string | false | empty | Only with plugin kind and required there | Last wins | Release manifest |
| `neko release migrate` | `--dry-run` | bool | false | `false` | Boolean; preview only | Last wins | Release manifest |
| `neko release patch` | `--dry-run` | bool | false | `false` | Boolean; no mutation/token/network | Last wins | Release manifest |
| `neko release patch` | `--unit` | string | false | V1 `default`; unique V2 unit | V1 only `default`; required for multi-unit V2 | Last wins | Release manifest |
| `neko release minor` | `--dry-run` | bool | false | `false` | Boolean; no mutation/token/network | Last wins | Release manifest |
| `neko release minor` | `--unit` | string | false | V1 `default`; unique V2 unit | V1 only `default`; required for multi-unit V2 | Last wins | Release manifest |
| `neko release major` | `--dry-run` | bool | false | `false` | Boolean; no mutation/token/network | Last wins | Release manifest |
| `neko release major` | `--unit` | string | false | V1 `default`; unique V2 unit | V1 only `default`; required for multi-unit V2 | Last wins | Release manifest |
| `neko release plan` | `--change` | string | true | none; required | `patch`, `minor`, `major` | Last wins | Release manifest |
| `neko release plan` | `--unit` | string | false | V1 `default`; unique V2 unit | V1 only `default`; required for multi-unit V2 | Last wins | Release manifest |
| `neko release doctor` | `--unit` | string | false | all V2 units/workflows | Existing V2 unit; shared workflow scope is retained | Last wins | Release manifest |
| `neko release doctor` | `--verify-remote` | bool | false | `false` | Boolean; explicit bounded GitHub GET mode | Last wins | Release manifest |
| `neko release pipeline` | `--unit` | string | false | unique V2 unit | Existing unit; required for multi-unit V2 | Last wins | Release manifest |
| `neko release pipeline` | `--verify-remote` | bool | false | `false` | Boolean; explicit bounded GitHub GET mode | Last wins | Release manifest |
| `neko release ci-validate-context` | `--unit` | string | true | none; required | Exact dispatched V2 unit | Last wins | Release manifest |
| `neko release ci-validate-context` | `--version` | string | true | none; required | Canonical SemVer without leading `v` | Last wins | Release manifest |
| `neko release ci-validate-context` | `--tag` | string | true | none; required | Exact canonical unit tag | Last wins | Release manifest |
| `neko release ci-validate-context` | `--release-sha` | string | true | none; required | Full lowercase commit object ID for repository object format | Last wins | Release manifest |
| `neko release github-workflow-init` | `--unit` | string | false | unique configured workflow | Existing V2 unit | Last wins | Release manifest |
| `neko release github-workflow-init` | `--path` | string | false | unique configured workflow | Exact configured direct `.github/workflows` YAML path; must agree with unit | Last wins | Release manifest |
| `neko release github-workflow-init` | `--dry-run` | bool | false | `false` | Boolean; returns exact preview without write | Last wins | Release manifest |
| `neko release resume` | `--unit` | string | false | unique matching unit | Existing V2 unit; required when selection is ambiguous | Last wins | Release manifest |
| `neko release resume` | `--dry-run` | bool | false | `false` | Boolean; local assessment only | Last wins | Release manifest |
| `neko release history` | `--unit` | string | false | V1 `default`; unique V2 unit | V1 only `default`; required for multi-unit V2 | Last wins | Release manifest |
| `neko release contributors` | `--unit` | string | false | V1 `default`; unique V2 unit | V1 only `default`; required for multi-unit V2 | Last wins | Release manifest |
| `neko release validate` | `--show` | bool | false | `false` | Boolean; detailed compatibility view | Last wins | Release manifest |
| `neko release validate` | `--unit` | string | false | all repository units | Focus V2 display only; complete repository still validated | Last wins | Release manifest |
| `neko release evidence` | `--family` | string | false | all families | `release-execution`, `dispatch`, `migration`, `v1-compensation`, `v2-pair-recovery` | Last wins | Release manifest |
| `neko release evidence` | `--unit` | string | false | all units | Exact unit filter after family support | Last wins | Release manifest |
| `neko release evidence` | `--identity` | string | false | all identities | Unique lowercase hexadecimal prefix, 8 to 64 characters | Last wins | Release manifest |
| `neko release evidence-archive` | `--family` | string | true | none; required | Completed `release-execution`, `v1-compensation`, or `v2-pair-recovery` | Last wins | Release manifest |
| `neko release evidence-archive` | `--identity` | string | true | none; required | Exact full 64-character identity; no prefix | Last wins | Release manifest |
| `neko release evidence-archive` | `--digest-sha256` | string | true | none; required | Exact current SHA-256 digest | Last wins | Release manifest |
| `neko release evidence-archive` | `--confirm-archive` | bool | true | none; required | Must be explicitly `true` | Last wins | Release manifest |
| `neko release plugin-index` | `--output-file` | string | false | empty | Repository-relative or explicit absolute output path; persist mode | Last wins | Release manifest |
| `neko release plugin-index` | `--check` | bool | false | `false` | Boolean; read-only validation mode; mutually exclusive with output file | Last wins | Release manifest |
| `neko release plugin-index` | `--pretty` | bool | false | `true` | Boolean; pretty or compact schema-v1 bytes | Last wins | Release manifest |
| `neko release plugin-index` | `--repository` | string | false | `nekoman-hq/neko-cli` | Non-empty repository identifier stored in artifact | Last wins | Release manifest |
<!-- release-local-flag-inventory:end -->

`release`, `init-options`, and `units` have no command-local flags. No public Release command or flag is deprecated. There are no public Release command aliases or compatibility flag aliases. `--project-type`, `--release-system`, and `--metadata` are not registered public flags; use `--executor`,
`--delivery`, `--kind`, and the plugin metadata flags. Use `--output-file` to persist Plugin Index bytes; Core `--output` remains a renderer selector.
`--check` and `--output-file` are mutually exclusive.

Global presentation flags preserve the selected source, version/tag
calculation, configured release tool, files, journals, Git ownership, workflow
handoff, domain JSON, and exit behavior. Describe never adds reads, token
resolution, network, or mutation. Verbose only exposes already-owned safe
phases for commands classified as verbose; it is a deterministic no-op
elsewhere. Combining the flags adds no duplicate sections or log records.

V1 and V2 intentionally differ in source selection, unit model, tag policy,
materialized files, lifecycle ownership, and validation requirements. Shared
commands retain those domain differences. Presentation flags do not make their
JSON outcomes or side effects converge and do not alter either path.

Every in-repository Release response explicitly owns its semantic process
status. Completed mutations, dry-runs, queries, checks, and successful negative
observations exit `0`. Invalid requests, failed checks, actionable refusals,
and execution failures exit `1`. In particular, blocked Plan and
blocked/uncertain/rejected Pipeline observations, warning-only Doctor results,
unsafe Resume dry-run assessments, malformed Evidence diagnostics, empty
Evidence inventories, and legacy empty History observations remain successful.
Doctor `not_ready`, Units issues, invalid Pipeline evidence, CI context
mismatch, invalid Validate results, workflow conflicts, actual lifecycle or
Resume refusals/failures, Evidence filter errors, archive guard failures, and
Plugin Index check/persist failures exit `1`.

Core validates a decoded response before output, renders it exactly once, then
applies its explicit exit. A valid response owns the result even when the
plugin subprocess also exits nonzero. Missing, malformed, or invalid responses,
renderer/output failures, and pre-dispatch Core failures are Core-owned exit
`1`. Core supports an explicit response exit `0` through `125`, although
Release itself uses only `0` and `1`. Installed legacy plugins that omit the
transport field temporarily retain implicit-success compatibility. The field
does not appear in public command JSON or GitHub output.

Representative semantic outcomes are intentionally exact:

- Pipeline `blocked` -> `0`; invalid Pipeline evidence -> `1`.
- Doctor warning -> `0`; Doctor `not_ready` -> `1`.
- Resume unsafe dry-run -> `0`; Resume with no matching journal -> `1`.
- Evidence malformed diagnostic -> `0`; Evidence invalid filter -> `1`.
- Plugin Index failed check -> `1`.

## General

```bash
neko release init --executor goreleaser --delivery github-actions --workflow .github/workflows/release-cli.yml
neko release init --unit plugin-release --kind plugin --plugin-name release --plugin-manifest plugin/release/manifest.json --plugin-asset-prefix plugin-release --plugin-binary-name plugin-release --executor goreleaser --delivery github-actions --workflow .github/workflows/release-plugin-release.yml --tag-prefix plugin-release/v
neko release unit-add --unit api --executor goreleaser --delivery github-actions --workflow .github/workflows/release-api.yml --tag-prefix api/v --paths "apps/api/**"
neko release github-workflow-init --dry-run
neko release doctor
neko release units
neko release init-options
```

`init` creates V2 `.neko/release.config.json` and `.neko/release.state.json` files for one release unit. Use `--kind release` or omit `--kind` for a normal service, app, CLI, SDK, library, or backend module; V2 JSON omits `kind` for normal units. Use `--kind plugin` plus `--plugin-name`, `--plugin-manifest`, `--plugin-asset-prefix`, and `--plugin-binary-name` only for a Neko CLI plugin unit. Plugin flags without `--kind plugin` are invalid. Plugin unit ids must start with `plugin-`, plugin tag prefixes must be `<unit-id>/v`, and plugin asset prefixes must match the unit id. `init` no longer creates `.release.neko.json` or initializes executor-specific tool files. Existing V1 repositories should use `neko release migrate`. `github-actions` delivery requires `--workflow .github/workflows/<file>.yml|yaml`; `init` and `unit-add` do not generate workflows or executor configuration. Use the separate `github-workflow-init` command for opt-in workflow scaffolding from an existing V2 pair.

`unit-add` appends one unit to existing V2 config/state. It uses the same unit flags as `init`; the plugin metadata flags are only for `--kind plugin` Neko CLI plugin units. Normal repositories can contain only normal release units and need no plugin metadata or plugin registry. It requires both `.neko/release.config.json` and `.neko/release.state.json`, preserves existing units, and never overwrites an existing unit. It does not generate workflow files, GoReleaser config files, plugin manifests, source directories, or any release artifacts. See [Normal release units vs Neko CLI plugin units](examples.md#normal-release-units-vs-neko-cli-plugin-units).

### Setup and migration presentation

`init`, `unit-add`, `migrate`, and `github-workflow-init` declare the supported
Core formats `table,json`. They inherit the global `--describe`, `--verbose`,
and `--output` flags. They do not declare local copies of those flags, and
there is no selectable global format named `text`.

| Command | Default table output | `--describe` additions | `--verbose` phases |
| --- | --- | --- | --- |
| `init` | initialized unit, version, executor, delivery, V2 config/state, next action | `Resolved Configuration` with complete unit and force facts; `Artifact Write Plan`; `Validation Facts`; `Limitations` | input validation; repository-state inspection; configuration resolution; pair validation; artifact preparation; config/state write; completion |
| `unit-add` | added unit, version, executor, delivery, updated V2 config/state, next action | `Resolved Unit`; `Existing Unit Comparison`; `Artifact Write Plan`; `Validation Facts`; `Limitations` | existing-pair inspection/read; input/default resolution; duplicate check; updated-pair validation; config/state write; completion |
| `migrate` | V1/V2 contracts, readiness/outcome, dry-run state, planned count, V2 targets, archive decision, next action | `Source Facts`; `Resolved V2 Configuration`; `Generated Artifacts`; `Ordered Migration Plan`; `Archive and Journal`; `Validation Facts`; `Write Plan`; `Limitations` | source/root discovery; V1 validation; V2 derivation and validation; archive/journal planning; dry-run decision; successful pair write/verification; archive; journal completion |
| `github-workflow-init` | create/current result, target, canonical identity, contract, write outcome, next action; dry-run preview once | `Workflow Identity`; `Target Comparison`; `Validation Facts`; `Required Workflow Inputs`; `Write Plan`; `Limitations` | request/config selection; target/path resolution; read/render/validation; canonical comparison; classification; dry-run, idempotent acceptance, conflict, or create; completion |

Failures remain visible without `--describe`. Init reports the affected V1/V2
source or pair and whether `--force` applies. Unit Add reports duplicate or
invalid unit facts and remediation. Migrate distinguishes planning failures
with no writes from later failures where recovery evidence may remain.
Workflow Init reports the safe target, refuses overwrite, and recommends a
dry-run/manual comparison for differing content.

JSON remains the established domain response for every command:
`--describe --output json` does not add presentation-only fields or change
values, ordering, nullability, dry-run/actual variants, or exit behavior.
`--verbose` may add captured logs outside domain data. Human output and logs
use repository-relative paths, do not print generated config documents,
credentials, token values, or environment values, and contain no ANSI when
redirected or when `NO_COLOR` is set.

The mode flags do not change side effects. Init and Unit Add persist one
validated V2 pair with the existing recovery policy. Migrate dry-run performs
no write; successful migration retains its established V1 archive and journal
semantics. Workflow Init creates only a missing target, accepts byte-identical
content without rewriting, and never overwrites differing content. None of
these presentation modes performs Git or network operations.

## V1

V1 uses `.release.neko.json` and supports the existing release commands.

```bash
neko release patch --dry-run
neko release plan --change patch
neko release patch
neko release minor
neko release major
neko release history
neko release contributors
neko release validate --show
```

`--unit default` is accepted for V1. Any other unit is rejected.

## V2

V2 uses:

```text
.neko/release.config.json
.neko/release.state.json
```

Supported now:

```bash
neko release migrate
neko release migrate --dry-run
neko release validate
neko release validate --unit api --show
neko release history --unit api
neko release contributors --unit web
neko release patch --unit api --dry-run
neko release minor --unit api --dry-run
neko release major --unit api --dry-run
neko release plan --change patch --unit api
neko release plan --change minor --unit api
neko release plan --change major --unit api
neko release doctor
neko release doctor --unit api
neko release doctor --output json
neko release units
neko release units --output json
neko release pipeline --unit api
neko release pipeline --unit api --output json
neko release pipeline --unit api --verify-remote
neko release github-workflow-init --dry-run
neko release github-workflow-init --unit api
neko release github-workflow-init --path .github/workflows/release-api.yml
neko release ci-validate-context --unit api --version 2.4.0 --tag api/v2.4.0 --release-sha <full-commit-object-id>
neko release patch --unit api
neko release minor --unit api
neko release major --unit api
neko release resume --unit api
neko release resume --unit api --dry-run
neko release evidence
neko release evidence --family release-execution
neko release evidence --family release-execution --unit api --identity 0123abcd
neko release evidence-archive --family release-execution --identity <sha256> --digest-sha256 <sha256> --confirm-archive
neko release plugin-index
neko release plugin-index --check
neko release plugin-index --output-file build/plugin-index.json
neko release plugin-index --check --output json
```

`plugin-index` has three modes:

| Mode | Selection | Default stdout | `--describe` | `--verbose` | Core `--output json` |
| --- | --- | --- | --- | --- | --- |
| Raw render | no `--check` or `--output-file` | Exact pretty schema-v1 JSON (`--pretty=false` selects compact JSON) | Intentional no-op for raw table output | Intentional no-op for raw table output | Core public plugin-response JSON containing the raw artifact |
| Check | `--check` | Concise local validation result, repository/plugin counts, and next action | Source resolution, repository/plugin inventories, validation checks, and limitations | Safe validation phases | Core public plugin-response JSON with the established `data.items` fields |
| Persist | `--output-file <path>` | Concise successful write result with safe target label, formatting, counts, validation, and next action | Source/inventory/validation facts, write plan, outcome, and limitations | Safe construction, validation, target, atomic-write, and completion phases | Core public plugin-response JSON with the established `data.items` fields |

The raw artifact and Core public JSON are separate contracts. Default raw
stdout has no response envelope, metadata, timestamp, log, ANSI sequence, or
presentation declaration. Explicit `--output json` requests Core's public
response envelope; it never selects a file. Check and persist use structured
responses and keep their established `Status`, `Plugins`, `Repository`, and
persist-only `Output` machine rows.

For `plugin-index --output-file`, relative paths are resolved from the
repository root. Explicit absolute paths remain supported for CI or temporary
artifacts. Repository-contained output is blocked from overwriting release
config/state, recovery evidence, Git internals, or plugin manifest inputs.
Missing parents, overwrite mode preservation, and target-local atomic
replacement retain their existing behavior.

The former `plugin-index --output <path>` local spelling is not an alias.
`--output` is exclusively Core's response-format flag: `json` and `table`
follow Core rendering behavior, unsupported values are rejected before plugin
dispatch, and no fallback file is written. Use `--output-file` for persistence.

### Patch, Minor, Major, and Resume presentation

`patch`, `minor`, and `major` use one Release-owned human presentation
contract. Their established `data.items`, outcome variants, status/error
envelopes, and exit behavior remain the machine contract.

| Mode | Patch / Minor / Major | Resume |
| --- | --- | --- |
| default success | release change, unit, previous/resulting version, tag, executor/delivery, lifecycle and handoff result, next action | execution identity, journal phase, pending action, recovery status, Resume eligibility, retry safety, next action |
| default dry-run | the same release identity plus ordered principal operations, primary materialized files, explicit mutation boundary, blockers, and preview status | the recovery decision plus planned continuation, no-write boundary, refusal or eligibility, retry safety, and guidance |
| `--describe` | `Release Summary`, `Operations`, `Materialized Files`, `Source and Configuration`, declared release files, `Execution Evidence`, `Git and Handoff`, and `Limitations` | `Resume Summary`, `Planned Continuation`, `Recovery Journal`, `Local Git Evidence`, `Recovery Assessment`, `Continuation and Handoff`, and `Limitations` |
| `--verbose` | chronological source/unit, preflight, version/tag, materialization, Git, push, dispatch, handoff, and completion phases already owned by the selected V1/V2 path | chronological discovery, exact journal selection, local recovery evaluation, policy resolution, config/Git validation, selected continuation, push/dispatch/handoff, completion, or refusal phases |
| `--output json` | unchanged ReleaseCommandOutcome-derived domain rows; presentation declarations are excluded | unchanged ResumeCommandOutcome-derived domain rows or error envelope; presentation declarations are excluded |

Default failures keep the typed code and exact actionable reason visible.
Rejected or uncertain operations, ambiguous pending pushes, invalid local
evidence, conflicts, and required manual intervention are never hidden behind
`--describe`. Presentation-only failure text replaces absolute local paths
with a repository-local label; the established machine error message remains
unchanged.

Describe and verbose are independent. Describe performs no extra planning,
journal selection, recovery resolution, Git query, token lookup, or provider
call. Verbose reports the existing orchestration and does not select an
operation. Patch/Minor/Major dry-run invokes no release tool and creates no
journal, commit, tag, push, or dispatch. Resume dry-run stops after the
authoritative local assessment and performs no continuation.

Human file references are repository-relative or safe artifact labels. Logs
never print repository roots, developer paths, token values, authorization
headers, full config/journal payloads, raw provider bodies, or raw Git command
output. Narrow output retains identity, action/status, pending action, and
reason; unknown width uses deterministic vertical records. Redirected and
`NO_COLOR` output is ANSI-free.

V1 retains its real compatibility differences: one virtual `default` unit,
local delivery, `.release.neko.json`, and the configured legacy release tool.
Its established outcome does not retain V2 journal/push/dispatch evidence, so
describe states that outcome boundary instead of inventing evidence. V2 keeps
Neko-owned state/materialization/commit/tag/push and the configured GitHub
Actions handoff. No presentation helper owns lifecycle or recovery policy.

### Validate presentation

`neko release validate` validates the complete repository release source. In V2 this
means strict decoding and repository-wide validation of both
`.neko/release.config.json` and `.neko/release.state.json`, even when `--unit`
is supplied. `--unit` only focuses displayed V2 details.

Default human output is a concise `Release Configuration Validation` property
summary with status, source, schema, config/state paths, selected unit when
supplied, and configured-unit count. It does not include healthy unit details.
Global `--describe` adds the responsive `Validated Units` view with complete
per-unit facts plus `Validation Scope`; Validate remains local and read-only.

`--show` adds a responsive V2 unit table. Its essential columns are `Unit`,
`Version`, and `Kind`; optional columns are considered in `Executor`,
`Delivery`, `Workflow` order. Complete per-unit details follow the table in
optional display name, version, kind, working directory, tag prefix, executor,
delivery, workflow, and paths order. Paths render one per line, and plugin name,
manifest, asset prefix, and binary are appended only for plugin units. V1 uses
one virtual `default`
row with essential `Unit`, `Version`, and `Project type`, optional `Release
system`, and legacy-only details.

`--show` remains the compatibility switch for the established detailed human
and mode-sensitive JSON view. `--describe` without `--show` does not broaden
JSON. `--show --describe` renders the unit view once and adds only the
describe-only scope section. Validate owns no execution narration, so
`--verbose` is a deterministic no-op.

The existing Core semantic roles color only focused facts in an interactive
terminal: success status, emphasized unit IDs, informational versions and
plugin kinds, and informational unit-detail headings. Workflow paths,
materialized paths, and ordinary metadata stay neutral. Redirected output is
ANSI-free.

The flags are:

```text
--show  Display structured release configuration details and unit summaries
--unit  Focus displayed V2 unit details; the complete repository is still validated.
```

Human presentation metadata does not alter the established public JSON
`data.items` schema, values, or ordering.

`neko release github-workflow-init [--unit <unit-id>] [--path
<configured-path>] [--dry-run]` creates the canonical GitHub Actions Release V2
workflow without overwriting consumer content. All flags are optional. With no
selector, the V2 pair must contain one unique configured workflow path; shared
paths produce one central workflow. Multiple distinct paths require an exact
unit or path selection. `--path` must exactly match at least one configured
unit, and when combined with `--unit` it must match that unit's path.

The only accepted targets are configured, repository-root-relative
`.github/workflows/*.yml` or `.yaml` files directly below that directory.
Absolute paths, traversal, nested paths, protected release files, unsupported
names/extensions, target symlinks, and parent symlink escapes are rejected.

The command is create-only:

- missing target: atomically create exact canonical bytes with mode `0644`;
- byte-identical target: return success without rewriting;
- different target: return `WORKFLOW_TARGET_CONFLICT` with exit code `1` and
  preserve the file;
- `--dry-run`: make no write and return classification plus the complete
  generated YAML, including for a conflict.

Default human-readable output uses concise result/target/identity/write/guidance
fields; dry-run preview uses readable preformatted YAML. Describe adds the
complete structured identity, target comparison, validation, input, write
plan, and limitation sections without rendering the YAML twice. JSON returns
typed `target`, `classification`, `action`, `written`, `unchanged`, `dry_run`,
`contract_version`, `selected_unit`, `units_using_workflow`, and `guidance`;
`generated_content` appears only for preview. The command supports `table` and
`json`, not GitHub output mode. It reads no token, contacts no network, runs no
Git operation, and never commits, pushes, dispatches, publishes, or creates a
GitHub Release.

Generated contract version `1` installs pinned Neko CLI and Release Plugin
versions from repository variables, validates the exact four dispatch inputs
through `ci-validate-context`, and ends in a deliberately failing
consumer-owned extension point. Consumers own build systems, secrets,
permissions, publication, signing, deployment, release notes, and GitHub
Release creation. There is no provider, force, managed-update, or arbitrary
consumer-command flag.

Follow the [Release V2 GitHub Actions Golden Path](github-actions-golden-path.md) for the complete installation-through-publication workflow. See [Release V2 Examples](examples.md) for additional copy-ready init, unit-add, release, plugin-index, and plugin install/update flows.

V2 non-dry-run release commands are active only for `delivery: github-actions`. `local` is not a supported executable V2 delivery mode; existing V2 configs using it are rejected during validation before planning or execution. Dry-run output includes:

```text
unit
currentVersion
nextVersion
tag
executor
delivery
workflow
dispatch
dispatchRef
dispatchInputs
journalIdentity
journalLocation
workingDirectory
unitRoot
stateChange
materializedFiles
knownReleaseFiles
plannedReleaseCommit
plannedTag
plannedPushOrder
toolOwnership
v2GitOwnership
stateCommitGuarantee
executorStart
```

`neko release plan --change <patch|minor|major> [--unit <unit>]` is a dedicated
local plan inspection command. Default human output keeps the resolved release
identity, readiness, every blocker, the ordered principal operations, mutation
boundary, and planned materialized files needed to understand the preview.
Global `--describe` adds source ownership, complete planning/preflight facts,
complete materialized and known-file facts, and individually labeled
assumptions and limitations. Absolute repository paths are not rendered.
`--verbose` adds no query narration. The typed facts and established JSON
`data.items` order remain unchanged. The command does not read tokens, inspect
remotes or journals, write files, mutate Git, dispatch workflows, publish, or
start executors. It remains separate from lifecycle `--dry-run`.

`neko release doctor [--unit <unit-id>] [--verify-remote]` is a Release V2
integration inspection. By default it remains strictly local, offline, and
token-free. It checks all configured units and unique
workflow paths; selecting a unit still retains every unit sharing that
workflow. It returns `ready`, `ready_with_warnings`, or `not_ready`, with exit
code `1` for `not_ready`. JSON exposes `readiness`, severity and verified
counts, ordered unit/workflow facts, additive ordered `verifications`, and
ordered diagnostics. Additive `remote_verification` records whether remote
verification was requested, whether it was complete, partial, or unavailable,
and verified/unresolved/failed counts. Remote states distinguish `missing`,
`mismatch`, `not_attempted`, `unavailable`, `unauthorized`, `rate_limited`, and
`unsupported`. References are repository-relative and collections are not
`null`.

`--verify-remote` enables only exact GitHub `GET` reads. Repository metadata is
anonymous-first; a token is resolved only for a private-resource retry or
protected Actions variable, secret-name, or policy metadata. The Doctor checks
repository/default-branch identity, exact workflow bytes and enabled state,
only recognized version variables, only referenced custom-secret names,
Actions policy, and exact locally derived tags, releases, and assets. It does
not list arbitrary variables or secrets, query secret values, use latest/fuzzy
discovery, automatically retry, or select workflow runs without a durable exact
run ID. It never dispatches, uploads, publishes, writes settings, mutates Git,
or changes config/state/workflows/journals/Evidence.

Human-readable output presents readiness and counts first, followed only by
actionable diagnostics and verification facts. Global `--describe` adds
`Complete Diagnostics`, `Verification Facts`, configured units/workflows, safe
evidence, guidance, deferred facts, and limitations. Essential finding columns
are `Check`, `Status`, and `Scope`; optional `Subject`, `Evidence`, and
`Guidance` are admitted while width permits. Details retain the workflow path, message, and
remediation and wrap at known widths. Narrow output can use vertical records,
and width-unknown or non-terminal output uses the deterministic vertical
fallback. These layout choices do not alter JSON or raw JSON.

Diagnostics use the closed severities `error`, `warning`, `recommendation`,
and `not_verifiable`. Their stable fields are `severity`, `scope`, optional
`unit`, optional `workflow`, `code`, `message`, and `remediation`. Ordering is
deterministic by severity, scope, unit, workflow, code, and message. A generated
canonical workflow is recognized byte-for-byte but remains `not_ready` while
its deliberately failing consumer placeholder is present. A structurally
equivalent manual workflow is supported; unsupported custom build/publication
shapes remain explicitly limited.

Supported repository workflows expose five verified fact categories per
workflow: `consumer_structure`, `goreleaser_configuration`,
`installation_wiring`, `credential_wiring`, and `publication_identity`.
Offline boundary facts retain `remote_workflow_identity`,
`repository_variable_values`, and `dispatch_authorization` as not verifiable.
Successful explicit remote verification resolves the first two; exact dispatch
authorization remains mutation-required.
Credential names are classified and scoped without reading values. Focused
installation/publication checks read only supported local workflow,
GoReleaser, installer, manifest, registry, manager, and plugin-index contracts;
no commands are executed. Remote success still does not prove runner success,
runtime installation/loading, credential value validity, publication
acceptance, or exact dispatch authorization. See
[Remote Doctor verification](integration-doctor-remote-verification.md).

Permission diagnostics distinguish workflow defaults from job overrides. An
omitted job declaration inherits the workflow permission set; an explicit job
declaration replaces it. Workflow-level writes, `write-all`, unsupported
permission forms, and unjustified job writes emit the warning
`PERMISSIONS_BROAD`. A job-scoped `contents: write` is justified only by a
same-job non-snapshot, non-skip-publish GoReleaser release or direct
`gh release create`/`gh release upload`. A job-scoped
`packages: write` is justified only by a literal GitHub Container Registry
push. No OIDC publisher is structurally recognized. Names, paths, secret
presence, and publish-looking no-op commands never count as evidence.

`neko release units` is the flat, Release V2-only inventory command. It has no
command-specific flags and lists every config or state unit in canonical unit
ID order. Current versions come only from state. Canonical versions are exposed
as `version`; a safe raw invalid value remains `configured_version`. Tag facts
come from canonical `TagSpec`: `tag_shape` is version-independent and
`configured_tag` uses only the validated current state version. Neither field
claims that a Git tag exists, and the command never calculates a next version.

The unit alignment classification is closed: `aligned` means config and state
exist and canonical fields validate; `config_only` and `state_only` identify a
missing pair member; `invalid` identifies malformed or conflicting unit facts.
Unit issues contain `severity`, `unit`, `code`, `message`, and `remediation`.
Stable unit codes are `UNIT_STATE_MISSING`, `UNIT_CONFIG_MISSING`,
`UNIT_VERSION_INVALID`, `UNIT_TAG_PREFIX_INVALID`,
`UNIT_TAG_PREFIX_CONFLICT`, `UNIT_EXECUTOR_INVALID`,
`UNIT_DELIVERY_INVALID`, `UNIT_WORKFLOW_PATH_INVALID`, and
`UNIT_CONFIG_INVALID`.

JSON `data` contains `status`, `summary`, `units`, `workflow_paths`, and an
optional `source_issue`. `status` is `valid`, `has_issues`, or
`source_invalid`. Summary contains numeric `total`, `aligned`, `incomplete`,
`invalid`, and `workflow_paths` counts plus boolean `source_usable`. Unit rows
contain only stable inventory facts: `id`, optional `display_name`, state
version fields, tag fields, executor/delivery/workflow/working-directory
fields, `alignment`, `issues`, and `issue_codes`. Empty issue lists remain JSON
arrays; absent optional facts are omitted. Units sort by ID, issues by severity,
unit, code, and message, and distinct workflow paths lexically.

Human-readable default output keeps the useful unit inventory. `Unit`,
`Version`, and `Status` are essential; `Kind`, executor, delivery, and concise
issue codes are optional, and every issue also appears in an actionable
`Issues` table. Global `--describe` adds `Unit Details` with source ownership,
workflow/tag/plugin metadata and complete issue evidence plus `Limitations`.
Unknown width uses deterministic vertical records, and invalid units remain
visible. `--verbose` is a no-op. `valid` exits `0`; `has_issues` and
`source_invalid` exit `1`. Missing, malformed, unsupported, V1-only, mixed, and
recovery-blocked source states are nil-Go-error structured responses with a
stable source issue. Source codes are `V2_SOURCE_INSPECTION_FAILED`,
`MIXED_RELEASE_SOURCES`, `V1_SOURCE_UNSUPPORTED`, `V2_SOURCE_MISSING`,
`V2_CONFIG_INVALID`, `V2_STATE_INVALID`, `V2_SCHEMA_UNSUPPORTED`,
`V2_RECOVERY_BLOCKED`, `V2_CONFIG_MISSING`, `V2_STATE_MISSING`, and
`V2_SOURCE_EMPTY`.

The command reads only strict local V2 config/state and the existing pair
recovery readiness marker. It does not parse workflow YAML, invoke Doctor
workflow inspection, inspect Git or tags, read tokens, contact a network, read
journals or Evidence, inspect build files, plan releases, execute releases, or
write anything. `release doctor` remains the workflow-readiness command;
`release pipeline` separately describes one configured execution path.

### Pipeline inspection

`neko release pipeline [--unit <unit>] [--verify-remote]` is the Release
V2-only, read-only view of the configured pipeline for one unit. The default
path is local, offline, and token-free; remote verification is explicit:

This section is the canonical Pipeline Inspection contract. Other Release
documentation summarizes it and links here rather than redefining the schema,
status, presentation, safety, and exit rules.

```bash
neko release pipeline --unit cli
neko release pipeline --unit plugin-release
neko release pipeline --unit plugin-ui
neko release pipeline --unit cli --describe
neko release pipeline --unit cli --output json
neko release pipeline --unit cli --verify-remote
neko release pipeline --unit cli --verify-remote --describe
neko release pipeline --unit cli --verify-remote --output json
```

The command uses the existing V2 unit-selection policy: `--unit` is required
for a multi-unit repository and may be omitted only when the repository has one
unit. It has no `--all`, journal-selection, repair, planning, resume, retry, or
pipeline-specific output flag. `--verify-remote` must be a boolean and is the
only remote mode. V1, unknown units,
malformed requests, and unsupported executor, delivery, or workflow
configurations return typed failures with exit code `1`. Structurally invalid
runtime evidence is returned as an `invalid` inspection result with exit code
`1`. Valid observations, including `active`, `resumable`, `completed`,
`blocked`, `uncertain`, and `rejected`, exit `0`.

The result contains the selected unit, current configured version and tag,
repository-relative working directory and workflow path, configured
materialization files, canonical dispatch inputs, selected release tool,
ordered consumer-operation facts, publication and plugin-registry summaries,
and the full configured stage list. It adds execution-journal,
dispatch-journal, local Git, recovery, resume-eligibility, retry-safety, and
manual-intervention sections plus `verification.summary` and ordered
`verification.facts`. Local branch and HEAD are inspected; tracking and
lifecycle remote freshness remain explicitly `remote_not_inspected`. The
configured tag is derived only from the current state version and tag prefix.
No next version, next tag, proposed commit, or publication identity is
calculated.

Each stage has a stable ID, label, owner, execution location, strongest
mutation class, static `configured` status, source, and separate runtime
observation. Runtime values are `not_observed`, `not_started`, `pending`,
`confirmed`, `blocked`, `unknown`, `rejected`, or `invalid`. Confirmation comes
from the existing monotonic execution-journal phase order; pending operations
come from the journal's durable pending action. Dispatch submission is mapped
only from the exactly linked dispatch journal. Consumer-workflow stages remain
unobserved because verification facts never become runtime progress. Even when
`--verify-remote` confirms workflow identity or enabled state,
`progress_inspection.remote_state_inspected` remains `false`: Pipeline does not
inspect a durable workflow run or publication result. A stage marked
`configured` is never itself a runtime-completion claim.

The root lifecycle IDs are ordered as executed by the production coordinator:

1. `source-unit-resolution`
2. `release-context-planning`
3. `dispatch-token-resolution`
4. `release-file-planning`
5. `release-preflight`
6. `execution-journal-preparation`
7. `release-file-materialization`
8. `selected-unit-state-write`
9. `known-release-file-staging`
10. `release-commit-creation`
11. `unit-tag-creation`
12. `workflow-request-preparation`
13. `release-commit-push`
14. `unit-tag-push`
15. `workflow-request-submission`
16. `handoff-confirmation`

Recognized consumer IDs follow in their literal workflow order:
`canonical-context-validation`, conditional `plugin-manifest-validation`,
`consumer-tests`, `release-tool-configuration-validation`, `snapshot-build`,
`consumer-worktree-validation`, `release-artifact-packaging`,
`release-publication`, conditional `plugin-index-generation`, and conditional
`plugin-index-publication`. Only operations actually present in the selected
workflow are included.

Human output is titled `Release Pipeline Inspection`. Default output keeps the
`Summary` concise and adds `Findings` only when an actionable problem exists.
Each finding retains a readable check/problem, status, sanitized reason, and a
safe subject when useful. It covers failed, unauthorized, rate-limited, and
confidence-affecting unavailable verification facts; malformed/conflicting
journals; multiple unresolved executions; rejected/unknown dispatch; invalid,
missing, or mismatched local commit/tag evidence; blocked/uncertain recovery;
and manual-intervention reasons. Healthy, not-checked, intentionally deferred,
and mutation-required inventories stay summarized unless they directly explain
an actionable problem.

Global `--describe` adds the complete responsive `Verification Facts` table,
grouped `Configured Pipeline`, applicable safe execution/dispatch/local-Git/
recovery evidence, and complete `Limitations`. Verification uses essential
`Check`, `Status`, and `Scope` columns with optional `Subject` and `Evidence`;
the old `Category`/`Class`/`Source` display labels are not the current human
contract. Stages use essential `Stage`, `Runtime`, and `Owner` with optional
`#`, `Location`, `Mutation`, and `Evidence`. Width-unknown output uses
deterministic vertical records. Semantic color is interactive-terminal-only,
and redirected output is ANSI-free.

Global `--verbose` is a deterministic no-op because this read-only inspection
has no useful chronological execution narration. Combining it with
`--describe` is therefore equivalent to describe alone. Neither flag changes
command exit behavior or enables a Pipeline capability.

JSON keeps the existing response envelope and `schema_version: 1`. The original
`status`, `unit`, `release`, `repository`, `workflow`, `stages`,
`progress_inspection`, and `limitations` sections remain. Append-only sections
are `execution`, `dispatch`, `local_git`, `recovery`, `manual_intervention`,
and `verification`; arrays are never `null`, ordering is deterministic, and
presentation metadata, absolute paths, credentials, and raw journals are
excluded. JSON is already the complete machine contract: `--describe --output
json` is identical to `--output json`, and verbose/describe presentation
choices do not create another schema.

The stable `data` object contains exactly these top-level fields in schema
version 1: `schema_version`, `status`, `unit`, `release`, `repository`,
`workflow`, `stages`, `progress_inspection`, `execution`, `dispatch`,
`local_git`, `recovery`, `manual_intervention`, `verification`, and
`limitations`. Its nested contract is:

| Object | Fields |
| --- | --- |
| `unit` | `id`, optional `display_name`, `kind`, `executor`, `delivery`, `configured_version`, `working_directory` |
| `release` | `configured_version`, `tag_prefix`, `configured_tag`, `materialized_files[]` (`path`, `reason`) |
| `repository` | `source_generation`, `local_branch`, `local_head`, `tracking` |
| `workflow` | `path`, `delivery`, `required_inputs[]`, `release_tool`, `consumer_operations[]`, `publication`, `plugin_registry` |
| `stages[]` | `id`, `label`, `owner`, `location`, `mutation`, `configuration_status`, `source`, optional `conditional_reason`, `runtime_status`, optional `runtime_evidence`, `runtime_reason`, `runtime_identity`, `runtime_confirmed_at` |
| `progress_inspection` | `execution_progress`, `journals_inspected`, `resume_eligibility_evaluated`, `remote_state_inspected` |
| `execution` | `present`, `identity`, `journal_count`, `unresolved_count`, `validity`, `state`, `pending_action`, `terminal`, optional `created_at`, `updated_at`, `journal_reference`, `observations[]` |
| `execution.observations[]` | `identity`, `reference`, optional `state`, `unresolved`, `valid`, optional `problem` |
| `dispatch` | `present`, `identity`, `journal_count`, `unlinked_count`, `correlation`, `state`, optional `workflow_path`, `run_id`, `observations[]` |
| `dispatch.observations[]` | `identity`, `reference`, optional `state`, `correlation`, `valid`, optional `problem` |
| `local_git` | `scope`, `remote_freshness`, `branch`, `head`, `index_state`, `worktree_state`, `expected_commit`, `commit_exists`, `commit_content_verified`, `expected_tag`, `tag_exists`, optional `tag_target`, `tag_matches_expected_commit`, `head_contains_expected_commit`, `index_contains_recovery_evidence`, `worktree_contains_recovery_evidence`, `consistent`, optional `problem` |
| `recovery` | `evaluated`, `classification`, `safe_to_continue`, `resume_eligible`, optional `resume_operation`, `resume_refusal`, `retry_safety`, `manual_intervention_required`, optional `guidance`, `reasons[]` |
| `manual_intervention` | `required`, `reasons[]` |
| `verification.summary` | `status`, `local_status`, `remote_status`, `remote_requested`, `remote_attempted`, `partial`, `verified`, `unresolved`, `failed`, `not_checked` |
| `verification.facts[]` | `id`, `category`, `class`, `status`, `subject`, `evidence`, `source`, `scope`, `references[]`, optional `unit`, `workflow` |

All established arrays encode as `[]`, never `null`. Root stages preserve the
authoritative lifecycle order; consumer operations preserve literal workflow
order; execution and dispatch observations sort by identity then safe relative
reference; verification facts sort by neutral identity fields; references are
deduplicated and sorted. Optional fields above are omitted only when empty;
required objects, booleans, counts, states, and aggregate fields remain
present. The standard public JSON envelope retains `status`, `metadata`,
`data`, optional `error`, `renderer_hint`, and any already captured `logs`.
Presentation properties, tables, group keys, notes, style roles, terminal
width, ANSI, raw journals, HTTP bodies, credentials, secrets, and absolute
developer paths are not part of `data`.

Each verification fact has a stable Pipeline-owned `id`, `category`, `class`,
`status`, `subject`, `evidence`, `source`, `scope`, non-null `references`, and
optional `unit`/`workflow`. Classes are `local`, `remote`,
`runtime_required`, or `mutation_required`. Statuses are `verified`, `failed`,
`unavailable`, `unauthorized`, `rate_limited`, `not_checked`, or `unresolved`.
Fact IDs derive only from immutable neutral identity fields, never evidence
messages, timestamps, array position, absolute paths, credentials, or terminal
presentation.

The verification summary is separate from Pipeline lifecycle status. It
reports `status`, `local_status`, `remote_status`, `remote_requested`,
`remote_attempted`, `partial`, and verified/unresolved/failed/not-checked
counts. A failed or partial verification does not change `ready`, `active`,
`resumable`, `completed`, `blocked`, `uncertain`, `rejected`, or `invalid`.

Default inspection reads the local V2 source pair, its recovery-readiness
marker, repository-confined workflow and focused local Doctor inputs, execution
and dispatch journals below the Git common directory, local Git
objects/refs/index/worktree, and known-file recovery evidence. It uses the
Doctor's neutral fact API directly, never its command handler, diagnostics,
readiness policy, response mapper, or presentation. Default inspection
constructs no HTTP client and never resolves `GITHUB_TOKEN`.

`--describe` is consumed only by Core response rendering and is not included in
the plugin request. It therefore cannot select remote verification, resolve a
token, call HTTP, fetch refs, or mutate anything. Only the command-local
`--verify-remote` boolean enables the existing explicit remote branch.

`--verify-remote` delegates to Doctor's existing single bounded GitHub reader:
exact GETs only, 12-second timeout, 1 MiB response cap, redirects refused, and
no automatic retry. Repository reads are anonymous-first; the existing lazy
resolver may read `GITHUB_TOKEN` once for a private-resource retry or protected
Actions metadata. No token, authorization header, secret value, private body,
or absolute path enters Pipeline output. Neither mode writes a journal or file,
changes cwd, stages, commits, tags, resets, cleans, pushes, dispatches, executes
a release tool, resumes, retries, repairs, uploads, or publishes.

Runtime status is a read-only projection. `active` means locally recorded
incomplete lifecycle evidence, not a live process. `resumable` is emitted only
when the existing Resume policy permits a named continuation. `completed`
means exact local commit/tag evidence plus an accepted workflow handoff; it
does not mean remote publication completed. `blocked` requires manual work
under existing recovery policy, `uncertain` preserves ambiguous external
effects, `rejected` reflects the exactly correlated terminal dispatch, and
`invalid` denotes malformed, contradictory, unlinked, or non-unique evidence.

The complete frozen machine vocabularies are:

- lifecycle: `ready`, `active`, `resumable`, `completed`, `blocked`,
  `uncertain`, `rejected`, `invalid`;
- runtime stage: `not_observed`, `not_started`, `pending`, `confirmed`,
  `blocked`, `unknown`, `rejected`, `invalid`;
- verification fact status: `verified`, `failed`, `unavailable`,
  `unauthorized`, `rate_limited`, `not_checked`, `unresolved`;
- verification class: `local`, `remote`, `runtime_required`,
  `mutation_required`;
- verification summary status: `verified`, `partial`, `failed`, `unresolved`,
  `not_checked`;
- remote verification aggregate: `not_requested`, `complete`, `partial`,
  `unavailable`;
- static stage configuration: `configured`.

Verification facts and remote availability never rewrite lifecycle, recovery,
resume, or runtime stages. Recovery and resume consume the already selected
local evidence and existing authoritative policies; Pipeline itself never
continues an operation. `completed` requires exact local commit/tag evidence
and accepted handoff evidence, not a completed build, upload, release, registry
update, or publication.

Exit code `0` covers every structurally valid inspection result, including
`ready`, `active`, `resumable`, `completed`, `blocked`, `uncertain`, and
`rejected`, regardless of verification fact status. Exit code `1` is reserved
for typed invalid requests/sources/configuration and a successful structured
inspection whose selected local runtime evidence is `invalid`. Default,
`--describe`, `--verbose`, their combination, and JSON preserve that same
policy.

The support fixture contract covers untouched/ready, active, resumable,
completed, blocked, uncertain, rejected, invalid, malformed and conflicting
journals, multiple unresolved executions, missing or mismatched commits and
tags, rejected or unknown dispatch, local failure, unauthorized/unavailable/
rate-limited/partial remote verification, and manual intervention. Repository
fixtures use isolated temporary Git repositories; remote fixtures use only
test-owned loopback HTTP listeners; presentation fixtures contain only safe
relative references. The repository's `cli`, `plugin-release`, and `plugin-ui`
units are the dogfood cases. Fixtures never depend on real refs, user plugin
directories, tokens, absolute developer paths, or external network state and
are safe to interpret in support output.

The supported release archive matrix derived from the focused GoReleaser
configs and current Go target support is Darwin `amd64`/`arm64`, Linux
`386`/`amd64`/`arm64`, and Windows `386`/`amd64`/`arm64`. Archive labels map
`amd64` to `x86_64` and `386` to `i386`; Windows uses `zip` where configured,
and the other focused archives use `tar.gz`. Darwin/i386 is not a supported Go
target and must not be expected by Doctor, publication verification, or the
installer. A missing Darwin/i386 archive is therefore consistent with the
supported build matrix rather than evidence of a missing published artifact.

Unsupported or read-only boundaries:

- Existing V2 configs with `delivery: local` are rejected with a clear unsupported-delivery validation error.
- Execution journals and dispatch journals are not written by dry-run.
- Public V2 local executor execution is not configured because no supported executor exposes a safe publish-only boundary.

For `delivery: github-actions`, V2 config must include `workflow: ".github/workflows/<file>.yml"` or `workflow: ".github/workflows/<file>.yaml"`. `neko release validate --show` displays the workflow only after repository-aware validation confirms that the file exists and stays inside `.github/workflows/`.

The execution journal records V2 release phases and recovery evidence under the Git common directory. The dispatch contract targets GitHub.com remotes only, uses `GITHUB_TOKEN` with repository Actions write permission, sends the existing unit tag as `ref`, and sends exactly four inputs: `unit`, `version`, `tag`, and `release_sha`. No public standalone dispatch or retry command exists.

`neko release evidence` is read-only, offline, and token-free. Its default human output is a concise `Evidence Summary` plus an actionable inventory whose essential fields are family, exact identity, state, classification, and action. Unit, version, tag, and linked execution are optional responsive fields. Malformed or conflicting evidence, ambiguous or unlinked evidence, active/resumable/uncertain/terminal/manual-recovery classifications, and every retained diagnostic remain visible without `--describe`. Narrow output keeps the essential fields; width-unknown, piped, and redirected output uses deterministic vertical records.

Global `--describe` adds focused safe sections for execution evidence, dispatch evidence, linkage, journal-retained local Git facts, classification reasons, recovery relevance, and limitations. Digests and repository-relative source labels appear there; absolute roots, raw journals, credentials, raw Git output, and remote response bodies do not. Evidence does not re-inspect local Git or infer missing/mismatched commit or tag outcomes: authoritative local recovery checks remain owned by Resume and Pipeline. Global `--verbose` is an intentional no-op, so default and verbose human output are identical.

`neko release evidence --identity <prefix>` applies family and unit filters before selecting exactly one record. Prefixes must be 8-64 lowercase hexadecimal characters; uppercase input is rejected rather than normalized, full identities are accepted, and zero or ambiguous matches fail. The legacy JSON compatibility shape is unchanged for both summary and identity-filtered results: `data.items`, typed `data.evidence`, and `data.diagnostics` retain their established fields, casing, duplication, values, ordering, and nullability. Describe and verbose metadata is excluded from JSON, and all global presentation modes leave domain data and status unchanged.

`neko release evidence-archive` is the separate guarded mutation capability. It supports only `archive-completed` for completed `release-execution`, `v1-compensation`, and `v2-pair-recovery` evidence and requires `--family`, the exact 64-character `--identity`, the current `--digest-sha256`, and `--confirm-archive`; identity prefixes are inspection-only. Default success output reports family, identity, confirmation, digest match, archive result, safe source/target labels, and the next action. Guard failures preserve their specific actionable error and exit `1`; successful archival exits `0`. `--describe` adds validation facts, the guarded write plan, and limitations. `--verbose` reports chronological validation, lookup, digest, target, write, verification, source-removal, completion, or refusal phases without full digests or absolute paths. The operation writes and verifies an exact private archive before removing only the selected completed source; its established conflicts, idempotency, rollback limits, and JSON remain unchanged.

Neither command repairs, retries, resumes, infers remote state, commits, tags, pushes, dispatches, or archives dispatch/migration evidence.

`neko release plugin-index` generates the public plugin registry index from V2 plugin units, `.neko/release.state.json`, and plugin manifests. The generated `plugin-index.json` is not committed as source and is not published by this command. Plugin release workflows publish or replace it as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases. Runtime `neko plugin available`, `install`, and `update` use that asset as the source of truth and do not use `/releases/latest` or release-prefix fallback discovery for plugin discovery.

### CI release-context validation

`neko release ci-validate-context` validates one dispatched Release V2 context
before a consumer builds or publishes:

```bash
neko release ci-validate-context \
  --unit api \
  --version 2.4.0 \
  --tag api/v2.4.0 \
  --release-sha <full-lowercase-commit-object-id>
```

All four flags are required; there are no command-specific optional flags. The
command requires a complete valid V2 config/state pair with no V1 conflict or
unresolved pair-recovery evidence. It resolves the explicit unit, compares the
exact authoritative state version and canonical tag, verifies that
`release_sha` is a full local commit object ID for the repository's object
format, and requires checked-out `HEAD` and the peeled local tag target to equal
it. Lightweight and annotated tags are accepted. Detached HEAD is accepted when
it matches. Missing or incomplete local tag history fails with guidance; the
command never fetches.

Default output is a concise `Validated Release Context` summary containing the
unit, version, tag, release commit and aggregate local Git consistency. On
failure, every independently available failed check is shown; version and tag
mismatches are evaluated together while the existing primary error code and
exit remain stable. Global `--describe` adds every executed `Context Check`,
the complete resolved context, GitHub-output mapping, and local/token-free/
read-only limitations. `--verbose` is a deterministic no-op. Long values wrap
within the actual width, and narrow or width-unknown tables use vertical
records. `--output json` returns the unchanged response envelope with
deterministic `data` keys:

```text
valid
unit
display_name
version
tag_prefix
tag
release_sha
working_directory
executor
delivery
workflow
git_object_format
head_matches
tag_target_matches
```

`valid`, `head_matches`, and `tag_target_matches` are JSON booleans. All other
data values are canonical strings. Presentation and integration transport
metadata are excluded from public JSON.

GitHub Actions uses an explicit command-file destination:

```bash
neko release ci-validate-context \
  --unit "$RELEASE_UNIT" \
  --version "$RELEASE_VERSION" \
  --tag "$RELEASE_TAG" \
  --release-sha "$RELEASE_SHA" \
  --output github \
  --github-output-file "$GITHUB_OUTPUT"
```

The stable step outputs, in order, are `unit`, `display_name`, `version`,
`tag_prefix`, `tag`, `release_sha`, `working_directory`, `executor`, `delivery`,
and `workflow`. Empty values, Unicode, spaces, newlines, carriage returns, and
delimiter-like lines are encoded without shell evaluation. The destination is
never inferred from ambient environment, so outside GitHub Actions callers can
use human/JSON output or explicitly supply a pre-created command file.

Validation mismatches return a structured error response and nonzero CLI exit.
Stable command codes are:

```text
INVALID_CONTEXT_INPUT
INVALID_RELEASE_SHA
UNSUPPORTED_RELEASE_SOURCE
V2_CONTEXT_SOURCE_MISSING
V2_CONTEXT_SOURCE_CONFLICT
V2_CONTEXT_RECOVERY_BLOCKED
V2_CONFIGURATION_INVALID
V2_STATE_INVALID
V2_CONFIG_STATE_MISMATCH
V2_CONTEXT_SOURCE_INVALID
RELEASE_UNIT_NOT_FOUND
RELEASE_VERSION_MISMATCH
RELEASE_TAG_MISMATCH
GIT_REPOSITORY_UNAVAILABLE
GIT_OBJECT_FORMAT_UNSUPPORTED
RELEASE_SHA_NOT_COMMIT
HEAD_UNAVAILABLE
HEAD_MISMATCH
TAG_HISTORY_UNAVAILABLE
RELEASE_TAG_MISSING
TAG_TARGET_INVALID
TAG_TARGET_MISMATCH
```

Core output failures use `GITHUB_OUTPUT_DESTINATION_UNAVAILABLE` or
`GITHUB_OUTPUT_ENCODING_FAILED`. Safe messages omit raw Git output, filesystem
internals, and secrets. The command reads no token, contacts no network, writes
no release files or journals, and mutates no Git worktree, index, or ref.

## Resume

`resume` continues only an existing unresolved V2 GitHub Actions execution journal. It never calculates a new version, chooses a new tag, or blindly retries uncertain push or dispatch outcomes.

```bash
neko release resume --unit api
neko release resume --unit api --dry-run
neko release resume --unit api --dry-run --describe
neko release resume --unit api --dry-run --verbose
neko release resume --unit api --dry-run --describe --verbose
neko release resume --unit api --dry-run --output json
```

Default output is a recovery decision, not a generic execution result. It
keeps the exact refusal or recovery status, journal phase, pending action,
eligibility, retry safety, manual guidance, and next safe action visible.
Dry-run adds the planned continuation and explicit no-write boundary.
`--describe` adds the safe journal identity/path, retained release identity,
local Git assessment boundary, complete recovery decision, continuation and
handoff facts, and limitations. `--verbose` follows actual discovery,
selection, assessment, validation, continuation, and refusal/completion order.

The presentation mapper does not infer a continuation. `resolveResumeRecovery`
and `resolveResumeDispatch` remain the only policy owners, and the existing
selectors choose exactly one named operation. Missing/malformed/conflicting
journals, multiple unresolved executions, mismatched commit/tag evidence,
ambiguous pending pushes, rejected/unknown dispatch, completed handoff, and
manual-intervention outcomes retain their established error, status, retry,
JSON, and exit semantics.

## Unit Flag

`--unit <unit-id>` is available on:

```text
init
unit-add
patch
minor
major
plan
doctor
pipeline
ci-validate-context
github-workflow-init
resume
history
contributors
validate
evidence
```

The command-local flag matrix above is authoritative for presence and manifest
requiredness. Domain selection can add a conditional requirement: unit-bound
commands require `--unit` when a V2 repository defines multiple eligible
units. `init` and `unit-add` use it as the unit being created;
`ci-validate-context` always requires it. `validate` is not unit-bound: it
validates the complete repository and treats `--unit` only as a presentation
focus. Doctor may retain shared-workflow scope even when a unit filter is
supplied.

## Migrate

`migrate` has one flag:

```text
--dry-run
```

It migrates only `.release.neko.json` in the Git root to a V2 `default` unit. It does not migrate nested V1 files, infer multiple units, change tag prefixes, or run a release.

Default human output is a concise source/destination/readiness/write summary.
Describe exposes the normalized source and resolved V2 facts, generated
artifact summaries, exact ordered plan, archive/journal decisions, validation,
write outcome, and limitations. Dry-run logs contain planning phases and an
explicit no-write result, never write-completed claims. Actual logs announce
only completed write, verification, archive, and journal phases. JSON retains
the complete generated artifact data and existing outcome variants.
