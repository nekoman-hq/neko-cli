# Neko CLI Command and Flag Reference

> **Audience:** Users and contributors who need the complete public command, flag, I/O, and exit inventory.
>
> **Purpose:** Provide the canonical Core CLI command reference and route Release-specific semantics to their owner.

## Authority and inventory

This page is the canonical Core CLI command reference and authoritative
repository-wide inventory for Core commands and
the first-party plugin manifests shipped in this checkout. The
[Release CLI Reference](release/cli-reference.md) is authoritative for Release
V1/V2 behavior and every Release-local flag. [Installation](installation.md)
is authoritative for the install script and Core self-update contract.

The current public surface has 42 command paths: 15 Core/Cobra paths, 21
Release paths (the manifest overview plus 20 commands), and 6 UI paths (the
manifest overview plus 5 commands). Command aliases: none. The only flag
shorthands are Cobra's `-h` for `--help` and Core's `-v` for `--verbose`.

Installed plugins are discovered from `NEKO_PLUGIN_DIR`, defaulting to
`$HOME/.neko/plugins`. Their `manifest.json` files create the visible command
and local-flag surface; Core generates overview and help without starting the
plugin binary. This page inventories the two first-party manifests in the
repository. A user's installation may contain additional third-party paths.

### Public command inventory

