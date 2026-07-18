# Release V2 GitHub Actions Golden Path

This is the canonical, build-system-neutral guide for configuring and running
a Neko Release V2 release through GitHub Actions. It documents commands and
contracts that exist in the current Release Plugin. Product capabilities that
do not exist yet are labeled separately.

The short version is:

```text
Neko prepares and coordinates the release.
GitHub Actions validates the dispatched context.
The consumer workflow builds and publishes.
```

## Product model

Neko owns the selected unit's version, derived tag, release commit SHA,
release-owned file materialization, state update, exact release commit, unit
tag, commit and tag pushes, workflow dispatch, Evidence, resume policy, and
recovery rules. The workflow must consume those facts; it must not calculate a
different version or create another release commit or tag.

GitHub Actions owns checkout and validation of the dispatched context. The
consumer repository owns its build, tests, artifacts, signing, publication,
credentials, package registries, deployment, and promotion. Selecting
`github-actions` delivery does not make Neko the owner of Gradle, GoReleaser,
JReleaser, release-it, or any other build system.

There is no `neko release execute` command. The executable release commands
are:

```bash
neko release patch
neko release minor
neko release major
```

V2 supports `delivery: "github-actions"` only. `delivery: "local"` is invalid
V2 configuration. V1 local executor compatibility through
`.release.neko.json` is a separate legacy path and is not a V2 alternative.

### Ownership at a glance

| Concern | Owner |
| --- | --- |
| Unit selection, current and next version, tag, and release SHA | Neko Release Plugin |
| Materialized release files, V2 state, commit, tag, push, and dispatch | Neko Release Plugin |
| Exact checkout and release-context validation | GitHub Actions integration |
| Mapping a unit and version to modules, tasks, matrices, or targets | Consumer or build-system adapter |
| Build, artifacts, signing, publication, deployment, and secrets | Consumer repository |

## Current and future capabilities

Implemented today:

- V2 `init`, `unit-add`, `validate`, and token-free `plan`;
- `patch`, `minor`, and `major`, including token-free `--dry-run`;
- GitHub Actions delivery with release commit, lightweight unit tag, commit
  push, tag push, and `workflow_dispatch`;
- the four-input dispatch contract documented below;
- execution and dispatch Evidence, `evidence`, guarded completed-Evidence
  archival, `resume`, and `resume --dry-run`;
- V1-to-V2 migration, plugin index generation, and isolated V1 compatibility.

Future capabilities, not available as commands or generated artifacts today:

- a stable CI release-context validation command;
- a workflow generator or reusable Neko-provided workflow package;
- an integration doctor, release-unit overview, or pipeline inspection;
- completed official build-system adapters, including a Gradle adapter;
- a final GitHub Actions packaging decision;
- V2 local execution and standalone dispatch or retry commands.

Do not put candidate syntax for those future capabilities into a production
workflow. The manual CI validation block in this guide is temporary integration
wiring until a stable Neko CI context command replaces it.

## Prerequisites

### Neko CLI and Release Plugin

Install the Neko CLI by one of the supported methods in the repository
[installation guide](../installation.md). The documented release install
method is:

```bash
curl -fsSL https://raw.githubusercontent.com/nekoman-hq/neko-cli/main/install.sh | bash
```

For reproducible environments, the installer supports an explicit
`NEKO_VERSION`; see the installation guide rather than copying an unverified
version from another repository.

Install or make the Release Plugin available:

```bash
neko plugin install release
```

The plugin installer also supports `--version <release-plugin-version>` when a
consumer needs to pin the plugin. Confirm that `neko release init-options`
works before configuring a repository.

### Repository and remote

Before a real release, the repository must have:

- GitHub Actions enabled on a GitHub.com repository;
- the configured workflow committed below `.github/workflows/` and present on
  the repository's default branch, because GitHub must find it to accept
  `workflow_dispatch`;
- an attached local branch with a configured upstream remote and branch;
- a supported GitHub.com HTTPS or SSH remote;
- Git credentials that can push the release commit to the upstream branch and
  push the unit tag;
