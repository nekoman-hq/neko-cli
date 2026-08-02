# Release Plugin Architecture Constraints

> **Audience:** Maintainers evaluating a Release Plugin design or dependency change.
>
> **Purpose:** Record adopted architecture constraints and unsupported boundaries without duplicating runtime detail.

Detailed behavior lives in [Current Architecture](current-state.md), package
direction in [Package Ownership](package-ownership.md), and compatibility gates
in [V1 Compatibility Policy](v1-compatibility-policy.md). Completed rationale
is isolated in the [numbered history](../history/README.md).

## Adopted constraints

- V1 compensation, V2 pair recovery, migration recovery, execution journals,
  and dispatch journals remain distinct typed evidence families.
- Evidence inspection is read-only; completed-evidence archival is a separate
  guarded local mutation.
- Production command composition uses one explicit repository root. Cwd-based
  helpers remain compatibility facades only.
- V2 lifecycle progress is typed and presentation-neutral below its terminal
  adapter.
- V2 local delivery is unsupported; GitHub Actions is the executable V2 path.
- Workflow scaffolding is focused and create-only. It cannot update, merge, or
  overwrite a consumer-owned workflow.
- Generated-output path policy stays with each output family; there is no
  universal path manager.
- Plan, Units, default Doctor, default Pipeline, Validate, History,
  Contributors, Evidence query, and Context Validation are local, read-only,
  offline, and token-free.
- Doctor and Pipeline explicit remote verification reuse one bounded GET-only
  observation boundary and cannot mutate lifecycle state.
- Pipeline projects lifecycle evidence; it does not own transitions, dispatch,
  retry, or publication state.
- Accepted dispatch is handoff evidence and cannot be interpreted as workflow
  or publication completion.
- Exact-source validation tooling is permitted only in the dedicated Release
  Plugin self-release workflow, after an independent immutable-identity check
  and with runner-temporary CLI/plugin paths. Other workflows retain pinned
  published validation tools.

## Unsupported architecture

The following additions conflict with the maintained design unless an explicit
architecture decision replaces the relevant owner and preserves contracts:

- generic lifecycle framework or second state machine;
- stage registry or command-handler chaining;
- provider hierarchy for a single GitHub integration;
- dependency bag or service locator;
- workflow DSL or managed workflow updater;
- second renderer inside the Release Plugin;
- command-name switches or domain-status interpretation in Core;
- journal repair or schema migration hidden inside inspection;
- blind retry for uncertain push or dispatch;
- remote mutation from Doctor or Pipeline;
- lifecycle inference from remote verification facts;
- local V2 executor invocation without a proven non-overlapping publish boundary.

## Present product gaps

- Durable workflow-run and publication-completion state are absent.
- GitHub Enterprise Server is not a dispatch target.
- A public standalone dispatch/retry command is absent.
- Official build-system adapter packages are absent; consumer workflows own
  build-system mapping.

These gaps describe the product boundary and do not authorize implementation.

## Review rule

Every change remains subject to `plugin/release/RULES.md` and the
[Maintainability Policy](maintainability-policy.md). A new capability must have
one owner, explicit I/O and mutation boundaries, consumer-owned ports, focused
contract tests, and no competing interpretation of authoritative lifecycle
evidence.

Workflow scaffolding is focused, create-only, and deliberately avoids a
generic pipeline/state-machine abstraction. Its public ownership is described in
[GitHub Actions Delivery](../../../../docs/release/github-actions-delivery.md).