<!-- public-command-inventory:start -->
| Command | Owner | Support | Read or mutate | Network and token | Git and filesystem | Output | Exit | Source or restriction | Replacement |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `neko` | Core | Core | Read-only help | Offline; no token | None | Cobra text | `0`; Cobra failure `1` | No source files | — |
| `neko help [command]` | Cobra | Core | Read-only help | Offline; no token | None | Cobra text | `0`; unknown path `1` | Registered command tree | — |
| `neko completion` | Cobra | Core | Read-only overview | Offline; no token | None | Cobra text | `0` | Select a shell subcommand | — |
| `neko completion bash` | Cobra | Core | Read-only generation | Offline; no token | None | Bash script on stdout | `0`; writer failure `1` | No files are written by the command | — |
| `neko completion fish` | Cobra | Core | Read-only generation | Offline; no token | None | Fish script on stdout | `0`; writer failure `1` | No files are written by the command | — |
| `neko completion powershell` | Cobra | Core | Read-only generation | Offline; no token | None | PowerShell script on stdout | `0`; writer failure `1` | No files are written by the command | — |
| `neko completion zsh` | Cobra | Core | Read-only generation | Offline; no token | None | Zsh script on stdout | `0`; writer failure `1` | No files are written by the command | — |
| `neko version` | Core and `pkg/version` | Core | Inspects build and local Git facts | GitHub GET is attempted only when a GitHub repository identity is resolved; optional `GITHUB_TOKEN` | Local Git inspect; no write | Direct styled text; plugin-response flags are no-ops | `0`, including remote warning outcomes | No Release V1/V2 source files | — |
| `neko update` | Core and `pkg/update` | Core | Dry-run capable; executable replacement | Stable release metadata is required for release builds; optional `GITHUB_TOKEN`; archive/checksum downloads only when applying | No Git; inspects installation and atomically replaces the canonical unmanaged target | Direct progress text; plugin-response flags are no-ops | No-op/success `0`; refusal/failure `1` | Release build; supported self-update platform and installation ownership | Reinstall via installer or package manager when refused |
| `neko plugin` | Core | Core | Read-only help | Offline; no token | Reads no plugin binary | Cobra text | `0` | Plugin manager overview | — |
| `neko plugin list` | Core and `pkg/plugin` | Core | Read-only | Offline; no token | Reads installed manifests | Direct table text | `0`; unreadable directory `1` | `NEKO_PLUGIN_DIR` | — |
| `neko plugin available` | Core and `pkg/plugin` | Core | Read-only local comparison | GitHub GET required; optional `GITHUB_TOKEN` | Reads installed manifests; no write | Direct table text | `0`; lookup failure `1` | Published `plugin-registry` `plugin-index.json` | — |
| `neko plugin install <plugin-name>` | Core and `pkg/plugin` | Core | Mutation | GitHub GET/download required; optional `GITHUB_TOKEN` | Replaces the selected plugin directory; no Git | Direct text | `0`; validation/download/install failure `1` | Registry entry and compatible archive | — |
| `neko plugin uninstall <plugin-name>` | Core and `pkg/plugin` | Core | Mutation | Offline; no token | Removes the selected plugin directory; no Git | Direct text | `0`; missing/removal failure `1` | Installed plugin | — |
| `neko plugin update [plugin-name]` | Core and `pkg/update` | Core | Dry-run capable; plugin replacement | GitHub GET required; archive download unless dry-run/no-op; optional `GITHUB_TOKEN` | Reads manifests and may replace selected plugin directories; no Git | Direct progress text | Selection/listing failure `1`; once selected, summarized per-plugin lookup/install failures exit `0` | One plugin or `--all`; published registry index | — |
| `neko release` | Core manifest loader | Core overview | Read-only manifest overview | Offline; no token | Reads installed manifest only | Static Core text | `0` | Installed Release manifest | — |
| `neko release init` | Release Plugin | V2 only | Guarded local pair creation/replacement | Offline; no token | No Git; writes V2 config/state and recovery evidence | Plugin response: table, json, wide | Explicit `0`/`1` | No active source, or replaceable V2 with `--force` | — |
| `neko release unit-add` | Release Plugin | V2 only | Guarded local pair update | Offline; no token | No Git; appends V2 config/state through pair recovery | Plugin response: table, json, wide | Explicit `0`/`1` | Complete valid V2 pair | — |
| `neko release init-options` | Release Plugin | V2 only | Read-only | Offline; no token | None | Plugin response: table, json, wide | Explicit `0` | No source required | — |
| `neko release migrate` | Release Plugin | V1 to V2 migration | Dry-run or guarded migration | Offline; no token | No Git mutation; writes V2 pair/journals and archives root V1 source | Plugin response: table, json, wide | Explicit `0`/`1` | Root `.release.neko.json` only | Use canonical V2 commands after verification |
| `neko release patch` | Release Plugin | Shared V1/V2 | Dry-run or lifecycle mutation | Dry-run offline; execution may require `GITHUB_TOKEN` and provider/Git remotes | May write release files, commit, tag, push and dispatch according to selected source | Plugin response: table, json, wide | Explicit `0`/`1` | Valid V1 source or complete V2 pair | — |
| `neko release minor` | Release Plugin | Shared V1/V2 | Same as patch | Same as patch | Same as patch | Plugin response: table, json, wide | Explicit `0`/`1` | Valid V1 source or complete V2 pair | — |
| `neko release major` | Release Plugin | Shared V1/V2 | Same as patch | Same as patch | Same as patch | Plugin response: table, json, wide | Explicit `0`/`1` | Valid V1 source or complete V2 pair | — |
| `neko release plan` | Release Plugin | Shared V1/V2 | Read-only | Offline; no token | Inspects local release files; no Git mutation | Plugin response: table, json, wide | Explicit `0`/`1` | Valid V1 source or complete V2 pair | — |
| `neko release doctor` | Release Plugin | V2 only | Read-only | Offline by default; explicit GET-only `--verify-remote`; optional `GITHUB_TOKEN` | Inspects local files; no Git mutation or writes | Plugin response: table, json, wide | Ready/warning `0`; not ready `1` | V2 inspection source | — |
| `neko release units` | Release Plugin | V2 only | Read-only | Offline; no token | Inspects V2 pair; no Git or writes | Plugin response: table, json, wide | Valid `0`; issues/source invalid `1` | V2 inspection source | — |
| `neko release pipeline` | Release Plugin | V2 only | Read-only | Offline by default; explicit GET-only `--verify-remote`; optional `GITHUB_TOKEN` | Inspects local Git/journals/files; never mutates | Plugin response: table, json, wide | Valid observation including blocked `0`; invalid `1` | Valid V2 unit/workflow | — |
| `neko release ci-validate-context` | Release Plugin | V2 only | Read-only domain validation | Offline; no token | Local Git inspect only; Core may append to explicit GitHub command file | Plugin response: table, json, wide, github | Valid `0`; contradiction/failure `1` | Complete valid V2 pair and exact local commit/tag | — |
| `neko release github-workflow-init` | Release Plugin | V2 only | Dry-run or create-only guarded write | Offline; no token | No Git; creates one missing configured workflow and never overwrites differing content | Plugin response: table, json, wide | Explicit `0`/`1` | Complete valid V2 pair with configured workflow | — |
| `neko release resume` | Release Plugin | V2 only | Dry-run or journaled continuation | Dry-run offline; continuation may require `GITHUB_TOKEN`, push and dispatch | May mutate journals, tags, pushes and handoff; never plans a new release | Plugin response: table, json, wide | Safe observation `0`; no journal/refusal/failure `1` | One matching unresolved V2 execution | — |
| `neko release history` | Release Plugin | Shared V1/V2 | Read-only | Offline; no token | Local Git inspect only | Plugin response: table, json, wide | Explicit `0`/`1`; legacy empty observation `0` | Valid V1 source or complete V2 pair | — |
| `neko release contributors` | Release Plugin | Shared V1/V2 | Read-only | Offline; no token | Local Git inspect only | Plugin response: table, json, wide | Explicit `0`/`1` | Valid V1 source or complete V2 pair | — |
| `neko release validate` | Release Plugin | Shared V1/V2 | Read-only | Offline; V1 compatibility reads `GITHUB_TOKEN`; no provider call | Inspects local files; no Git mutation | Plugin response: table, json, wide | Valid `0`; invalid `1` | Valid V1 source or V2 inspection pair | — |
| `neko release evidence` | Release Plugin | Shared V1/V2 | Read-only | Offline; no token | Reads evidence below worktree/Git common dir; no mutation | Plugin response: table, json, wide | Valid/diagnostic observation `0`; invalid filter `1` | Any supported evidence family | — |
| `neko release evidence-archive` | Release Plugin | Shared V1/V2 | Guarded local archive | Offline; no token | No Git mutation; archives then removes exactly one completed evidence source | Plugin response: table, json, wide | Success `0`; guard/failure `1` | Supported completed evidence plus exact digest/confirmation | — |
| `neko release plugin-index` | Release Plugin | V2 only | Read-only raw/check or explicit single-file persist | Offline; no token | No Git; persist atomically replaces only `--output-file` | Raw schema-v1 JSON or Core response JSON/table/wide | Success `0`; check/build/persist failure `1` | Complete V2 plugin units/state/manifests | `--output-file` replaces stale output-path spelling |
| `neko ui` | Core manifest loader | First-party plugin overview | Read-only manifest overview | Offline; no token | Reads installed manifest only | Static Core text | `0` | Installed UI manifest | — |
| `neko ui hello` | UI manifest/Core loader | Manifest-declared but unavailable | Dispatch fails before a handler | Offline; no token | No Git or filesystem mutation | Manifest advertises text/json help; no successful response | Execution failure `1` | The current UI router has no `hello` route | No replacement is declared |
| `neko ui init` | UI Plugin | First-party plugin | Mutation | Offline; no token | No Git; creates/replaces `.ui.neko.json` and creates component directory | Plugin response: table, json, wide | Legacy omitted response exit is implicit `0`; transport failure `1` | Required relative components path; `--force` overwrites config | — |
| `neko ui list` | UI Plugin | First-party plugin | Read-only | GitHub GET required; optional `GITHUB_TOKEN` | Reads `.ui.neko.json` and component directories | Plugin response: table, json, wide | Legacy omitted response exit is implicit `0`; transport failure `1` | Initialized UI project | — |
| `neko ui add [component-name]` | UI Plugin | First-party plugin | Mutation | GitHub GET/download required; optional token only for component listing | No Git; creates/replaces selected component files | Plugin response: table, json, wide | Legacy omitted response exit is implicit `0`; transport failure `1` | Component name or `--all`, not both | — |
| `neko ui remove [component-name]` | UI Plugin | First-party plugin | Mutation | Offline; no token | No Git; recursively removes selected component directories | Plugin response: table, json, wide | Legacy omitted response exit is implicit `0`; transport failure `1` | Component name or `--all`, not both | — |
<!-- public-command-inventory:end -->

