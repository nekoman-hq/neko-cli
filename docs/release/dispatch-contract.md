# Dispatch Contract

Milestone 5C2 defines the immutable local contract used by the internal GitHub Actions workflow-dispatch adapter. Public release commands still do not dispatch workflows.

## Request

`ReleaseDispatchRequest` is built only after V2 Git coordination has produced a verified release commit and unit tag. It contains:

```text
repositoryRemoteName
unit
version
tag
releaseCommitSHA
workflowPath
workflowFileName
delivery
executor
inputs
identity
```

The workflow path is the validated V2 value:

```text
.github/workflows/<file>.yml
.github/workflows/<file>.yaml
```

The workflow filename is derived from that path and is never separately configured.

The dispatch target is resolved from the exact selected V2 Git remote. Supported remotes are GitHub.com HTTPS, `ssh://git@github.com/...`, and SCP-like `git@github.com:...` forms. GitHub Enterprise, GitLab, Bitbucket, local file remotes, credentials, query strings, fragments, traversal, whitespace, and extra path segments are rejected.

## Inputs

Workflow inputs are deterministic and intentionally small:

```text
unit
version
tag
release_sha
```

No executor, delivery, workflow path, repository path, config path, secret, token, or arbitrary user input is sent as a workflow input. The checked-out workflow must derive executor and delivery from repository configuration, not from dispatch input.

The dispatch ref is the existing unit tag. GitHub Actions then checks out the immutable tag and validates config, state, unit, version, tag, and release SHA before publishing.

## Identity

The dispatch identity includes:

```text
repository remote name
repository remote identity
unit id
version
tag
release commit SHA
workflow path
executor
delivery
```

It is canonicalized and hashed with SHA-256. The hash is used for the journal filename, so tags containing `/` never become path fragments. A different unit, version, tag, commit SHA, workflow, executor, delivery, or remote identity creates a different identity. A different local checkout path for the same Git remote identity does not.

## HTTP Contract

The internal client sends:

```text
POST /repos/{owner}/{repo}/actions/workflows/{workflow_filename}/dispatches
```

with only `ref` and the four canonical inputs. It sets GitHub API headers, uses `GITHUB_TOKEN`, bounds request duration, and disables redirects so authorization cannot be forwarded to another host.

## Current Boundary

Public V2 non-dry-run releases remain blocked before state write, materialization, staging, commit, tag, push, journal write, GitHub API request, executor start, or publish.

V2 dry-run can show the future dispatch contract shape. Because no release commit exists during dry-run, the real `release_sha`, journal identity, and journal path are reported as pending until Git coordination creates the release commit.
