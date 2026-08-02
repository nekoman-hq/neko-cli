# GitHub Actions Delivery

> **Audience:** Repository owners integrating Release V2 with GitHub Actions and operators validating that integration.
>
> **Purpose:** Define workflow ownership, path validation, create-only scaffolding, dispatch, credentials, and read-only readiness inspection.

GitHub Actions is the executable V2 delivery contract. Neko CLI and the Release Plugin own release policy, version/state mutation, the release commit and tag,
push order, journals, and workflow handoff. Consumer repositories own
product-specific build, test, artifact, and publication behavior.

## Configured workflow

Each V2 unit names a workflow below `.github/workflows/`:

```json
{
  "id": "api",
  "paths": ["api/**"],
  "workingDirectory": "api",
  "tagPrefix": "api/v",
  "executor": {
    "type": "goreleaser",
    "delivery": "github-actions",
    "workflow": ".github/workflows/release-api.yml"
  }
}
```

The path is repository-relative and has one of these forms:

```text
.github/workflows/<workflow-file>.yml
.github/workflows/<workflow-file>.yaml
```

Validation requires a direct child file, lowercase supported extension, `/`
separators, and a filename matching
`[A-Za-z0-9][A-Za-z0-9._-]*.(yml|yaml)`. It rejects absolute paths, traversal,
nested workflow directories, URLs, refs, fragments, queries, duplicate
separators, shell expansion, unsupported casing, and symlink escapes.
Repository-aware validation also requires a regular file inside both the
repository and `.github/workflows/`.

## Workflow ownership

The workflow is a consumer-owned implementation of a small Neko handoff
contract. It receives exactly four required string inputs:

```text
unit
version
tag
release_sha
```

It derives executor, delivery, paths, and publication configuration from the
checked-out repository. It checks out the exact release commit with complete
tag history, validates the tag and SHA, builds the selected unit, and publishes
from that immutable identity. It does not calculate versions, update state,
commit, tag, or push.

The canonical context validator invocation is:

```bash
neko release ci-validate-context \
  --unit "$RELEASE_UNIT" \
  --version "$RELEASE_VERSION" \
  --tag "$RELEASE_TAG" \
  --release-sha "$RELEASE_SHA" \
  --output github \
  --github-output-file "$GITHUB_OUTPUT"
```

`ci-validate-context` is local, token-free, network-free, and read-only. It
validates config/state, unit, version, tag, checked-out `HEAD`, peeled tag
target, and release SHA. It never fetches a missing tag. Stable outputs are
`unit`, `display_name`, `version`, `tag_prefix`, `tag`, `release_sha`,
`working_directory`, `executor`, `delivery`, and `workflow`.

## Create-only scaffolding

`neko release github-workflow-init` creates one missing starter workflow for an
existing valid V2 config/state pair:

```bash
neko release github-workflow-init --dry-run
neko release github-workflow-init --unit api --dry-run
neko release github-workflow-init --path .github/workflows/release-api.yml
```

With no selector, the configuration must contain one unique workflow path.
Units sharing a path share one generated workflow. For multiple distinct paths,
select exactly one by unit or configured path. Combined selectors must identify
the same target; the command never guesses or writes multiple workflows.

Workflow Init is create-only. A missing target is created atomically with mode
`0644`; byte-identical targets are accepted without rewriting; different
existing content is a conflict and remains untouched. Dry-run renders the exact
canonical YAML without writing. There is no force, provider, managed-update, or
consumer-command flag.

The scaffold uses contract version 1, `workflow_dispatch`, the four inputs,
`contents: read`, non-cancelling unit/tag concurrency, exact-SHA checkout with
full tags, pinned CLI and plugin installation, context validation, and a
deliberately failing consumer-owned extension point. It grants no publication
permission and creates no GitHub Release. The command requires no token or network access and performs no Git mutation.

Existing manually maintained workflows remain supported. Neko CLI does not
perform managed workflow updates and never overwrites consumer-owned content.
Use the [GitHub Actions golden path](github-actions-golden-path.md) for the
complete scaffold and customization procedure.

## Dispatch contract

Dispatch starts only after the exact release commit and unit tag exist and have
been pushed. The repository target is derived from the verified selected Git
remote. Supported targets are GitHub.com HTTPS, `ssh://git@github.com/...`, and
SCP-style `git@github.com:...` remotes. GitHub Enterprise, other providers,
local file remotes, credentials in URLs, traversal, extra path segments, and
ambiguous remote forms are rejected.

The internal client sends one request:

```text
POST /repos/{owner}/{repo}/actions/workflows/{workflow_filename}/dispatches
```

