# GitHub Actions Delivery

`github-actions` is a valid V2 delivery mode for repository configuration. Public V2 non-dry-run releases now use execution and dispatch journals and dispatch the configured workflow after pushing the release commit and unit tag.

## Configuration

Each GitHub Actions unit must configure a workflow:

```json
{
  "id": "api",
  "paths": ["api/**"],
  "workingDirectory": "api",
  "tagPrefix": "api/v",
  "executor": {
    "type": "jreleaser",
    "delivery": "github-actions",
    "workflow": ".github/workflows/release-api.yml"
  }
}
```

The workflow path is repository-root-relative and must use this canonical form:

```text
.github/workflows/<workflow-file>.yml
.github/workflows/<workflow-file>.yaml
```

Filename-only values such as `release-api.yml` are invalid. Nested workflow paths and arbitrary repository-relative paths are invalid.

## Validation

Structural validation requires:

- `workflow` is mandatory for `delivery: github-actions`.
- `delivery: local` is rejected for V2 releases before workflow validation can make it executable.
- The path uses `/`, not `\`.
- The path begins exactly with `.github/workflows/`.
- The path points directly to one file below `.github/workflows/`.
- The filename matches `[A-Za-z0-9][A-Za-z0-9._-]*.(yml|yaml)`.
- The extension is lowercase `.yml` or `.yaml`.
- No absolute paths, `..`, `./`, duplicate separators, trailing slash, query strings, fragments, URLs, `@` refs, or shell-like expansion syntax are allowed.

Repository-aware validation additionally confirms that the configured workflow exists, is a regular file, resolves inside the repository root, and resolves inside `.github/workflows/`. Symlinks must not escape either boundary.

Valid examples:

```json
{"type":"jreleaser","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}
{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-web.yaml"}
{"type":"jreleaser","delivery":"github-actions","workflow":".github/workflows/api-v2-release.yml"}
```

Invalid examples:

```json
{"type":"jreleaser","delivery":"github-actions"}
{"type":"jreleaser","delivery":"github-actions","workflow":"release-api.yml"}
{"type":"jreleaser","delivery":"github-actions","workflow":".github/workflows/nested/release.yml"}
{"type":"jreleaser","delivery":"github-actions","workflow":".github/workflows/release.YML"}
{"type":"goreleaser","delivery":"local","workflow":".github/workflows/release.yml"}
```

## Dispatch Boundary

V2 dry-run output shows the configured workflow, dispatch ref, canonical input names, and pending journal identity. V2 GitHub Actions non-dry-run writes journals, materializes versions, updates V2 state, commits, tags, pushes commit and tag, and dispatches the workflow. V2 local delivery is unsupported; configs using it are invalid.

The configured workflow must support `workflow_dispatch` and accept only:

```text
unit
version
tag
release_sha
```

The workflow must derive executor and delivery from checked-out repository config rather than trusting dispatch inputs.

After checkout with complete tag history, validate this contract with:

```bash
neko release ci-validate-context \
  --unit "$RELEASE_UNIT" \
  --version "$RELEASE_VERSION" \
  --tag "$RELEASE_TAG" \
  --release-sha "$RELEASE_SHA" \
  --output github \
  --github-output-file "$GITHUB_OUTPUT"
```

The command validates the local V2 pair, authoritative unit/version/tag,
release commit, checked-out HEAD, and peeled tag target. It accepts matching
detached HEAD, never fetches missing tags, reads no token, contacts no remote,
and performs no mutation. Its stable outputs are `unit`, `display_name`,
`version`, `tag_prefix`, `tag`, `release_sha`, `working_directory`, `executor`,
`delivery`, and `workflow`; downstream publication should consume these
validated step outputs. See the [golden path](github-actions-golden-path.md) for
the canonical workflow.

## Workflow scaffolding

`neko release github-workflow-init` is the opt-in Release V2 integration
scaffolder. It requires an existing structurally valid V2 config/state pair,
but the selected workflow target itself may be missing. `init` and `unit-add`
remain config/state commands and do not invoke the scaffolder.

With no selector, one unique configured workflow path is required. Units that
share it use the same generated workflow. If units configure multiple distinct
paths, select exactly one with `--unit <unit-id>` or `--path <configured-path>`.
An explicit path must match at least one configured unit, and when combined
with `--unit` it must match that unit. The command never guesses or writes
multiple workflows.

```bash
neko release github-workflow-init --dry-run
neko release github-workflow-init --unit api --dry-run
neko release github-workflow-init --path .github/workflows/release-api.yml
```

The generator is create-only. Missing targets are atomically created with mode
`0644`; byte-identical targets are reported current and are not rewritten;
different targets fail closed. Preview is read-only and includes the exact
canonical YAML even for a conflict. There is no force, update, provider, or
consumer-command flag.

Contract version `1` starts with:

```text
# Generated by Neko Release workflow scaffolding.
# Workflow contract version: 1
# Create-only scaffold: customize consumer-owned steps after generation.
```

The generated workflow uses only `workflow_dispatch`, the exact inputs
`unit`, `version`, `tag`, and `release_sha`, `contents: read`, non-cancelling
unit-and-tag concurrency, exact-SHA checkout with full history and tags, pinned
CLI/plugin installation, `ci-validate-context`, and a deliberately failing
consumer-owned build/publication step. It generates no GitHub Release, build
system, secrets, publication permissions, repository settings, commit, tag,
push, or dispatch.

Generation is rooted at the resolved repository, allows only a canonical
`.github/workflows/*.yml|yaml` path already present in V2 config, rejects
absolute/traversal/protected/nested/unsupported targets and symlink escapes,
and does not depend on process cwd. It requires no token or network access and
performs no Git mutation. Manually maintained workflows remain fully supported;
different content is preserved as a conflict.

The dispatch `ref` is the existing unit tag. GitHub also requires the workflow file to exist on the repository default branch. Real internal dispatch uses `GITHUB_TOKEN` with repository Actions write permission and targets GitHub.com remotes only.

Workflow files are generated only by the explicit create-only scaffolding command; existing release commands never rewrite them. No public standalone dispatch or retry command exists. `neko release resume --unit <unit>` resumes only existing unresolved execution journals. Follow the [Release V2 GitHub Actions Golden Path](github-actions-golden-path.md) for a complete consumer setup. See also [Release V2 bootstrap product boundary](bootstrap-product-boundary.md), [GitHub Actions release flow](github-actions-release-flow.md), [Execution journal](execution-journal.md), [Recovery model](recovery-model.md), [GitHub Actions dispatch](github-actions-dispatch.md), [Dispatch contract](dispatch-contract.md), and [Dispatch journal](dispatch-journal.md).

## Integration doctor

`neko release doctor [--unit <unit-id>]` is the read-only local readiness
check for this delivery contract. It loads the strict V2 config/state pair,
deduplicates configured workflow paths, parses workflow YAML structurally, and
reports ordered unit, workflow, and diagnostic facts. Selecting one unit keeps
all units sharing its workflow in the workflow scope.

The workflow checks cover `workflow_dispatch`, the exact four required string
inputs, competing release-capable triggers, explicit and least-privilege
permissions, non-cancelling unit/tag concurrency, exact release-SHA checkout,
full history and tags, disabled persisted checkout credentials, pinned Neko
CLI and Release plugin installation before validation, every canonical
validator flag, GitHub output-file wiring, stable validator output identity,
and replacement of the generated consumer placeholder. Optional additional
dispatch inputs are allowed; additional required inputs are not, because Neko
cannot supply them. Unrelated pull-request or branch verification triggers are
not treated as publication conflicts merely by existing.

Permissions are evaluated at their actual workflow/job scope. Omitted job
declarations inherit workflow permissions; explicit job permissions replace
that default. A workflow-level write or any `write-all` remains
`PERMISSIONS_BROAD`.
Job-scoped `contents: write` is accepted only when that job contains a direct
GoReleaser publication or `gh release create`/`gh release upload`; job-scoped
`packages: write` additionally supports an explicit GitHub Container Registry
push. Unsupported shapes, values, unrelated write scopes, OIDC writes, and
write grants supported only by names, paths, secrets, or non-mutating commands
remain warnings. This is local structural evidence, not remote publication
proof.

Readiness is `not_ready` with exit code `1` when any error exists,
`ready_with_warnings` with exit code `0` when only warnings remain, and `ready`
with exit code `0` when findings are recommendations or locally not verifiable.
JSON exposes stable `readiness`, `summary`, `units`, `workflows`, and
`diagnostics` fields. Each diagnostic contains severity, scope, optional unit
and workflow identity, stable code, message, and remediation.

The doctor is token-free, network-free, Git-command-free, and mutation-free.
It cannot prove the remote default-branch workflow, repository-variable values,
remote install artifacts, publication credentials, workflow-dispatch
authorization, custom consumer build correctness, or publication-target
version acceptance.

## Nekocli Production Workflows

Nekocli dogfoods three independent V2 GitHub Actions units:

| Unit | Tag format | Workflow | GoReleaser config | Publishes |
| --- | --- | --- | --- | --- |
| `cli` | `vX.Y.Z` | `.github/workflows/release-neko-cli.yml` | `.goreleaser.cli.yaml` | main `neko` CLI assets |
| `plugin-release` | `plugin-release/vX.Y.Z` | `.github/workflows/release-plugin-release.yml` | `.goreleaser.plugin-release.yaml` | release plugin assets with `plugin/release/manifest.json` |
| `plugin-ui` | `plugin-ui/vX.Y.Z` | `.github/workflows/release-plugin-ui.yml` | `.goreleaser.plugin-ui.yaml` | UI plugin assets with `plugin/ui/manifest.json` |

Neko CLI creates the state/materialization commit, creates the unit tag, pushes
commit then tag, and dispatches the workflow with the four required string
inputs `unit`, `version`, `tag`, and `release_sha`. `workflow_dispatch` is the
only trigger, and concurrency is
`release-${{ inputs.unit }}-${{ inputs.tag }}` with cancellation disabled.

Each workflow defaults to `contents: read` and has two jobs. The `validate` job
checks out `${{ inputs.release_sha }}` with complete history and explicit tags,
does not persist checkout credentials, installs Neko from the canonical
`install.sh` URL pinned by repository variable `NEKO_VERSION`, and installs the
Release Plugin pinned by `NEKO_RELEASE_PLUGIN_VERSION`. Its one
`release-context` step invokes:

```text
neko release ci-validate-context \
  --unit "$RELEASE_UNIT" \
  --version "$RELEASE_VERSION" \
  --tag "$RELEASE_TAG" \
  --release-sha "$RELEASE_SHA" \
  --output github \
  --github-output-file "$GITHUB_OUTPUT"
```

The validated `unit`, `version`, `tag`, and `release_sha` step outputs become
same-named `validate` job outputs. Read-only tests, dedicated GoReleaser config
checking, unit-scoped snapshot builds, and plugin-manifest checks consume those
validated values. The dependent `publish` job checks out the validated job
output SHA with the same safe checkout settings and uses only
`needs.validate.outputs.*` release identity.

Only `publish` grants `contents: write`. The CLI GoReleaser publication and the
plugin `gh release create` plus registry update steps consume
`secrets.GITHUB_TOKEN`; no validation or build-check step receives the secret.
GitHub Actions cannot grant token permissions at step scope, so the separate
publication job is the narrowest boundary that preserves real GitHub Release
and registry publication. The local Doctor accepts these three scoped
publication grants: the repository result is `ready` with zero errors,
warnings, and recommendations. It retains seven honest `not_verifiable` facts
per workflow (21 total) for remote workflow state, repository variables,
installation artifacts, publication credentials, dispatch authorization,
consumer build correctness, and publication-target acceptance.

For prefixed plugin tags, the workflow does not run `goreleaser release` as the publisher because the free GoReleaser release command parses the full current tag as SemVer. Instead, GoReleaser packages archives and checksums with the dedicated plugin config, and `gh release create "$RELEASE_TAG"` creates the GitHub Release for the exact `plugin-release/vX.Y.Z` or `plugin-ui/vX.Y.Z` tag.

No workflow calculates versions, commits, tags, pushes, rewrites manifests, or
uses the global mixed-artifact `.goreleaser.yaml` for publishing. Plugin
workflows receive `PLUGIN_RELEASE_VERSION` or `PLUGIN_UI_VERSION` from the
validated version output. The CLI workflow receives `CLI_VERSION` from that
same output.
