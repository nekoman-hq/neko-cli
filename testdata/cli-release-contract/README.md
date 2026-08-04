# CLI Release Contract Fixtures

> **Purpose:** Provide shared offline GitHub release fixtures that verify the Bash installer and Go CLI release resolver follow the same stable CLI release-selection contract.
>
> **Audience:** Maintainers working on Neko CLI installation, version discovery, self-update behavior, and related regression tests.

These fixtures are the single source of truth for stable CLI release selection.
`contract.json` lists each release-list fixture together with the tag that both
resolvers must select:

- the Bash resolver in `install.sh` (`resolve_latest_cli_version`), and
- the Go resolver in `pkg/git` (`github.SelectStableCLIRelease`).

`TestBashAndGoCLIReleaseResolversSelectTheSameStableTag` in
`install_script_test.go` runs both resolvers over every fixture and requires the
same selection, which keeps the installer and the built-in updater from
drifting apart.

Each release-list fixture is a GitHub `GET /repos/{owner}/{repo}/releases`
response page holding a realistic mix of CLI releases, plugin unit releases, the
mutable plugin registry release, drafts, prereleases, and malformed tags.