The body contains `ref` set to the unit tag and the four canonical inputs.
Executor, delivery, workflow path, repository path, config path, secrets, and
arbitrary user input are not sent. Redirects are disabled so authorization is
not forwarded to another host.

Non-dry-run dispatch resolves `GITHUB_TOKEN` before release mutation and
requires repository Actions write permission. The value is never persisted,
logged, rendered, or placed in a journal. Dry-run does not resolve the token.

Dispatch results are:

- any `2xx`: `accepted`;
- `400`, `401`, `403`, `404`, `422`, or `429`: `rejected`;
- timeout, transport interruption, cancellation after request start, redirect, `5xx`, or unexpected status: `unknown`.

`accepted` means handoff succeeded. `rejected` and `unknown` preserve the
journals and do not roll back Git. An uncertain outcome is never retried
automatically. No public standalone dispatch or retry command exists. See
[Journals and Recovery](journals-and-recovery.md).

## Integration Doctor

`neko release doctor [--unit <unit-id>]` is a read-only readiness inspection.
The default is offline, token-free, Git-command-free, and mutation-free. It
never repairs configuration, workflows, permissions, credentials, or repository
state.

Local Doctor checks include:

- strict V2 config/state and unit/workflow mapping;
- `workflow_dispatch` and the four required string inputs;
- competing release-capable triggers;
- effective workflow/job permissions;
- non-cancelling unit/tag concurrency;
- exact-SHA checkout, complete tag history, and disabled persisted credentials;
- pinned CLI and Release Plugin installation before validation, except for the
  dedicated Release Plugin self-release workflow's bounded exact-source
  validation toolchain;
- the canonical validator flags and GitHub output wiring;
- replacement of the scaffold's failing consumer placeholder;
- recognized GoReleaser, GitHub Release, artifact, manifest, plugin-index, and credential shapes.

Additional optional dispatch inputs are allowed; additional required inputs are
not. Publication permissions are accepted only at a job whose recognized
commands require the matching write scope. Names, paths, secrets, comments, or
non-mutating commands do not prove publication.

Readiness is `not_ready` with exit `1` when errors exist,
`ready_with_warnings` with exit `0` for warnings only, and `ready` with exit `0`
when only recommendations or explicit local limitations remain. The structured
contract exposes readiness, summary, units, workflows, verifications, and
diagnostics.

### Explicit remote verification

`--verify-remote` opts into bounded GitHub `GET` requests. Remote mode checks
repository/default-branch identity, exact workflow bytes and active state,
recognized version-pin variables, referenced custom-secret name metadata,
Actions policy when authorized, and exact locally derived releases, tags, and
assets. Public facts are anonymous-first; protected reads resolve one optional
read token lazily.

Doctor never requests secret values, treats built-in `GITHUB_TOKEN` as a
repository secret, lists arbitrary secret/variable collections, follows latest
release heuristics, performs fuzzy asset matching, dispatches, uploads,
publishes, or mutates local or remote state. Remote failures remain explicit
partial evidence. See
[Integration Doctor Remote Verification](integration-doctor-remote-verification.md).

`pipeline --verify-remote` reuses neutral verification facts but remains a
read-only local projection. It does not invoke the Doctor handler, consume
Doctor readiness, complete lifecycle stages, authorize Resume, or prove
publication.

## This repository's workflows

Neko CLI uses three independently configured V2 units:

| Unit | Tag namespace | Workflow | GoReleaser configuration |
| --- | --- | --- | --- |
| `cli` | `vX.Y.Z` | `.github/workflows/release-neko-cli.yml` | `.goreleaser.cli.yaml` |
| `plugin-release` | `plugin-release/vX.Y.Z` | `.github/workflows/release-plugin-release.yml` | `.goreleaser.plugin-release.yaml` |
| `plugin-ui` | `plugin-ui/vX.Y.Z` | `.github/workflows/release-plugin-ui.yml` | `.goreleaser.plugin-ui.yaml` |

Each workflow validates the immutable release identity in a read-only job.
Only the publication job grants `contents: write`. The CLI publishes through
its dedicated GoReleaser config. Plugin workflows package with their dedicated
configs, create the exact prefixed GitHub Release, then publish or replace
`plugin-index.json` on the stable `plugin-registry` release.

The root mixed-artifact `.goreleaser.yaml` is not used by these production
workflows. No workflow rewrites version state or manifests.

## Related documentation

- [Release lifecycle](lifecycle.md)
- [Configuration and state](configuration.md)
- [GitHub Actions golden path](github-actions-golden-path.md)
- [Journals and recovery](journals-and-recovery.md)
- [Release command reference](cli-reference.md)
