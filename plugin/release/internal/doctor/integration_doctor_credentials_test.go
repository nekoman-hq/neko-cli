package doctor

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/localaction"
	"gopkg.in/yaml.v3"
)

func TestRepositoryDoctorClassifiesPublicationCredentials(t *testing.T) {
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	for _, behavior := range repositoryWorkflowBehaviors() {
		fact, ok := integrationDoctorVerificationByCategory(result.Verifications, behavior.path, "credential_wiring")
		if !ok || fact.State != integrationDoctorVerified || !strings.Contains(fact.Evidence, "built-in GitHub token") {
			t.Errorf("%s credential fact = %#v, present=%t", behavior.path, fact, ok)
		}
	}
}

func TestIntegrationDoctorCredentialReferenceClassification(t *testing.T) {
	root, jobs := integrationDoctorCredentialWorkflow(t, `
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          SERVICE_TOKEN: ${{ secrets.RELEASE_SERVICE_TOKEN }}
        run: gh release create "$RELEASE_TAG"
`)
	references := integrationDoctorCredentialReferences(root, jobs)
	if len(references) != 2 {
		t.Fatalf("credential references = %#v", references)
	}
	if references[0].Kind != integrationDoctorBuiltInCredential || references[0].Name != "GITHUB_TOKEN" || !references[0].Publication {
		t.Errorf("built-in reference = %#v", references[0])
	}
	if references[1].Kind != integrationDoctorCustomCredential || references[1].Name != "RELEASE_SERVICE_TOKEN" || !references[1].Publication {
		t.Errorf("custom reference = %#v", references[1])
	}
}

func TestIntegrationDoctorCredentialWiringMatrix(t *testing.T) {
	tests := []struct {
		name        string
		step        string
		permissions string
		wantCode    string
	}{
		{
			name: "publication only",
			step: `
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: gh release create "$RELEASE_TAG"
`,
			permissions: "write",
		},
		{
			name: "credential in validation",
			step: `
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: go test ./...
`,
			permissions: "write",
			wantCode:    "CREDENTIAL_SCOPE_INVALID",
		},
		{
			name: "credential echoed",
			step: `
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh release create "$RELEASE_TAG"
          echo "$GH_TOKEN"
`,
			permissions: "write",
			wantCode:    "CREDENTIAL_EXPOSURE_RISK",
		},
		{
			name: "credential in output",
			step: `
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh release create "$RELEASE_TAG"
          echo "token=$GH_TOKEN" >> "$GITHUB_OUTPUT"
`,
			permissions: "write",
			wantCode:    "CREDENTIAL_EXPOSURE_RISK",
		},
		{
			name: "permission mismatch",
			step: `
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: gh release create "$RELEASE_TAG"
`,
			permissions: "read",
			wantCode:    "CREDENTIAL_PERMISSION_MISMATCH",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, jobs := integrationDoctorCredentialWorkflowWithPermission(t, test.step, test.permissions)
			fact, diagnostics := inspectIntegrationDoctorCredentialWiring(".github/workflows/release.yml", root, jobs)
			if test.wantCode == "" {
				if fact.State != integrationDoctorVerified || integrationDoctorDiagnosticsContainErrors(diagnostics) {
					t.Fatalf("fact=%#v diagnostics=%#v", fact, diagnostics)
				}
				assertIntegrationDoctorCodes(t, diagnostics, "PUBLICATION_CREDENTIALS_NOT_VERIFIABLE")
				return
			}
			if fact.State != integrationDoctorMismatch {
				t.Errorf("fact state = %q", fact.State)
			}
			assertIntegrationDoctorCodes(t, diagnostics, test.wantCode)
		})
	}
}

func TestIntegrationDoctorCredentialInspectionNeverReadsOrLeaksSecretValues(t *testing.T) {
	const secretValue = "credential-value-that-must-never-appear"
	t.Setenv("GITHUB_TOKEN", secretValue)
	root, jobs := integrationDoctorCredentialWorkflow(t, `
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: gh release create "$RELEASE_TAG"
`)
	fact, diagnostics := inspectIntegrationDoctorCredentialWiring(".github/workflows/release.yml", root, jobs)
	payload, err := json.Marshal(struct {
		Fact        integrationDoctorVerification
		Diagnostics []integrationDoctorDiagnostic
	}{fact, diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), secretValue) {
		t.Fatal("ambient secret value entered Doctor evidence")
	}
	if got := os.Getenv("GITHUB_TOKEN"); got != secretValue {
		t.Fatal("test environment changed unexpectedly")
	}
}

func integrationDoctorCredentialWorkflow(t *testing.T, step string) (*yaml.Node, []integrationDoctorWorkflowJob) {
	t.Helper()
	return integrationDoctorCredentialWorkflowWithPermission(t, step, "write")
}

func integrationDoctorCredentialWorkflowWithPermission(
	t *testing.T,
	step, permission string,
) (*yaml.Node, []integrationDoctorWorkflowJob) {
	t.Helper()
	content := "name: credential-test\npermissions:\n  contents: read\njobs:\n  publish:\n    permissions:\n      contents: " + permission + "\n    steps:\n      - name: publish\n" + step
	root := parseIntegrationDoctorWorkflowBytes(t, []byte(content))
	return root, integrationDoctorWorkflowJobs(root, localaction.DeclaredSteps{})
}