The UI `hello` row records an existing manifest/router discrepancy; it is not
presented as a working example. This documentation audit does not change plugin
behavior. Release details, including exact V1/V2 source rules and mutation
ownership, live only in the canonical Release reference.

## Public flag inventory

Every listed flag is scalar. Unless a command rejects the resulting value,
scalar flags may be repeated; the last occurrence wins. Boolean flags accept a
bare flag or an explicit `=true`/`=false`. There are no repeatable slice flags,
public compatibility aliases, or deprecated Core/UI flags.

<!-- public-flag-inventory:start -->
| Command or scope | Flag | Owner | Required | Default | Accepted values | Repeat/conflict | Meaning |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `all commands` | `--help, -h` | Cobra | no | `false` | boolean | Last wins | Render command help and stop |
| `plugin response` | `--describe` | Core | no | `false` | boolean | Last wins; incompatible with successful GitHub output | Add safe human detail; never a command-local request flag |
| `plugin response` | `--github-output-file` | Core | only with GitHub output | `empty` | explicit command-file path | Last wins | Destination used only by `--output github`; never inferred from the environment |
| `plugin response` | `--output` | Core | no | `table` | `table`, `json`, `wide`, `github` | Last wins | Select Core response rendering; never a persistence path |
| `plugin response` | `--verbose, -v` | Core transport | no | `false` | boolean | Last wins | Request captured execution/debug logs where a plugin owns them |
| `neko completion bash` | `--no-descriptions` | Cobra | no | `false` | boolean | Last wins | Omit completion descriptions |
| `neko completion fish` | `--no-descriptions` | Cobra | no | `false` | boolean | Last wins | Omit completion descriptions |
| `neko completion powershell` | `--no-descriptions` | Cobra | no | `false` | boolean | Last wins | Omit completion descriptions |
| `neko completion zsh` | `--no-descriptions` | Cobra | no | `false` | boolean | Last wins | Omit completion descriptions |
| `neko update` | `--dry-run` | Core update | no | `false` | boolean | Last wins | Select release metadata/action and inspect capability without archive download or replacement |
| `neko update` | `--force` | Core update | no | `false` | boolean | Last wins | Reinstall only when installed and selected latest versions are equal |
| `neko plugin install` | `--version` | Core plugin manager | no | `latest` | `latest`, SemVer, `v`-SemVer, or exact matching unit tag | Last wins | Select registry version/tag |
| `neko plugin update` | `--all` | Core plugin update | no | `false` | boolean | Last wins; when true the current implementation ignores positional selection | Select every installed plugin |
| `neko plugin update` | `--dry-run` | Core plugin update | no | `false` | boolean | Last wins | Query registry versions without installing |
| `neko plugin update` | `--force` | Core plugin update | no | `false` | boolean | Last wins | Install registry `latest` even when normal comparison would skip; this is not Core self-update force |
| `neko ui hello` | `--name` | UI manifest | no | `World` | string | Last wins | Manifest greeting name; the UI router has no route for this command |
| `neko ui init` | `--components-path` | UI manifest | yes | none | non-empty relative path | Last wins | Store the component directory in `.ui.neko.json` |
| `neko ui init` | `--force` | UI manifest | no | `false` | boolean | Last wins | Overwrite an existing UI config; unrelated to Core update force |
| `neko ui add` | `--all` | UI manifest | no | `false` | boolean | Mutually exclusive with component argument | Add every discovered component |
| `neko ui remove` | `--all` | UI manifest | no | `false` | boolean | Mutually exclusive with component argument | Remove every installed component directory |
<!-- public-flag-inventory:end -->

