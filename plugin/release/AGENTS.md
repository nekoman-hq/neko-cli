# Release Plugin Instructions

These instructions apply to every file under `plugin/release`.

Before modifying Release Plugin files:

- Read `plugin/release/RULES.md` completely.
- Consult `plugin/release/docs/architecture/current-state.md` for the affected flow.
- Consult `plugin/release/docs/architecture/refactor-plan.md` for planned boundary work.
- Inspect the current implementation, tests, and `git status --short --branch`; current code is authoritative.

Preserve release safety and behavior before improving structure. Characterize risky behavior before refactoring version/tag selection, exact commit contents, side-effect ordering, journals, dispatch, retry/resume, error codes, responses, or secret handling.

Keep handlers at the command boundary: parse and validate input, build a typed request, invoke one use case, and map its typed result. Keep Git, filesystem, config/state, materialization, journal, token, clock, network, and response concerns behind explicit replaceable boundaries. Make unsafe operation order visible and preserve V2 commit-before-tag push ordering.

Do not silently change stable error codes, response schemas/item ordering, journal schemas/identities/transitions/pending actions, release commit contents/message, tag semantics, dry-run guarantees, recovery restrictions, or V1 compatibility. Never expose tokens or secrets in logs, errors, responses, metadata, journals, or tests.

Do not weaken, delete, skip, or broaden tests merely to make a change pass. Run focused tests while working and final repository validation described in `RULES.md`. Update architecture, rules, command, and user documentation whenever their contracts change.

When a requested milestone is complete and validated, create atomic Conventional Commit-style commits with the `release` scope automatically. Never stage or commit unrelated work. Do not amend, squash, rebase, reset, push, tag, release, dispatch, publish, or modify remote state unless the user explicitly requests that exact operation.
