# Release Plugin Compatibility Notes

## Preserved contracts

The code-quality refactor preserves command names, flags, manifest/help
contracts, supported Go APIs, V1 executor behavior, V2 planning and execution,
JSON/raw JSON, diagnostics, journal schemas and transitions, workflow contracts,
materialized files, tag shapes, commit message/order, push order, handoff,
resume, retry safety, and presentation behavior.

The three repository release workflows, dedicated GoReleaser configs, and the
mixed root `.goreleaser.yaml` were not changed.

## V1 compatibility

V1 still selects one of three fixed executors: GoReleaser, JReleaser, or
release-it. Production constructs fresh executor instances explicitly. Execution,
environment mapping, compensation, Git/GitHub effects, and legacy output remain
in the existing `pkg/release/tool/*` adapters.

Reusable identity and configuration facts moved inward. The public executor
configuration helpers now delegate to the canonical internal fact packages;
their signatures, bytes, errors, and behavior remain characterized.

The mutable registry, `Service`, `Preflight`, `Tool`, `ToolBase`, legacy executor
method shapes, cwd configuration helpers, version-evidence helpers, and the
inactive V2 local transaction remain compatibility debt. Production composition
does not use the registry. The authoritative per-symbol decisions and removal
preconditions remain in [v1-compatibility-policy.md](v1-compatibility-policy.md).

The source-format V1 requirements behavior is shared through
`internal/legacyrequirements`, so `ValidateRequirementsAt` and Validate use one
owner. Active V1/V2 execution-context validation remains a separate unit-root
contract in `ValidateRequirementsForContext`; it has different dry-run token
semantics by design. Token lookup order, required files, error text, and
redaction are unchanged on both characterized paths.

## V2 compatibility

V2 continues to use GitHub Actions delivery. Active non-dry-run V2 never invokes
a release tool locally. The operation order is documented in
[current-state.md](current-state.md) and remains guarded by focused
tests. Execution, dispatch, migration, V1 compensation, and V2 pair-recovery
journals remain distinct schemas with their existing locations and modes.

Dry-run and read-only commands remain token-free and mutation-free unless the
command explicitly requests an established exception. Doctor and Pipeline
remote verification remain opt-in and reuse the same GET-only client and lazy
read-token boundary. Their defaults remain offline/token-free, and neither
remote mode dispatches, uploads, publishes, or mutates. Workflow Init remains
create-only.

## Public facades and internal exports

Root handlers for Doctor, Units, Context Validation, and Workflow Init remain
supported forwarding surfaces while their implementations live in focused
internal packages. They translate internal typed results where required but do
not own command policy.

New exported identifiers under `internal` exist only to cross an intentional
package boundary: focused handler entry points, typed facts/contracts, and the
shared V1 requirement validator. Go's `internal` rule prevents them from becoming
module-external public API.

No compatibility alias introduces a second implementation. A facade must remain
a direct delegate to its canonical owner. No command handler may call another
command handler to reuse policy.

## Wire and isolation guarantees

- Stable command success/error codes, metadata, item ordering, JSON fields, raw
  JSON, widths, TTY color, and `NO_COLOR` behavior remain characterized.
- Patch, Minor, and Major share one transport-only lifecycle presentation;
  Resume owns a separate transport-only recovery presentation. Neither mapper
  reads configuration, journals, Git, tokens, remotes, or provider state.
- Global `--describe` changes human structure only. Global `--verbose` adds
  chronological captured phases only. Both preserve lifecycle and recovery
  selection, dry-run effects, outcome rows, error envelopes, and exit behavior.
- Explicit-root handlers continue to isolate two repositories in one process;
  production routing does not change cwd.
- Tokens remain typed/redacted and are resolved only at established mutation or
  explicit remote-verification boundaries.
- Dispatch response bodies remain sanitized; no external network call is needed
  by the test suite.
- Human lifecycle output uses repository-relative paths or safe artifact labels.
  Verbose Git diagnostics do not print repository roots, absolute command
  paths, raw command output, tokens, authorization headers, or full
  configuration/journal payloads.
