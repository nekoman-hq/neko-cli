package doctor

import (
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestRepositoryDoctorVerifiesPublicationIdentity(t *testing.T) {
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	for _, behavior := range repositoryWorkflowBehaviors() {
		fact, ok := integrationDoctorVerificationByCategory(result.Verifications, behavior.path, "publication_identity")
		if !ok || fact.State != integrationDoctorVerified || !strings.Contains(fact.Evidence, "validated tag/SHA") {
			t.Errorf("%s publication fact = %#v, present=%t", behavior.path, fact, ok)
		}
	}
}

func TestIntegrationDoctorGHReleaseIdentityMatrix(t *testing.T) {
	valid := `
jobs:
  publish:
    steps:
      - name: publish
        env:
          RELEASE_TAG: ${{ needs.validate.outputs.tag }}
          RELEASE_SHA: ${{ needs.validate.outputs.release_sha }}
        run: |
          gh release create "$RELEASE_TAG" \
            dist/*.tar.gz dist/*.zip dist/*_checksums.txt \
            --target "$RELEASE_SHA" --verify-tag
	`
	valid = strings.TrimSpace(valid) + "\n"
	validRoot := parseIntegrationDoctorWorkflowBytes(t, []byte(valid))
	step := integrationDoctorWorkflowJobs(validRoot)[0].steps[0]
	if !integrationDoctorGHReleaseUsesValidatedIdentity(step) {
		t.Fatal("valid gh release identity was rejected")
	}
	for name, replacements := range map[string][2]string{
		"tag mismatch":       {"${{ needs.validate.outputs.tag }}", "literal"},
		"target mismatch":    {`--target "$RELEASE_SHA"`, `--target "other"`},
		"missing checksum":   {"dist/*_checksums.txt", "dist/no-checksum"},
		"missing verify tag": {"--verify-tag", "--no-verify"},
	} {
		t.Run(name, func(t *testing.T) {
			content := strings.Replace(valid, replacements[0], replacements[1], 1)
			mutatedRoot := parseIntegrationDoctorWorkflowBytes(t, []byte(content))
			mutated := integrationDoctorWorkflowJobs(mutatedRoot)[0].steps[0]
			if integrationDoctorGHReleaseUsesValidatedIdentity(mutated) {
				t.Fatal("mismatched gh release identity was accepted")
			}
		})
	}
}

func TestIntegrationDoctorRecognizesSupportedGHReleaseUpload(t *testing.T) {
	if !integrationDoctorRunPublishesGitHubRelease("gh release upload plugin-registry plugin-index.json") {
		t.Fatal("supported gh release upload was not recognized")
	}
}

func TestIntegrationDoctorPluginRegistryPublicationMatrix(t *testing.T) {
	generate := readIntegrationDoctorRepositoryFile(t, ".github/scripts/generate-plugin-index.sh")
	publish := readIntegrationDoctorRepositoryFile(t, ".github/scripts/publish-plugin-index.sh")
	if !integrationDoctorPluginRegistryScriptsSupported(generate, publish) {
		t.Fatal("repository plugin registry scripts were not recognized")
	}
	if integrationDoctorPluginRegistryScriptsSupported(generate, append(publish, []byte("\n/releases/latest\n")...)) {
		t.Fatal("latest-release fallback was accepted")
	}
	unit := releaseconfig.ReleaseUnit{ID: "plugin-release", IsPlugin: true}
	validJobs := []integrationDoctorWorkflowJob{{steps: []integrationDoctorWorkflowStep{
		{run: `gh release create "$RELEASE_TAG"`},
		{run: ".github/scripts/generate-plugin-index.sh"},
		{run: ".github/scripts/publish-plugin-index.sh"},
	}}}
	_, diagnostics := inspectIntegrationDoctorPluginRegistryPublication(
		repositoryInspectionRoot(t).Path(), ".github/workflows/release.yml", unit, validJobs,
		filesystemIntegrationDoctorRepositoryFileReader{},
	)
	if integrationDoctorDiagnosticsContainErrors(diagnostics) {
		t.Fatalf("valid registry diagnostics = %#v", diagnostics)
	}
	invalidJobs := []integrationDoctorWorkflowJob{{steps: []integrationDoctorWorkflowStep{
		{run: ".github/scripts/generate-plugin-index.sh"},
		{run: `gh release create "$RELEASE_TAG"`},
		{run: ".github/scripts/publish-plugin-index.sh"},
	}}}
	_, diagnostics = inspectIntegrationDoctorPluginRegistryPublication(
		repositoryInspectionRoot(t).Path(), ".github/workflows/release.yml", unit, invalidJobs,
		filesystemIntegrationDoctorRepositoryFileReader{},
	)
	assertIntegrationDoctorCodes(t, diagnostics, "PLUGIN_REGISTRY_ORDER_INVALID")
}

func TestIntegrationDoctorUnsupportedDynamicPublicationRemainsLimited(t *testing.T) {
	root := parseIntegrationDoctorWorkflowBytes(t, []byte(`
jobs:
  publish:
    steps:
      - name: publish dynamically
        run: ./consumer-publish
`))
	fact, diagnostics := inspectIntegrationDoctorPublication(
		repositoryInspectionRoot(t).Path(), ".github/workflows/release.yml",
		[]releaseconfig.ReleaseUnit{{ID: "cli"}}, root, integrationDoctorWorkflowJobs(root),
		filesystemIntegrationDoctorRepositoryFileReader{},
		integrationDoctorRepositoryIdentity{Owner: "example", Repository: "tools"}, nil,
	)
	if fact.State != integrationDoctorUnsupported || fact.LimitationClass != integrationDoctorRuntimeLimitation {
		t.Fatalf("fact = %#v", fact)
	}
	assertIntegrationDoctorCodes(t, diagnostics, "PUBLICATION_TARGET_NOT_VERIFIABLE")
}