- a local `GITHUB_TOKEN` with access to the repository and Actions write
  permission for Neko's dispatch request;
- complete local tag history. Refresh tags before planning when the checkout
  may be shallow or stale:

  ```bash
  git fetch --tags --prune
  ```

- a clean worktree and index at release start;
- the required executor configuration below each unit's `workingDirectory`:
  `.goreleaser.yml` or `.goreleaser.yaml`, `jreleaser.yml`, or
  `.release-it.json`;
- consumer-owned build and publication commands that can use an already chosen
  unit, version, tag, and release SHA without creating replacement Git effects.

The Neko dispatch token and the Git credential are separate responsibilities:
the former authorizes the Actions API request, while the latter authorizes Git
pushes.

### Workflow permissions and credentials

The validation-only workflow below needs only:

```yaml
permissions:
  contents: read
```

Publication may require additional job-scoped permission such as
`contents: write`, `packages: write`, or `id-token: write`. Add only what the
chosen publication system uses. Store publication tokens, signing material,
cloud credentials, and registry credentials in GitHub environments, Actions
secrets, or an OIDC trust relationship owned by the consumer. They must never
be placed in V2 config, V2 state, workflow inputs, Evidence, command lines that
are logged, or committed files.

## Initialize Release V2

The workflow file and unit working directory must exist before `init` or
`unit-add`, because repository-aware V2 validation checks them.

Create a first normal release unit:

```bash
neko release init \
  --unit service \
  --display-name "Service" \
  --version 0.1.0 \
  --executor goreleaser \
  --delivery github-actions \
  --workflow .github/workflows/release.yml \
  --tag-prefix v \
  --working-directory . \
  --paths "**"
```

Append another unit to an existing V2 pair:

```bash
neko release unit-add \
  --unit worker \
  --display-name "Worker" \
  --version 0.1.0 \
  --executor goreleaser \
  --delivery github-actions \
  --workflow .github/workflows/release.yml \
  --tag-prefix worker/v \
  --working-directory services/worker \
  --paths "services/worker/**"
```

`release init` creates a validated V2 pair with one unit:

```text
.neko/release.config.json
.neko/release.state.json
```

`release unit-add` appends one config unit and one matching state entry while
preserving existing units. Both commands validate the complete pair before
writing it.

They do not create workflows, secrets, build commands, publication logic,
provider credentials, source directories, plugin manifests, or executor
configuration. They also do not create `.release.neko.json`.

Validate and inspect the result:

```bash
neko release validate
neko release validate --show
neko release plan --change patch --unit service
```

`validate` checks the complete config/state pair even when `--unit` focuses the
display on one unit.

## Single-unit repository

For a repository with one `service` unit, the initialization command above
creates the equivalent of:

```json
{
  "schemaVersion": 2,
  "units": [
    {
      "id": "service",
      "displayName": "Service",
      "paths": ["**"],
      "workingDirectory": ".",
      "tagPrefix": "v",
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release.yml"
      }
    }
  ]
}
```

with current version state:

```json
{
  "schemaVersion": 2,
  "units": {
    "service": {
      "version": "0.1.0"
    }
  }
}
```

When V2 contains exactly one unit, unit-bound commands may omit `--unit`; Neko
selects the only unit. The workflow still receives `unit: service` because the
dispatch contract is identical for single- and multi-unit repositories.

The complete single-unit patch path is:

```bash
neko release validate --show
neko release plan --change patch
neko release patch --dry-run
neko release patch
```

The plan shows `0.1.0` as current, `0.1.1` as next, `v0.1.1` as the tag,
`goreleaser` as executor metadata, and `.github/workflows/release.yml` as the
dispatch target. The dry-run previews the existing release command contract.
The real command performs the Neko-owned Git effects and dispatches the
workflow; the workflow performs the consumer-owned build and publication.

## Multi-unit repository

The following example creates independently versioned `api` and `cli` units.
It assumes the workflow, working directories, and executor files already
exist.

