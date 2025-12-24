# Neko - CLI

Neko is a universal CLI tool for orchestrating release workflows across frontend and backend projects.

Neko is a command-line tool designed to streamline release management
for the Nekoman team. It helps developers and release engineers automate
common tasks such as version bumping, changelog generation, and deployment.

With neko-cli, you can:
- Initialize a new release management
- Automatically update version numbers with one source of truth
- Validate release readiness

---
## Hot To Use

**Global Flags**

`-h` Help displays 

`-v` Verbose Output

## Commands

### `neko init`
Initialize Neko in the current project with the underlying release system.
**Supported Systems**
- `goreleaser` 
- `release-it` (In Progress)
- `jreleaser`  (In Progress)

### `neko release`
Run the release process using the detected or configured tool.  
**Args / Flags:**
- `patch` : increment by 0.0.1
- `minor` : increment by 0.1.0
- `major` : increment by 1.0.0

### `neko version`
Show current version of this repo.  
**Args / Flags:**
- `--set=<version>` : set specific version (In Progress)

### `neko validate`
Show or validate the Neko configuration.  
**Args / Flags:**
- `--config-show` : display current configuration

### `neko history` (In Progress)
Show release/tag history.  

### `neko status` (In Progress)
Display current release status.  
**Checks include:** git clean state, branch, version file, changelog status

### `neko check-release` (In Progress)
Validate whether the project is ready for release (pre-flight checks).





