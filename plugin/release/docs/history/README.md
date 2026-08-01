# Release Documentation History

## Purpose

This folder preserves completed or superseded Release planning, milestone,
hardening, evolution, and completion-review records. It is not the active
product or architecture source. Historical entries retain their original
decisions and limitations; the current sources below own supported behavior.

## Numbering policy

History filenames use a fixed-width three-digit chronological sequence. A
number is permanent once merged and represents archival sequence, not a product
version. Existing entries are never renumbered to insert an older discovery.
The next new entry uses the next unused sequence, and gaps are not reused
casually. Each file owns one logical roadmap or milestone series, keeps a
descriptive filename subject, and is not renamed when its status changes.
Merged records retain provenance notes.

A previously undiscovered older record is appended with the next sequence. Its
actual historical dates and chronological position are then recorded here;
existing entries are not renumbered.

## Chronological index

| Sequence | Entry | Historical period | Status | Outcome | Current replacement |
| -------: | ----- | ----------------- | ------ | ------- | ------------------- |
| 001 | [Behavior-Preserving Refactor](001-behavior-preserving-refactor.md) | 2026-07-14 to 2026-07-15 | completed | Nine-stage refactor completed; later limitations transferred to the review and decision record. | [Current architecture](../architecture/current-state.md) |
| 002 | [Post-Refactor Architecture Review](002-post-refactor-architecture-review.md) | 2026-07-15 to 2026-07-21 | completed | Completion review and later code-quality consolidation verified package ownership, lifecycle cohesion, compatibility, and restoration boundaries. | [Package ownership](../architecture/package-ownership.md) |
| 003 | [Post-Refactor Architecture Evolution](003-post-refactor-architecture-evolution.md) | 2026-07-15 to 2026-08-01 | superseded | Hardening, compatibility, developer-experience, and inspection decisions were completed, rejected, or transferred to current decision owners. | [Architecture decisions](../architecture/architecture-decisions.md) |

## Evolution summary

The first record captures the behavior-preserving extraction that made command,
application, persistence, Git, dispatch, migration, and V1 compatibility
boundaries explicit. The second records the architecture audit and subsequent
code-quality consolidation. The third preserves the follow-on safety and
recovery hardening, compatibility decisions, explicit-root and output work,
product inspection capabilities, and the rejection of unsafe V2 local delivery.
Ongoing product planning and unresolved architecture decisions now live only in
the current sources.

## Inventory decisions

| Original file | Current purpose | Historical content | Current content | First known date | Last active date | Decision |
| --- | --- | --- | --- | --- | --- | --- |
| `plugin/release/docs/architecture/refactor-plan.md` | Former execution ledger | Complete behavior-preserving refactor plan | None | 2026-07-14 | 2026-07-15 | merge-duplicate |
| `plugin/release/docs/architecture/refactor-history.md` | Condensed closed ledger | Completed refactor boundaries and limitations | None | 2026-07-17 | 2026-07-21 | move |
| `plugin/release/docs/architecture/post-refactor-review.md` | Mixed completion review and current package map | Refactor audit, metrics, transfer, and completion findings | Package ownership and active boundaries | 2026-07-15 | 2026-07-22 | split |
| `plugin/release/docs/architecture/post-refactor-roadmap.md` | Former follow-on roadmap | Safety, compatibility, developer-experience, and feature sequence | None | 2026-07-15 | 2026-07-17 | merge-duplicate |
| `plugin/release/docs/architecture/architecture-evolution.md` | Rolling decision record | Completed and rejected follow-on decisions | One unresolved architecture boundary | 2026-07-17 | 2026-07-31 | split |
| `plugin/release/docs/architecture/current-state.md` | Detailed current architecture | Closed-refactor context only | Runtime, data, I/O, lifecycle, and compatibility facts | 2026-07-14 | Current | retain-current |
| `plugin/release/docs/architecture/maintainability-policy.md` | Active changed-code policy and debt controls | Refactor context only | Cohesion, dependency, wrapper, and test policy | 2026-07-21 | Current | retain-current |
| `plugin/release/docs/architecture/compatibility-notes.md` | Active compatibility inventory | Retired-path context only | Preserved V1/V2, wire, and isolation contracts | 2026-07-21 | Current | retain-current |
| `plugin/release/docs/architecture/v1-compatibility-policy.md` | Active support/deprecation register | Completed removal evidence | Retained surfaces and future removal gates | 2026-07-16 | Current | retain-current |
| `docs/release/bootstrap-product-boundary.md` | Active product boundary | None | Current ownership plus explicitly labeled future capabilities | 2026-07-18 | Current | retain-current |
| `docs/release/compatibility.md` | Active user compatibility reference | None | Supported behavior and genuinely unavailable capabilities | 2026-07-07 | Current | retain-current |
| `docs/release/github-actions-golden-path.md` | Active operational guide | None | Current integration path with separately labeled future capabilities | 2026-07-18 | Current | exclude-nonhistorical |
| `docs/plugins/plugin-deploy.md` | Deploy-plugin guide | None related to Release history | Deploy planning and optional Kubernetes support | 2026-02-04 | Current | exclude-nonhistorical |
| `docs/plugins/plugin-monitoring.md` | Monitoring design guide | None related to Release history | Monitoring planes and their own next steps | 2026-02-04 | Current | exclude-nonhistorical |

The deleted full plan and roadmap were replaced in Git by their condensed
successors. Their unique decisions remain represented in entries 001 and 003;
their former paths and dates are retained as provenance rather than restored as
duplicate active documents.

## Current sources

- [Current Release architecture](../architecture/current-state.md)
- [Current package ownership and lifecycle boundaries](../architecture/package-ownership.md)
- [Current architecture decisions](../architecture/architecture-decisions.md)
- [Canonical Release command reference](../../../../docs/release/cli-reference.md)
- [Release compatibility reference](../../../../docs/release/compatibility.md)
- [V1 compatibility policy](../architecture/v1-compatibility-policy.md)
- [V1-to-V2 migration guidance](../../../../docs/release/migration-v1-to-v2.md)
- [Current Release product overview](../../../../docs/release/overview.md)

## Adding a new entry

Choose the next unused three-digit sequence without changing existing numbers.
Create one descriptively named Markdown file for one logical series. Include
`Sequence`, `Title`, `Status`, `Created`, `Completed or superseded`,
`Predecessor`, `Successor`, `Current references`, and `Original source`
metadata. Use exactly one status from `proposed`, `active`, `completed`,
`superseded`, or `abandoned`. Use ISO-style dates when exact and `Unknown` or an
explicit approximate value when Git cannot establish a date. Add exactly one
chronological-index row, record merged provenance, and verify every relative
link before merging.
