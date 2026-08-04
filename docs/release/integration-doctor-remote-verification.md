# Release V2 Integration Doctor remote verification

> **Audience:** Operators who explicitly request GitHub-backed readiness evidence and contributors maintaining the GET-only client.
>
> **Purpose:** Define the opt-in remote read boundary, token handling, endpoint allowlist, evidence semantics, and remaining limitations.

## Command boundary

`neko release doctor` is offline, token-free, deterministic, and mutation-free
by default. Remote observation is opt-in:

```bash
neko release doctor --verify-remote
neko release doctor --unit cli --verify-remote
neko release doctor --output json --verify-remote
```

The explicit mode is observation only. It cannot dispatch a workflow, create or
upload a GitHub Release, publish a package, change Actions policy, mutate a
variable or secret, update a workflow, write config/state, read or write
journals/Evidence, run a shell command, or mutate Git.

## Pipeline reuse

`neko release pipeline` consumes the same neutral verification facts without
invoking the Doctor command handler, diagnostics/readiness policy, response
mapper, or presentation:

```bash
neko release pipeline --unit cli
neko release pipeline --unit cli --verify-remote
neko release pipeline --unit cli --verify-remote --output json
```

The Pipeline default calls the local fact boundary, which constructs no GitHub
client and never resolves a token. Its explicit flag calls the same GET reader
and lazy token resolver documented below; there is no second Pipeline HTTP
client. Pipeline maps Doctor states once into its own closed vocabulary and
generates stable fact IDs from neutral identity fields. It does not consume
Doctor diagnostics, remediation, readiness, exit policy, or human formatting.

Pipeline's verification summary is independent from its lifecycle status.
Remote workflow identity, Actions settings, variables, releases, tags, and
artifacts are verification facts, not execution progress. The verifier owns no
durable workflow-run ID and does not observe publication completion, so remote
verification cannot mark a stage complete, authorize resume/retry, or change an
accepted handoff into proof of publication success.

## GitHub read operations

The package-private client targets `https://api.github.com` in production and
can use an injected test base URL. Every possible HTTP request uses `GET`:

| Purpose | Exact endpoint shape |
| --- | --- |
| Repository identity, visibility, default branch | `/repos/{owner}/{repository}` |
| Workflow bytes on the default branch | `/repos/{owner}/{repository}/contents/{workflow-path}?ref={default-branch}` |
| Workflow enabled state | `/repos/{owner}/{repository}/actions/workflows/{exact-filename}` |
| One recognized repository variable | `/repos/{owner}/{repository}/actions/variables/{exact-name}` |
| One referenced custom secret name | `/repos/{owner}/{repository}/actions/secrets/{exact-name}` |
| Repository Actions policy | `/repos/{owner}/{repository}/actions/permissions` |
| One exact release | `/repos/{owner}/{repository}/releases/tags/{exact-tag}` |
| One exact tag reference | `/repos/{owner}/{repository}/git/ref/tags/{exact-tag}` |
| One durable exact workflow run, when owned by Doctor | `/repos/{owner}/{repository}/actions/runs/{exact-run-id}` |

There is no list-all-variable, list-all-secret, list-releases, latest-release,
tag-prefix, newest-run, SHA-only run, or fuzzy asset request. Doctor
owns no durable workflow-run ID, so it emits no workflow-run request. The exact
run operation requires an already-owned durable identity; adding journal reads
merely to discover one is prohibited.

Requests use a finite 12-second timeout, reject redirects, cap response bodies
at 1 MiB, decode one deterministic JSON object, and never retry automatically.
An authenticated repository lookup after an anonymous ambiguous `404` or `401`
is the one deliberate identity escalation, not a retry loop. Response bodies,
authorization headers, tokens, scopes, and arbitrary headers never cross the
client boundary. Safe rate-limit output is limited to a validated retry delay
and reset time.

## Authentication decisions

Repository identity is attempted anonymously first. Public workflow content,
workflow metadata, tags, and releases remain anonymous. If repository identity
is missing or unauthorized anonymously, Doctor resolves `GITHUB_TOKEN` once and
performs one authenticated identity lookup. An authenticated exact `404` is
`missing`; an unauthenticated `404` without a usable token is
`unauthorized`, because the repository may be private.

Recognized repository variables, custom-secret name metadata, and Actions
policy are protected metadata checks. They use the same lazily resolved token.
A missing token does not invalidate already verified public facts. Default
offline mode never invokes the token resolver.

## Supported facts

Remote repository verification requires the returned owner/repository to match
the local GitHub origin and requires a non-empty default branch. Workflow
content is read from that exact branch and compared byte-for-byte with the
repository-confined local bytes, including the final newline. The exact
workflow metadata state must be `active`; `disabled_manually` and
`disabled_inactivity` are mismatches, while an unknown state is unsupported.
An enabled repository Actions policy is understood only for GitHub's focused
`all`, `local_only`, and `selected` allowed-actions states; an unknown
state remains unsupported rather than being accepted by guesswork.

Only locally recognized installation pins actually referenced by a workflow
are queried:

- `NEKO_VERSION` for the Neko CLI installer;
- `NEKO_RELEASE_PLUGIN_VERSION` for the Release Plugin installer.

Values must be pinned semantic versions, not `latest`, a tag namespace, or a
discovery expression. The canonical version may appear in safe evidence. No
other `vars.*` reference is queried merely because it exists in YAML.

An exact-source self-release workflow has no installation-pin variables.
Doctor verifies its CLI/plugin builds and temporary-path wiring locally, so
remote verification performs no repository-variable or installation-release
query for it. All three first-party repository workflows use that shape.

Only custom `secrets.*` names already referenced by a local workflow are
queried. Built-in `GITHUB_TOKEN` causes no secrets API request. The secret
metadata model contains only name and safe timestamps; it has no value field,
so a secret value cannot enter a fact, diagnostic, JSON, raw JSON, or human
output.

Installation and publication checks derive exact tags and asset names from the
locally verified unit, installer, manifest, and focused GoReleaser contracts.
The checks cover pinned CLI and Release Plugin installation releases where a
workflow uses them, each selected unit's current exact release and tag, and the
exact `plugin-registry` release/`plugin-index.json` target where plugin
publication uses it. Remote assets are compared by exact name; no prefix or
newest-match fallback exists.

## Result and failure classification

`remote_verification` is additive to the existing Doctor response:

- `requested` records the explicit mode;
- `status` is `not_requested`, `complete`, `partial`, or `unavailable`;
- `verified`, `unresolved`, and `failed` count remote facts only.

Remote fact states are `verified`, `missing`, `mismatch`, `not_attempted`,
`unavailable`, `unauthorized`, `rate_limited`, and `unsupported`. Existing
`not_verifiable` remains available for local/runtime boundaries. Fact order and
diagnostic order remain deterministic, and arrays never become `null`.

Definite negative facts are actionable errors: repository/workflow/release/tag
missing after sufficient proof, workflow bytes mismatched, workflow or Actions
disabled, recognized variables missing or invalid, referenced custom-secret
metadata missing, and required exact assets missing. Any error yields
`not_ready` and exit code `1`.

Unauthorized, rate-limited, unavailable, malformed-response, unsupported, and
ambiguous private-resource outcomes remain unresolved evidence. They do not by
themselves create a structural error: without another error the existing
warning/readiness policy still exits `0`. A failed variable or policy read does
not erase successfully verified repository, workflow, tag, release, or asset
facts.

Remote diagnostics use focused code families:

- `REMOTE_REPOSITORY_*` and `REMOTE_REPOSITORY_IDENTITY_MISMATCH`;
- `REMOTE_WORKFLOW_CONTENT_*`, `REMOTE_WORKFLOW_STATE_*`, and
  `REMOTE_WORKFLOW_DISABLED`;
- `REMOTE_ACTIONS_POLICY_*` and `REMOTE_ACTIONS_DISABLED`;
- `REMOTE_REPOSITORY_VARIABLE_*`;
- `REMOTE_SECRET_METADATA_*`;
- `REMOTE_INSTALLATION_RELEASE_*`, `REMOTE_INSTALLATION_RELEASE_INVALID`, and
  `REMOTE_INSTALLATION_ASSET_MISSING`;
- `REMOTE_PUBLICATION_RELEASE_*`, `REMOTE_PUBLICATION_TAG_*`,
  `REMOTE_PUBLICATION_ASSET_MISSING`, and
  `REMOTE_PUBLICATION_ARTIFACTS_UNSUPPORTED`;
- `REMOTE_PLUGIN_REGISTRY_RELEASE_*` and
  `REMOTE_PLUGIN_REGISTRY_ASSET_MISSING`.

For the `*` families, terminal suffixes describe `MISSING`, `UNAUTHORIZED`,
`RATE_LIMITED`, `UNSUPPORTED`, or `UNAVAILABLE` as applicable. Messages include
only the safe exact subject and sanitized rate-limit metadata, never a raw
private response body.

## Limitations after remote verification

Successful workflow and variable checks replace the corresponding offline
limitations. Verified installation releases/assets narrow
`INSTALLATION_ARTIFACTS_NOT_VERIFIABLE` to runtime download, extraction,
execution, and plugin-loading uncertainty. Missing required remote artifacts
produce a focused error instead of retaining a remote-availability limitation.

`PUBLICATION_CREDENTIALS_NOT_VERIFIABLE` remains because secret-name existence
does not prove issuance, value validity, expiry, authorization, or external
service acceptance. `PUBLICATION_TARGET_NOT_VERIFIABLE` remains narrowed to
version acceptance, upload/overwrite authorization, and service
availability at mutation time. `CONSUMER_BUILD_NOT_VERIFIABLE` remains for
runner, test, binary, and artifact behavior.

`REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE` always remains
`mutation_required`: repository metadata and Actions policy cannot prove the
exact authorization decision for a real dispatch. Doctor never dispatches to
find out. A historical workflow run, even when an exact durable identity is
available, cannot prove that another build or publication will succeed.
