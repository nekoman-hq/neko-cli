package doctor

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/localaction"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// Renamed identities used by the fixture below. They deliberately share no
// substring with the repository's own workflow filenames, local action
// directories, or unit ids, so any surviving hardcoding fails these tests.
const (
	renamedFixtureUnitID           = "plugin-shipper"
	renamedFixtureTagPrefix        = "plugin-shipper/v"
	renamedFixtureWorkflowPath     = ".github/workflows/ship.yml"
	renamedFixtureToolchainAction  = ".github/actions/build-tools"
	renamedFixtureContextAction    = ".github/actions/check-context"
	renamedFixtureRegistryAction   = ".github/actions/ship-index"
	renamedFixtureGoReleaserConfig = ".goreleaser.plugin-shipper.yaml"
)

// TestIntegrationDoctorRecognizesRenamedWorkflowAndLocalActions proves the
// inspection identifies the self-release contract from workflow and action
// contents alone. The workflow file, every local action directory, and the
// release unit id are renamed; only the semantics are unchanged.
func TestIntegrationDoctorRecognizesRenamedWorkflowAndLocalActions(t *testing.T) {
	root := newIntegrationDoctorRenamedRepository(t)
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
	if result.Readiness != integrationDoctorReady || result.Summary.Errors != 0 || result.Summary.Warnings != 0 {
		t.Fatalf("readiness=%q summary=%#v diagnostics=%#v", result.Readiness, result.Summary, result.Diagnostics)
	}
	if len(result.Workflows) != 1 || result.Workflows[0].Path != renamedFixtureWorkflowPath {
		t.Fatalf("inspected workflows = %#v", result.Workflows)
	}
	fact, ok := integrationDoctorVerificationByCategory(result.Verifications, renamedFixtureWorkflowPath, "installation_wiring")
	if !ok || fact.State != integrationDoctorVerified ||
		!strings.Contains(fact.Evidence, "builds and wires the CLI and Release Plugin from the exact checked-out commit") {
		t.Fatalf("installation fact = %#v present=%t", fact, ok)
	}
	if fact.Unit != renamedFixtureUnitID {
		t.Fatalf("installation fact unit = %q", fact.Unit)
	}
}

// TestIntegrationDoctorLocatesRenamedLocalActionOperationsByContent proves each
// semantic operation is still located inside the renamed local actions.
func TestIntegrationDoctorLocatesRenamedLocalActionOperationsByContent(t *testing.T) {
	root := newIntegrationDoctorRenamedRepository(t)
	jobs := integrationDoctorWorkflowJobs(
		parseIntegrationDoctorWorkflowBytes(t, readIntegrationDoctorFixtureFile(t, root, renamedFixtureWorkflowPath)),
		localaction.NewRepositoryActions(root.Path()),
	)

	toolchain, _, toolchainOK := integrationDoctorSourceToolchainBuildStep(jobs)
	if !toolchainOK || toolchain.action.ActionPath != renamedFixtureToolchainAction+"/action.yml" {
		t.Fatalf("exact-source toolchain origin = %#v", toolchain.action)
	}
	validators := integrationDoctorMatchingSteps(jobs, func(step integrationDoctorWorkflowStep) bool {
		return strings.Contains(step.run, integrationDoctorContextValidatorCommand)
	})
	if len(validators) != 1 || validators[0].action.ActionPath != renamedFixtureContextAction+"/action.yml" {
		t.Fatalf("context validator steps = %#v", validators)
	}
	registry := integrationDoctorMatchingSteps(jobs, func(step integrationDoctorWorkflowStep) bool {
		return strings.Contains(step.run, ".github/scripts/publish-plugin-index.sh")
	})
	if len(registry) != 1 || registry[0].action.ActionPath != renamedFixtureRegistryAction+"/action.yml" {
		t.Fatalf("plugin index publication steps = %#v", registry)
	}
	for _, reference := range integrationDoctorCredentialReferences(nil, jobs) {
		if !reference.Publication {
			t.Fatalf("credential %q in step %q is not publication-scoped", reference.Name, reference.StepName)
		}
	}
}