```bash
neko release init \
  --unit api \
  --display-name "API" \
  --version 1.4.2 \
  --executor jreleaser \
  --delivery github-actions \
  --workflow .github/workflows/release.yml \
  --tag-prefix api/v \
  --working-directory services/api \
  --paths "services/api/**,docs/api/**"

neko release unit-add \
  --unit cli \
  --display-name "CLI" \
  --version 0.8.0 \
  --executor goreleaser \
  --delivery github-actions \
  --workflow .github/workflows/release.yml \
  --tag-prefix cli/v \
  --working-directory tools/cli \
  --paths "tools/cli/**,docs/cli/**"
```

The API unit requires `services/api/jreleaser.yml`; Neko materializes its
`project.version` and includes it with `.neko/release.state.json` in the API
release commit. The CLI unit requires `tools/cli/.goreleaser.yml` or
`tools/cli/.goreleaser.yaml`; ordinary GoReleaser units are tag/context based,
so their release commit normally contains only the selected state update.
Plugin units are a separate exception because Neko materializes the selected
plugin manifest.

Every unit-bound command must be explicit once the repository has multiple
units:

```bash
neko release validate --unit api --show
neko release plan --change patch --unit api
neko release patch --unit api --dry-run
neko release patch --unit api
```

This changes only `units.api.version`, derives `api/v1.4.3`, and dispatches the
central workflow with `unit=api`. The workflow must select only API build and
publication targets. Releasing `cli` later plans from `0.8.0`, derives
`cli/v0.8.1`, and leaves the API version unchanged.

One central workflow can route by the validated unit and map it to
consumer-owned commands or a matrix. Per-unit workflows are also valid: set
each unit's `executor.workflow` to its own direct child of
`.github/workflows/`, copy the same four-input and validation contract into
each workflow, and optionally reject every unit except the one it owns. Neko
owns unit identity, versions, tag prefixes, and dispatch routing. The consumer
owns the mapping from a validated unit to build tasks and publication targets.

## Canonical workflow inputs

Neko dispatches the configured workflow at the existing unit tag with exactly
four string inputs:

| Input | Created by | Authority and validation | Secret? |
| --- | --- | --- | --- |
| `unit` | Neko unit selection | Authoritative selected unit. It must exist exactly once in checked-out config. | No |
| `version` | Neko version plan and committed state update | Authoritative release version. It must equal checked-out state for `unit`; the workflow must not bump or recalculate it. | No |
| `tag` | Neko `tagPrefix + version` calculation | Authoritative release tag. It must equal the checked-out unit prefix plus `version` and resolve to `release_sha`. | No |
| `release_sha` | Neko's verified release commit | Authoritative 40-character commit SHA. Checked-out `HEAD` and the tag target must both equal it. | No |

The workflow receives them through `github.event.inputs` and the typed
`inputs` context. A mismatch means the dispatch and checked-out release facts
do not describe the same immutable release; validation must fail before any
build or publication step. Executor, delivery, workflow path, repository
paths, and credentials are deliberately not inputs and must be read from the
checked-out repository or consumer secret store.

## Current CI validation

The stable Neko CI release-context command is not implemented. Today, the
workflow must perform the following temporary manual checks after exact
checkout:

1. config and state use schema version 2 and have the same unit IDs;
2. the selected unit exists exactly once;
3. selected state version equals `version`;
4. configured tag prefix plus `version` equals `tag`;
5. the unit uses `github-actions` and owns the running workflow path;
6. `release_sha` has the full lowercase Git SHA shape;
7. checked-out `HEAD` equals `release_sha`;
8. the exact tag ref exists and resolves to `release_sha`.

`neko release validate` remains the canonical pre-release validation for the
complete V2 schema and pair. The CI block below checks the additional immutable
dispatch-to-checkout relationship. If a pinned Neko CLI and Release Plugin are
already provisioned in CI, running
`neko release validate --unit "$RELEASE_UNIT"` before this block provides an
additional full-schema check, but it does not replace the context checks.

