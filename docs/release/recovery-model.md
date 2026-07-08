# Release Recovery Model

The recovery assessor classifies local evidence from `ReleaseExecutionJournal`. `resume --dry-run` uses it without mutation. Non-dry-run `resume` continues only when the journal state is exact and unambiguous; it does not blindly retry uncertain push or dispatch outcomes.

## Inputs

The assessor inspects:

```text
ReleaseExecutionJournal
current repository HEAD
local unit tag
known release files
V2 state file metadata
index state where locally knowable
local push markers recorded in the journal
```

Remote success is not claimed unless it is already recorded locally in the journal. The assessor does not fetch or query remotes.

## Statuses

```text
not-started
interrupted-before-commit
interrupted-after-commit
interrupted-after-tag
interrupted-before-push
interrupted-after-commit-push
interrupted-after-tag-push
ready-for-dispatch
already-handed-off
conflicted
corrupted
```

`not-started` means the journal is prepared and no mutation is confirmed.

`interrupted-before-commit` means release-file mutations may exist, but no release commit boundary is confirmed.

`interrupted-after-commit` means a local release commit is recorded, but the unit tag is not confirmed.

`interrupted-after-tag` means the local tag is confirmed on the expected release commit.

`interrupted-before-push` means local staging is confirmed, but push work is not.

`interrupted-after-commit-push` means commit push was recorded locally. The tag push is not confirmed.

`interrupted-after-tag-push` means tag push was recorded locally. Dispatch handoff is not confirmed.

`ready-for-dispatch` means the execution journal reached dispatch-journal preparation and resume may inspect the dispatch journal.

`already-handed-off` means execution handoff is complete and the dispatch journal is the next source of truth.

`conflicted` means journal facts and local repository state diverge, such as file hash mismatch or a tag pointing to an unexpected commit.

`corrupted` means the journal is structurally invalid or cannot be trusted.

## Safety

The assessor prefers `conflicted` or `corrupted` over guessing. Unknown remote state is reported as local evidence only. Non-dry-run resume decides explicitly whether and how to continue from each status and blocks ambiguous push or dispatch outcomes.

`neko release resume --unit <unit>` exists for V2 GitHub Actions execution journals. No public standalone retry or dispatch command exists.
