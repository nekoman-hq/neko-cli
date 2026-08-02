# Neko Release

> **Audience:** Users evaluating or operating the Neko Release Plugin.
>
> **Purpose:** Serve as the canonical Release documentation entry point and route each task to its authoritative document.

Neko Release provides version planning, repository validation, release-unit
state, GitHub Actions handoff, evidence inspection, and guarded recovery. V2 is
the canonical architecture for new repositories. V1 remains supported
compatibility for repositories that still use `.release.neko.json`.

## Product boundary

Neko CLI owns release policy and local coordination:

- configuration and version state;
- release-unit selection and next-version calculation;
- required version materialization;
- the exact release commit and unit tag;
- push order and workflow dispatch;
- execution and dispatch journals;
- read-only inspection and guarded Resume.

Consumer repositories own product-specific work in their workflows:

- build and test commands;
- artifact production and signing;
- GitHub Release creation;
- asset publication;
- environment-specific credentials and permissions.

An accepted workflow dispatch is a completed Neko-to-GitHub handoff. It is not
evidence that the workflow or publication succeeded.

## Start here

| Need | Canonical document |
| --- | --- |
| Commands, flags, output, and exits | [Release command reference](cli-reference.md) |
| V2 config, state, units, and tags | [Configuration and state](configuration.md) |
| Planning, materialization, Git, push, and delivery | [Release lifecycle](lifecycle.md) |
| Workflow scaffold, dispatch, credentials, and Doctor | [GitHub Actions delivery](github-actions-delivery.md) |
| End-to-end repository setup | [GitHub Actions golden path](github-actions-golden-path.md) |
| Execution/dispatch journals and Resume | [Journals and recovery](journals-and-recovery.md) |
| V1 and V2 support boundary | [Compatibility](compatibility.md) |
| Convert V1 to V2 | [V1 to V2 migration](migration-v1-to-v2.md) |
| Copyable scenarios | [Examples](examples.md) |
| Explicit remote Doctor verification | [Integration Doctor remote verification](integration-doctor-remote-verification.md) |

## V1 and V2

V1 reads `.release.neko.json`, exposes one virtual `default` unit, and keeps its
legacy executor adapters. It is supported compatibility, not the model for new
configuration.

V2 reads `.neko/release.config.json` and `.neko/release.state.json`, supports
multiple release units, and uses explicit GitHub Actions delivery. Root V1 and
V2 files cannot act as competing authorities. Use `neko release migrate` for
the supported transition.

## Inspection and mutation

Inspection commands have deliberately different responsibilities:

- `units` shows configured units and their declared contracts.
- `plan` calculates a release without mutation.
- `pipeline` projects local lifecycle/evidence state; it is not a lifecycle engine.
- `doctor` performs read-only readiness checks and never repairs files or workflows.
- `evidence` reads journal evidence.
- `evidence archive` is a separate guarded local mutation.

Mutation commands are explicit:

- `init`, `unit-add`, and `migrate` write Release configuration/state under their create or migration contracts.
- `github-workflow-init` is create-only and never overwrites differing content.
- non-dry-run V2 release commands create the release state/commit/tag, push, and dispatch.
- `resume` continues one exact unresolved execution; it does not create a new release.

## Current limitations

- Executable V2 local delivery is unsupported; V2 uses `github-actions`.
- GitHub Enterprise remotes are rejected by dispatch target validation.
- No public standalone dispatch or retry command exists.
- Dispatch acceptance does not track durable workflow-run or publication completion.
- Unknown push or dispatch outcomes require operator inspection and are not retried automatically.

These are present product boundaries, not commitments to additional behavior.

## Repository implementation references

Public behavior is described in this directory. Contributors should use the
[Release implementation architecture](../../plugin/release/docs/architecture/current-state.md),
[package ownership](../../plugin/release/docs/architecture/package-ownership.md),
and [architecture constraints](../../plugin/release/docs/architecture/architecture-decisions.md).
Completed or superseded rationale is isolated in the
[Release history](../../plugin/release/docs/history/README.md).

## Canonical guide

For a complete consumer-repository setup and operator sequence, use the
[GitHub Actions golden path](github-actions-golden-path.md).