### Copy-ready minimal workflow

Save this as `.github/workflows/release.yml`. Change only
`EXPECTED_WORKFLOW` and the final consumer-owned command when adopting a
different workflow filename or publication entry point.

<!-- golden-path-workflow:start -->
```yaml
name: Release selected unit

on:
  workflow_dispatch:
    inputs:
      unit:
        description: Neko Release V2 unit id
        required: true
        type: string
      version:
        description: Neko-authoritative release version
        required: true
        type: string
      tag:
        description: Neko-created unit tag
        required: true
        type: string
      release_sha:
        description: Neko-created release commit SHA
        required: true
        type: string

permissions:
  contents: read

concurrency:
  group: release-${{ inputs.unit }}-${{ inputs.tag }}
  cancel-in-progress: false

jobs:
  release:
    runs-on: ubuntu-latest
    env:
      RELEASE_UNIT: ${{ inputs.unit }}
      RELEASE_VERSION: ${{ inputs.version }}
      RELEASE_TAG: ${{ inputs.tag }}
      RELEASE_SHA: ${{ inputs.release_sha }}
      EXPECTED_WORKFLOW: .github/workflows/release.yml
    steps:
      - name: Checkout the exact release commit with tags
        uses: actions/checkout@v4
        with:
          ref: ${{ inputs.release_sha }}
          fetch-depth: 0
          fetch-tags: true
          persist-credentials: false

      # Temporary integration wiring. Replace this whole step with the stable
      # Neko CI release-context command when that command becomes available.
      - name: Validate Neko release context
        shell: bash
        run: |
          set -euo pipefail

          fail() {
            printf '::error::%s\n' "$1"
            exit 1
          }

          [[ "$RELEASE_UNIT" =~ ^[a-z][a-z0-9-]*$ ]] ||
            fail "unit is not a valid V2 unit id"
          [[ "$RELEASE_VERSION" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]] ||
            fail "version is not valid SemVer"
          [[ "$RELEASE_SHA" =~ ^[0-9a-f]{40}$ ]] ||
            fail "release_sha must be a full lowercase 40-character Git SHA"

          jq -e '
            .schemaVersion == 2 and
            (.units | type == "array") and
            (.units | length > 0)
          ' .neko/release.config.json >/dev/null ||
            fail "release.config.json is not a V2 config"
          jq -e '
            .schemaVersion == 2 and
            (.units | type == "object")
          ' .neko/release.state.json >/dev/null ||
            fail "release.state.json is not V2 state"

          config_units="$(jq -c '[.units[].id] | sort' .neko/release.config.json)"
          state_units="$(jq -c '.units | keys | sort' .neko/release.state.json)"
          [[ "$config_units" == "$state_units" ]] ||
            fail "config and state unit ids differ"

          unit_matches="$(
            jq -c --arg unit "$RELEASE_UNIT" \
              '[.units[] | select(.id == $unit)]' \
              .neko/release.config.json
          )"
          [[ "$(jq 'length' <<<"$unit_matches")" == "1" ]] ||
            fail "selected unit is missing or duplicated in release config"
          unit_json="$(jq -c '.[0]' <<<"$unit_matches")"

          state_version="$(
            jq -er --arg unit "$RELEASE_UNIT" \
              '.units[$unit].version | strings | select(length > 0)' \
              .neko/release.state.json
          )" || fail "selected unit has no state version"
          [[ "$state_version" == "$RELEASE_VERSION" ]] ||
            fail "state version does not match dispatched version"

          tag_prefix="$(
            jq -er '.tagPrefix | strings | select(length > 0)' <<<"$unit_json"
          )" || fail "selected unit has no tag prefix"
          [[ "$RELEASE_TAG" == "${tag_prefix}${RELEASE_VERSION}" ]] ||
            fail "tag does not equal configured tag prefix plus version"

          delivery="$(jq -er '.executor.delivery' <<<"$unit_json")"
          workflow="$(jq -er '.executor.workflow' <<<"$unit_json")"
          [[ "$delivery" == "github-actions" ]] ||
            fail "selected unit does not use github-actions delivery"
          [[ "$workflow" == "$EXPECTED_WORKFLOW" ]] ||
            fail "selected unit is routed to a different workflow"

          head_sha="$(git rev-parse --verify HEAD)"
          [[ "$head_sha" == "$RELEASE_SHA" ]] ||
            fail "checked-out HEAD does not match release_sha"

          tag_ref="refs/tags/${RELEASE_TAG}"
          git show-ref --verify --quiet "$tag_ref" ||
            fail "release tag is missing from the checkout"
          tag_sha="$(
            git rev-parse --verify --quiet --end-of-options "${tag_ref}^{commit}"
          )" || fail "release tag does not resolve to a commit"
          [[ "$tag_sha" == "$RELEASE_SHA" ]] ||
            fail "release tag does not resolve to release_sha"

      # Consumer-owned extension point. Replace this command with a script that
      # builds and publishes only RELEASE_UNIT at RELEASE_VERSION. Pass secrets
      # in this step's env and add only its required job permissions.
      - name: Build and publish selected unit
        shell: bash
        run: |
          set -euo pipefail
          ./tooling/publish-release \
            --unit "$RELEASE_UNIT" \
            --version "$RELEASE_VERSION" \
            --tag "$RELEASE_TAG" \
            --release-sha "$RELEASE_SHA"
```
<!-- golden-path-workflow:end -->