// newIntegrationDoctorRenamedRepository builds a single-unit repository whose
// workflow file, local action directories, and unit id are renamed while every
// release semantic stays identical.
func newIntegrationDoctorRenamedRepository(t *testing.T) workspace.RepositoryRoot {
	t.Helper()
	root := t.TempDir()
	writeIntegrationDoctorFixtureFile(t, root, ".git/config",
		[]byte("[remote \"origin\"]\n\turl = https://github.com/nekoman-hq/neko-cli.git\n"))
	for _, relativePath := range []string{
		"go.mod",
		"plugin/release/main.go",
		"plugin/release/manifest.json",
		".github/scripts/generate-plugin-index.sh",
		".github/scripts/publish-plugin-index.sh",
	} {
		writeIntegrationDoctorFixtureFile(t, root, relativePath, readIntegrationDoctorRepositoryFile(t, relativePath))
	}

	// Every renamed identity is rewritten together: local action directories,
	// the release unit id, its tag prefix, and its artifact identity.
	rename := strings.NewReplacer(
		"./.github/actions/setup-source-neko-toolchain", "./"+renamedFixtureToolchainAction,
		"./.github/actions/validate-neko-release-context", "./"+renamedFixtureContextAction,
		"./.github/actions/publish-plugin-index", "./"+renamedFixtureRegistryAction,
		".units.cli.version", `.units["`+renamedFixtureUnitID+`"].version`,
		"plugin-release", renamedFixtureUnitID,
	)
	for source, target := range map[string]string{
		".github/workflows/release-plugin-release.yml":             renamedFixtureWorkflowPath,
		".github/actions/setup-source-neko-toolchain/action.yml":   renamedFixtureToolchainAction + "/action.yml",
		".github/actions/validate-neko-release-context/action.yml": renamedFixtureContextAction + "/action.yml",
		".github/actions/publish-plugin-index/action.yml":          renamedFixtureRegistryAction + "/action.yml",
		".goreleaser.plugin-release.yaml":                          renamedFixtureGoReleaserConfig,
	} {
		writeIntegrationDoctorFixtureFile(t, root, target, []byte(rename.Replace(
			string(readIntegrationDoctorRepositoryFile(t, source)),
		)))
	}

	version := integrationDoctorFixturePluginVersion(t)
	writeIntegrationDoctorFixtureFile(t, root, ".neko/release.config.json", []byte(fmt.Sprintf(`{
  "schemaVersion": 2,
  "units": [
    {
      "id": %[1]q,
      "displayName": "renamed release plugin",
      "paths": ["plugin/release/**"],
      "workingDirectory": ".",
      "tagPrefix": %[2]q,
      "kind": "plugin",
      "plugin": {
        "name": "release",
        "manifest": "plugin/release/manifest.json",
        "assetPrefix": %[1]q,
        "binaryName": %[1]q
      },
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": %[3]q
      }
    }
  ]
}
`, renamedFixtureUnitID, renamedFixtureTagPrefix, renamedFixtureWorkflowPath)))
	writeIntegrationDoctorFixtureFile(t, root, ".neko/release.state.json", []byte(fmt.Sprintf(
		"{\n  \"schemaVersion\": 2,\n  \"units\": {\n    %q: {\n      \"version\": %q\n    }\n  }\n}\n",
		renamedFixtureUnitID, version,
	)))

	resolved, err := workspace.ResolveInspectionRepositoryRoot(root)
	if err != nil {
		t.Fatalf("resolve renamed repository root: %v", err)
	}
	return resolved
}

func integrationDoctorFixturePluginVersion(t *testing.T) string {
	t.Helper()
	manifest := integrationDoctorPluginManifestIdentity{}
	if err := json.Unmarshal(readIntegrationDoctorRepositoryFile(t, "plugin/release/manifest.json"), &manifest); err != nil {
		t.Fatalf("decode Release Plugin manifest: %v", err)
	}
	return manifest.Version
}

func readIntegrationDoctorFixtureFile(t *testing.T, root workspace.RepositoryRoot, relativePath string) []byte {
	t.Helper()
	content, err := (filesystemIntegrationDoctorRepositoryFileReader{}).ReadFile(root.Path(), relativePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", relativePath, err)
	}
	return content
}