Core `neko update --force` is only a same-version reinstall switch. It does not bypass permissions, package-manager ownership, platform checks, integrity or archive validation, and it does not permit a Core downgrade. UI init and plugin update have separate command-owned `--force` meanings shown above.

For installed plugin commands, `--describe`, `--output`, and
`--github-output-file` remain entirely Core-owned. `--verbose` is transported
only as `Request.Context.Verbose`; none of the four appears in a plugin's local
flag map. `--output github` requires an explicit command file and a successful
response that declares GitHub output fields. Other successful plugin responses
fail GitHub encoding rather than inventing fields.

See the [Release local flag inventory](release/cli-reference.md#command-local-flag-reference)
for every Release flag, default, accepted value, restriction, and mutual
exclusion.

## Source classification

| Category | Canonical source | Commands |
| --- | --- | --- |
| Core | Cobra registrations in `cmd` | Help, completion, version, update, plugin management |
| Installed plugin surface | Installed `manifest.json` plus Core loader | Plugin overview, command names, local flags, required markers, declared outputs |
| Release V1 compatibility | `.release.neko.json` and Release compatibility contracts | Shared release, plan, history, contributors, validate and evidence surfaces |
| Release V2 canonical | `.neko/release.config.json` plus `.neko/release.state.json` | Setup, lifecycle, Doctor, Units, Pipeline, workflow init, context validation, resume, evidence and Plugin Index |
| UI | `.ui.neko.json` plus UI manifest/router | UI initialization and component list/add/remove |

V1 and V2 Release sources are never merged. See
[Release V1 versus V2](release/cli-reference.md#release-v1-versus-release-v2)
and the [migration guide](release/migration-v1-to-v2.md).

## Output and process exit

Core direct commands write their own text and treat plugin-response presentation
flags as no-ops. Installed plugin commands return the JSON protocol response;
Core validates and renders it once. Core accepts exactly `table`, `json`,
`wide`, and `github`. `wide` is the extended human table mode. GitHub output is
an explicit command-file integration, not ordinary stdout JSON.

A valid decoded plugin response owns process exit. Explicit `0` through `125`
propagates exactly. Release commands explicitly use `0` or `1`. A legacy plugin
response that omits exit intent remains temporary implicit success, which is
why routed UI response errors do not imply a nonzero process exit.
Malformed/absent responses, subprocess failures without a valid response,
renderer failures, GitHub command-file failures, and ordinary Cobra errors are
Core exit `1`. Errors render once.

## Network and mutation summary

- Help, completion, plugin list/uninstall, UI init/remove, default Release
  inspection, Evidence, and Plugin Index are offline.
- `neko version` may read GitHub release metadata when the current repository
  identifies a GitHub remote. `neko update`, plugin registry commands, and UI
  list/add require GitHub reads. `GITHUB_TOKEN` is optional for those public
  read/download paths.
- Release Doctor and Pipeline contact GitHub only with command-local
  `--verify-remote`. Describe and verbose never enable remote access.
- Actual Release V1/V2 execution and Resume may commit, tag, push, or dispatch
  as documented. A workflow dispatch is a handoff, not publication. Consumer
  workflows own build and publication.
- Dry-run never authorizes an actual Release, plugin install, UI mutation, or
  Core executable replacement. Consult each command row for its exact scope.

## Deprecation and compatibility

No public Core command, Core flag, or routed UI command is marked deprecated.
No command aliases exist. Release has no public deprecated command/flag or
compatibility flag aliases; supported V1 behavior is source compatibility on
shared commands. Internal Go compatibility facades are not CLI commands.

The old local Plugin Index output-path spelling is not an alias: use
`neko release plugin-index --output-file <path>`. Core `--output` accepts only
the renderer values above. Release init's old protocol-only
`--project-type`, `--release-system`, and `--metadata` names are not registered
public flags; use V2 `--executor`, `--delivery`, `--kind`, and the relevant
plugin metadata flags.

## Related documentation

- [Installation and self-update](installation.md)
- [Release CLI Reference](release/cli-reference.md)
- [V1 to V2 migration](release/migration-v1-to-v2.md)
- [Release V2 GitHub Actions Golden Path](release/github-actions-golden-path.md)
- [UI plugin](plugins/ui.md)
- [Plugin development](plugin-development.md)