`EXPECTED_WORKFLOW` is the committed path configured for the selected unit; it
is not a secret. `./tooling/publish-release` is the only consumer-specific
placeholder. Replace it with a checked-in command that accepts the validated
context and does not bump, commit, tag, push, or dispatch. The workflow assumes
`git` and `jq`, both available on GitHub-hosted Ubuntu runners; self-hosted
runners must provide them.

## Permissions and secrets

### Before dispatch

The machine running `neko release patch`, `minor`, or `major` needs:

- Git authorization to push the current commit to its configured upstream;
- Git authorization to push the exact unit tag;
- `GITHUB_TOKEN` in the local environment with repository access and Actions
  write permission so Neko can dispatch the configured workflow.

Neko resolves the dispatch token before mutation and does not store or render
it. Planning and dry-run do not resolve it.

### Inside the workflow

`contents: read` is the generic minimum for checkout and validation.
`contents: write` is not needed merely because Neko dispatched a release. Add
it only when consumer publication writes repository content, for example when
creating or updating a GitHub Release. GitHub Packages commonly needs
`packages: write`; reading private dependencies may need `packages: read`.
Cloud publication can use `id-token: write` with a narrowly scoped OIDC trust
policy instead of a long-lived token.

GitHub's `secrets.GITHUB_TOKEN` may be sufficient for GitHub Releases or
Packages when repository policy grants the needed workflow permission. Other
registries may require consumer-owned tokens. Signing keys, Maven or npm
credentials, container registry tokens, and deployment credentials remain
consumer-controlled because Neko neither understands their scope nor owns the
external publication result.

## Release plan and dry-run

Use the dedicated plan command for stable, read-only local planning facts:

```bash
neko release plan --change patch --unit service
neko release plan --change minor --unit service
neko release plan --change major --unit service
```

Use release-command dry-run for the compatibility preview of the command that
would execute:

```bash
neko release patch --unit service --dry-run
neko release minor --unit service --dry-run
neko release major --unit service --dry-run
```

The distinction is:

```text
release plan
  -> read-only local planning facts

release --dry-run
  -> compatibility preview for patch, minor, or major
```

Both are token-free and write-free. `plan` does not inspect remotes, journals,
Evidence, or publication readiness. Dry-run does not fetch, write config or
state, create Evidence, start executors, commit, tag, push, dispatch, publish,
or roll back.

## First real release

Use this checklist for a unit named `service`; omit `--unit` only when V2 has
exactly one unit.

1. Validate the complete V2 pair.

   ```bash
   neko release validate --unit service --show
   ```

