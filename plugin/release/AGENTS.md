# Release Plugin Instructions

These instructions apply to every file under `plugin/release`.

Before modifying Release Plugin files:

- Read `plugin/release/RULES.md` completely.
- Consult `plugin/release/docs/architecture/current-state.md` for the affected flow.
- Consult `plugin/release/docs/architecture/refactor-history.md` for completed boundary context and `plugin/release/docs/architecture/architecture-evolution.md` for pending architecture decisions.
- Inspect the current implementation, tests, and `git status --short --branch`; current code is authoritative.

Release correctness and long-term maintainability are joint acceptance criteria. Characterize risky behavior before refactoring version/tag selection, exact commit contents, side-effect ordering, journals, dispatch, retry/resume, error codes, responses, or secret handling.

Every changed function must have one clearly nameable responsibility, one primary reason to change, and one abstraction level. Do not create or preserve god functions, procedural release playbooks, deeply nested control flow, hidden or generic state machines, large phase/status workflow switches, boolean workflow parameters, vague service/manager/coordinator dumping grounds, or generic step pipelines.

Keep handlers limited to parsing and validating input, building a typed request, invoking one focused use case, and mapping its typed result. Use focused named operations, explicit narrowly scoped dependencies, and small consumer-owned interfaces where a real substitution seam exists. Keep Git, filesystem, config/state, materialization, journals, token, clock, network, executors, and response concerns out of business logic. Make unsafe operation order visible through readable application calls, not callbacks or generic step lists.

Do not silently change stable error codes, response schemas/item ordering, journal schemas/identities/transitions/pending actions, release commit contents/message, tag semantics, dry-run guarantees, recovery restrictions, or V1 compatibility. Never expose tokens or secrets in logs, errors, responses, metadata, journals, or tests.

Do not weaken, delete, skip, or broaden tests merely to make a change pass. Reject an implementation that passes tests but violates `RULES.md`; extraction alone is not success when responsibilities, dependencies, abstraction levels, or control flow remain mixed. Run focused tests while working and final repository validation described in `RULES.md`. Update architecture, rules, command, and user documentation whenever boundaries or contracts change.

When a requested change is complete and validated, create atomic Conventional Commit-style commits with the `release` scope automatically. Never stage or commit unrelated work. Do not amend, squash, rebase, reset, push, tag, release, dispatch, publish, or modify remote state unless the user explicitly requests that exact operation.
