package release

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIntegrationDoctorPermissionParserRetainsWorkflowAndJobScopes(t *testing.T) {
	root := integrationDoctorPermissionWorkflowRoot(t, `
permissions:
  contents: read
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
  publish:
    permissions:
      contents: write
    steps:
      - run: gh release create "$RELEASE_TAG"
`)

	workflowPermissions := workflowMappingValue(root, "permissions")
	if got := workflowScalar(workflowMappingValue(workflowPermissions, "contents")); got != "read" {
		t.Fatalf("workflow contents permission = %q, want read", got)
	}

	jobs := integrationDoctorWorkflowJobs(root)
	if len(jobs) != 2 || jobs[0].id != "validate" || jobs[1].id != "publish" {
		t.Fatalf("parsed jobs = %#v", jobs)
	}
	if jobs[0].permissions != nil {
		t.Fatal("validate job unexpectedly has explicit permissions")
	}
	if got := workflowScalar(workflowMappingValue(jobs[1].permissions, "contents")); got != "write" {
		t.Fatalf("publish contents permission = %q, want write", got)
	}
}

func TestIntegrationDoctorPermissionWarningsPreserveBroadDefaultsAndOmissions(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string
	}{
		{
			name: "workflow write default is broad",
			yaml: `
permissions:
  contents: write
jobs:
  publish:
    steps:
      - run: gh release create "$RELEASE_TAG"
`,
			want: []string{"PERMISSIONS_BROAD"},
		},
		{
			name: "job write-all is broad",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions: write-all
    steps:
      - run: gh release create "$RELEASE_TAG"
`,
			want: []string{"PERMISSIONS_BROAD"},
		},
		{
			name: "write without mutation evidence is broad",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      contents: write
    steps:
      - run: echo publish
`,
			want: []string{"PERMISSIONS_BROAD"},
		},
		{
			name: "omitted workflow and job permissions are implicit",
			yaml: `
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
`,
			want: []string{"PERMISSIONS_IMPLICIT"},
		},
		{
			name: "explicit read default is narrow",
			yaml: `
permissions:
  contents: read
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
`,
			want: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := integrationDoctorPermissionDiagnosticCodes(t, test.yaml)
			if !slices.Equal(got, test.want) {
				t.Fatalf("permission diagnostics = %v, want %v", got, test.want)
			}
		})
	}
}

func integrationDoctorPermissionDiagnosticCodes(t *testing.T, content string) []string {
	t.Helper()
	root := integrationDoctorPermissionWorkflowRoot(t, content)
	codes := make([]string, 0)
	inspectIntegrationDoctorPermissions(root, func(_ integrationDoctorSeverity, code, _, _ string) {
		codes = append(codes, code)
	})
	return codes
}

func integrationDoctorPermissionWorkflowRoot(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		t.Fatalf("parse permission workflow: %v", err)
	}
	root := workflowDocumentRoot(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		t.Fatalf("permission workflow root = %#v", root)
	}
	return root
}