2. Inspect the local plan.

   ```bash
   neko release plan --change patch --unit service
   ```

3. Verify that the unit's workflow path matches a committed default-branch
   workflow with the exact four inputs and validation block.
4. Verify local Git push credentials, the dispatch `GITHUB_TOKEN`, and
   consumer publication credentials without printing them.
5. Refresh complete tag history, confirm the attached upstream branch, and
   confirm the worktree and index are clean.

   ```bash
   git fetch --tags --prune
   git status --short --branch
   ```

6. Run the exact release command as a dry-run.

   ```bash
   neko release patch --unit service --dry-run
   ```

7. Execute one version change.

   ```bash
   neko release patch --unit service
   # or: neko release minor --unit service
   # or: neko release major --unit service
   ```

8. Observe the dispatched Actions run. Dispatch acceptance means only that
   GitHub accepted the request; wait for consumer publication to finish.
9. If the command was interrupted or the result is unclear, inspect Evidence
   before attempting another action.

   ```bash
   neko release evidence --unit service
   neko release resume --unit service --dry-run
   ```

## Evidence and recovery

Execution Evidence records the Neko-owned materialization, state, staging,
commit, tag, push, and handoff phases. Dispatch Evidence records one immutable
GitHub workflow-dispatch attempt and distinguishes `prepared`,
`request-started`, `accepted`, `rejected`, and `unknown`. The records live
under the Git common directory, outside the worktree and release commit.

Inspect redacted summaries without tokens or mutation:

```bash
neko release evidence
neko release evidence --family release-execution --unit service
neko release evidence --family dispatch --unit service
neko release resume --unit service --dry-run
```

`safe_to_resume=true` means the existing Evidence is recognized by the current
resume policy as eligible for its typed continuation after local checks. It
does not mean publication succeeded, remote state was inferred, or arbitrary
retry is safe. `automatic_continuation=false` or `manual_recovery=true` means
an operator must inspect the reported Evidence, local Git state, and relevant
GitHub run before doing anything else.

Use non-dry-run resume only for one exact unresolved V2 GitHub Actions intent:

```bash
neko release resume --unit service
```

Resume can continue supported states after a confirmed release commit. It does
not calculate a new version, create a new release identity, infer ambiguous
push completion, or redispatch `request-started`, `rejected`, or `unknown`
dispatch Evidence. Manual recovery means Neko has failed closed because the
persisted facts cannot prove a safe automatic continuation.

Do not edit or delete Evidence manually. Completed release-execution,
V1-compensation, or V2-pair-recovery Evidence can be archived only through the
guarded digest-and-confirmation flow:

```bash
neko release evidence --family release-execution --output json
neko release evidence-archive \
  --family release-execution \
  --identity <identity-from-evidence> \
  --digest-sha256 <digest-from-evidence> \
  --confirm-archive
```

Dispatch and migration Evidence are inspect-only. Archival writes and verifies
a private exact copy before removing an eligible completed source record.

An accepted dispatch is a successful Neko-to-GitHub handoff, not proof that
the workflow built or published anything. Publication completion belongs to
the consumer workflow and its target system.

## Failure scenarios

