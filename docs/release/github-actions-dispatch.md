# GitHub Actions Dispatch

The GitHub Actions workflow-dispatch adapter is used by public V2 GitHub Actions releases after Neko CLI has pushed the release commit and unit tag.

## Repository Target

The dispatch target is derived only from the verified V2 Git remote selected by release Git coordination. Neko CLI does not infer owner or repository from V1 fields, workflow paths, environment overrides, or user input.

Supported GitHub.com remote forms:

```text
https://github.com/OWNER/REPOSITORY.git
https://github.com/OWNER/REPOSITORY
ssh://git@github.com/OWNER/REPOSITORY.git
ssh://git@github.com/OWNER/REPOSITORY
git@github.com:OWNER/REPOSITORY.git
git@github.com:OWNER/REPOSITORY
```

Exactly one optional trailing `.git` suffix is removed. GitHub Enterprise Server, GitLab, Bitbucket, local file remotes, arbitrary SSH hosts, query strings, fragments, credentials, traversal, whitespace, extra path segments, and shell-like syntax are rejected. GitHub Enterprise support is intentionally deferred.

## HTTP Contract

The internal client sends exactly one request:

```text
POST https://api.github.com/repos/{owner}/{repo}/actions/workflows/{workflow_filename}/dispatches
```

The workflow identifier is the validated workflow filename, not the full path. The dispatch `ref` is the existing unit tag.

The JSON body contains only:

```json
{
  "ref": "<unit-tag>",
  "inputs": {
    "unit": "...",
    "version": "...",
    "tag": "...",
    "release_sha": "..."
  }
}
```

Executor, delivery, workflow path, repository paths, config paths, secrets, token-derived values, and arbitrary user inputs are never sent. The workflow must read executor and delivery from the checked-out repository configuration. The workflow file must support `workflow_dispatch` and must exist on the repository default branch for GitHub to accept the dispatch.

## Token And Redirects

Real internal dispatch requires `GITHUB_TOKEN` with repository Actions write permission. Dry-run never resolves or requires a token. Missing `GITHUB_TOKEN` fails before `request-started`, so a prepared journal is not converted into an uncertain request.

Redirects are not followed. This prevents an `Authorization` header from being forwarded to another host. Redirect responses are classified as `unknown`.

## Outcomes

Any `2xx` response is `accepted`. Optional safe workflow-run metadata may be recorded, but malformed or absent metadata does not turn a success into a retry.

`400`, `401`, `403`, `404`, `422`, and `429` are `rejected`. Only capped safe error fields such as GitHub's `message` and `documentation_url` are stored.

Timeouts, transport interruptions, context cancellation after request start, `5xx`, redirects, and unexpected statuses are `unknown`. Unknown outcomes are never retried automatically.

## Public Boundary

Public V2 GitHub Actions releases dispatch after commit and tag push. V2 local delivery is unsupported, and no public standalone dispatch or retry command exists.