| Failure | Boundary and owner | Safe next diagnostic | Resume or manual recovery |
| --- | --- | --- | --- |
| Invalid config or state | Local; Release Plugin reports the consumer-owned file error | `neko release validate --show` | Fix the complete pair. Resume only when an existing journal and guidance explicitly allow it. |
| Unit not found or omitted in multi-unit V2 | Local; command selection | `neko release validate --show` and `neko release plan --change patch --unit <unit>` | Correct `--unit`; no resume for a release that never started. |
| Dirty worktree or index | Local Git preflight; consumer | `git status --short --branch` | Clean or commit intended changes, then start again. Preflight creates no release commit. |
| Target tag already exists or tag facts mismatch | Local preflight or CI validation; Neko/consumer boundary | `neko release plan --change patch --unit <unit>` and `git show-ref --verify refs/tags/<tag>` | Do not move or delete a release tag blindly. Follow Evidence guidance if execution started. |
| Checked-out SHA differs | Remote workflow validation; workflow owner | Inspect checkout step and `git rev-parse HEAD` in the failed run | Stop publication. Neko resume is not a workflow repair tool; fix the immutable-context problem and recover manually. |
| Configured workflow file is missing | Local repository validation; consumer | `neko release validate --unit <unit> --show` | Add the exact file below `.github/workflows/` and to the default branch, then retry only if no release started. |
| Workflow omits a required input | Remote dispatch rejection, commonly before a run; consumer | `neko release evidence --family dispatch --unit <unit>` and inspect the default-branch workflow | Rejected dispatch is terminal for automatic resume. Fix the workflow and perform manual recovery; there is no standalone retry command. |
| Dispatch rejected | Remote API result recorded locally; token/workflow/repository owner | `neko release evidence --family dispatch --unit <unit>` | No automatic redispatch. Correct authorization or workflow contract and follow manual recovery guidance. |
| Dispatch result uncertain | Remote boundary; network or GitHub outcome is unknown | `neko release evidence --family dispatch --unit <unit>` plus GitHub Actions inspection | Never retry blindly. `request-started` and `unknown` require manual recovery. |
| Release command interrupted | Local Neko process with durable Evidence | `neko release evidence --unit <unit>` then `neko release resume --unit <unit> --dry-run` | Run resume only when assessment says it is safe. Pending push or pre-commit ambiguity can require manual recovery. |
| Workflow build or publication fails | Remote consumer workflow or publication system | Actions logs and publication-system audit, then `neko release evidence --unit <unit>` for handoff facts | Dispatch may already be accepted and Neko complete. Repair or rerun consumer publication only under the consumer's idempotency policy; `neko release resume` does not prove publication. |
| Evidence is malformed, unsupported, or conflicting | Local evidence boundary; operator and Release Plugin | `neko release evidence` | Neko fails closed. Preserve the file and perform manual recovery; do not delete, edit, or archive it by force. |

## Consumer publication examples

These snippets replace the final consumer-owned step in the minimal workflow.
They assume the context-validation step has already succeeded.

### Generic custom command

```yaml
- name: Build and publish selected unit
  shell: bash
  env:
    PUBLISH_TOKEN: ${{ secrets.PUBLISH_TOKEN }}
  run: |
    set -euo pipefail
    ./tooling/publish-release \
      --unit "$RELEASE_UNIT" \
      --version "$RELEASE_VERSION" \
      --tag "$RELEASE_TAG" \
      --release-sha "$RELEASE_SHA"
```

The script is consumer-owned. It should select only the intended unit and use
the supplied version rather than reading the newest tag or calculating a bump.

### GoReleaser

For a tag shape and GoReleaser configuration that support direct publication:

```yaml
- name: Publish with GoReleaser
  uses: goreleaser/goreleaser-action@v6
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    RELEASE_VERSION: ${{ inputs.version }}
  with:
    distribution: goreleaser
    version: "~> v2"
    args: release --clean --config .goreleaser.yaml
```

This step needs consumer-selected `contents: write`. It runs after Neko has
created and pushed the tag; it must not add another version bump, commit, tag,
or push. For prefixed tags unsupported by a chosen GoReleaser release mode,
consumer workflows can package with a dedicated GoReleaser config and create
the GitHub Release for the exact validated tag with `gh release create`, as
Neko's plugin workflows do.

### JReleaser

JReleaser runs inside GitHub Actions as publication logic. Neko materializes
the selected unit's checked-in `jreleaser.yml` version before its release
commit; the workflow supplies the same validated version and tells JReleaser
not to create a replacement tag:

```yaml
- name: Publish release metadata with JReleaser
  uses: jreleaser/release-action@2.5.0
  env:
    JRELEASER_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    JRELEASER_PROJECT_VERSION: ${{ inputs.version }}
  with:
    version: 1.20.0
    setup-java: "false"
    working-directory: ${{ github.workspace }}/services/api
    arguments: >
      release
      --config-file jreleaser.yml
      --tag-name ${{ inputs.tag }}
      --skip-tag
      --git-root-search
```

Pin action and tool versions according to the consumer's dependency policy.
The job commonly needs `contents: write` if JReleaser creates or updates a
GitHub Release.

### release-it

The current Release Plugin recognizes `release-it` executor metadata for V2
GitHub Actions delivery, but the checked-in executor model does not define a
safe, supported publish-only release-it recipe. Its existing adapter can own
version files, commit, tag, push, and publication. Therefore this guide does
not present a copy-ready release-it command that might duplicate Neko's Git
effects. A consumer may integrate release-it only after it independently
proves and tests a mode that consumes the validated context while disabling
all replacement version, commit, tag, push, and dispatch behavior.

### Gradle

Current Gradle integration is consumer-specific. A workflow can map the
validated unit and version into checked-in tasks:

```yaml
- name: Publish selected Gradle targets
  shell: bash
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    set -euo pipefail
    ./gradlew publishSelectedRelease \
      -Pneko.release.unit="$RELEASE_UNIT" \
      -Pneko.release.version="$RELEASE_VERSION" \
      -Pneko.release.tag="$RELEASE_TAG" \
      -Pneko.release.sha="$RELEASE_SHA" \
      --no-daemon
```

`publishSelectedRelease` and those project properties are placeholders owned
by the consumer; they are not official Neko tasks. Existing consumers use this
pattern to verify version alignment, discover unit-scoped targets, and publish
selected packages or images. An official Gradle adapter remains future work
and must not be assumed to exist.

## Verification CI versus release CI

Normal push and pull-request verification answers whether repository changes
build, test, lint, and satisfy general policy. It may deliberately cover the
whole repository and normally checks out the branch or pull-request commit. It
does not need release Evidence.

Release CI is a separate `workflow_dispatch` path. It must:

- checkout the exact Neko-created release SHA with complete tag history;
- validate the selected unit, authoritative version, tag, and SHA;
- select only that unit's intended build and publication targets;
- publish only those targets;
- avoid creating replacement release Git effects.

Do not make ordinary verification depend on execution or dispatch Evidence.
Evidence belongs to Neko's release and recovery boundary, not to whether a
normal source change passes CI.

## Migrating an existing V2 GitHub Actions integration

Keep migration history outside the current workflow. Compare the existing
workflow against this checklist:

- `workflow_dispatch` declares exactly `unit`, `version`, `tag`, and
  `release_sha` as required string inputs;
- checkout uses the exact release SHA, full history, and tags;
- the temporary validation block checks unit existence, config/state unit
  alignment, state version, configured tag prefix, full SHA, `HEAD`, exact tag
  target, delivery, and workflow route;
- generic validation starts with `contents: read` and publication permissions
  are added only to the jobs that need them;
- concurrency includes both unit and tag and does not cancel a release in
  progress;
- build and publish consume the validated context and select one unit;
- the workflow does not bump, commit, tag, push release Git effects, or
  dispatch another workflow;
- every V2 unit uses `delivery: "github-actions"`; one legacy `local` unit
  invalidates the complete V2 pair even when another unit is selected;
- `.release.neko.json` and V1 executor behavior remain isolated from V2 and
  are not read by the release workflow.

For a root V1 repository, use the separate
[V1-to-V2 migration guide](migration-v1-to-v2.md), create the referenced
workflow before a real V2 release, and validate the resulting pair. Do not
copy V1 local executor steps into the V2 workflow.

## Related documentation

- [Release overview](overview.md)
- [Release CLI reference](cli-reference.md)
- [Release configuration](configuration.md)
- [GitHub Actions delivery](github-actions-delivery.md)
- [Dispatch contract](dispatch-contract.md)
- [Execution journal](execution-journal.md)
- [Recovery model](recovery-model.md)
- [Release V2 examples](examples.md)
