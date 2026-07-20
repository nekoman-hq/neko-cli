package doctor

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

func TestIntegrationDoctorPermissionScopeAcceptsRecognizedSameJobPublication(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "GitHub Release through GoReleaser",
			yaml: `
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
      - uses: goreleaser/goreleaser-action@v6
        with:
          args: release --config .goreleaser.yaml --clean
`,
		},
		{
			name: "GitHub Release through GitHub CLI",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      contents: write
    steps:
      - run: |
          set -euo pipefail
          gh release create "$RELEASE_TAG" dist/*.zip
`,
		},
		{
			name: "GitHub Release asset upload through GitHub CLI",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      contents: write
    steps:
      - run: gh release upload "$RELEASE_TAG" dist/*.zip
`,
		},
		{
			name: "GitHub Packages container publication",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      packages: write
    steps:
      - run: docker push ghcr.io/example/service:"$RELEASE_VERSION"
`,
		},
		{
			name: "GitHub Packages build-push action",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      packages: write
    steps:
      - uses: docker/build-push-action@v6
        with:
          push: true
          tags: ghcr.io/example/service:latest
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := integrationDoctorPermissionDiagnosticCodes(t, test.yaml); len(got) != 0 {
				t.Fatalf("permission diagnostics = %v, want none", got)
			}
		})
	}
}

func TestIntegrationDoctorPermissionScopeRejectsUnsupportedOrUnjustifiedWrites(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "workflow write-all",
			yaml: `
permissions: write-all
jobs:
  publish:
    steps:
      - run: gh release create "$RELEASE_TAG"
`,
		},
		{
			name: "reusable job write-all",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions: write-all
    uses: example/release/.github/workflows/publish.yml@main
`,
		},
		{
			name: "validation job write",
			yaml: `
permissions:
  contents: read
jobs:
  validate:
    permissions:
      contents: write
    steps:
      - run: neko release ci-validate-context
  publish:
    permissions:
      contents: write
    steps:
      - run: gh release create "$RELEASE_TAG"
`,
		},
		{
			name: "publish-looking step name without mutation",
			yaml: `
permissions:
  contents: read
jobs:
  release:
    permissions:
      contents: write
    steps:
      - name: Publish GitHub Release
        run: echo publish
`,
		},
		{
			name: "GoReleaser snapshot without publication",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      contents: write
    steps:
      - uses: goreleaser/goreleaser-action@v6
        with:
          args: release --snapshot --skip=publish --clean
`,
		},
		{
			name: "unrelated action write scope",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      actions: write
    steps:
      - run: gh release create "$RELEASE_TAG"
`,
		},
		{
			name: "unrelated security write scope",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      security-events: write
    steps:
      - run: gh release create "$RELEASE_TAG"
`,
		},
		{
			name: "unrelated deployment write scope",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      deployments: write
    steps:
      - run: gh release create "$RELEASE_TAG"
`,
		},
		{
			name: "package write without package publication",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      packages: write
    steps:
      - run: echo package
`,
		},
		{
			name: "OIDC write without supported publication",
			yaml: `
permissions:
  contents: read
jobs:
  publish:
    permissions:
      id-token: write
    steps:
      - run: cloudctl publish
`,
		},
		{
			name: "unsupported permission value",
			yaml: `
permissions:
  contents: admin
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
`,
		},
		{
			name: "unsupported write value for read-only scope",
			yaml: `
permissions:
  models: write
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
`,
		},
		{
			name: "unsupported permission shape",
			yaml: `
permissions:
  - contents: read
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := integrationDoctorPermissionDiagnosticCodes(t, test.yaml)
			if !slices.Equal(got, []string{"PERMISSIONS_BROAD"}) {
				t.Fatalf("permission diagnostics = %v, want [PERMISSIONS_BROAD]", got)
			}
		})
	}
}

func TestIntegrationDoctorPermissionScopeUnderstandsReadOnlyAndReplacementShapes(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "read-all workflow default",
			yaml: `
permissions: read-all
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
`,
		},
		{
			name: "empty workflow permission mapping",
			yaml: `
permissions: {}
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
`,
		},
		{
			name: "every job declares an explicit read scope",
			yaml: `
jobs:
  validate:
    permissions:
      contents: read
    steps:
      - run: neko release ci-validate-context
  inspect:
    permissions: {}
    steps:
      - run: echo inspect
`,
		},
		{
			name: "current read-only GitHub scopes",
			yaml: `
permissions:
  artifact-metadata: read
  code-quality: read
  models: read
  vulnerability-alerts: read
jobs:
  validate:
    steps:
      - run: neko release ci-validate-context
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := integrationDoctorPermissionDiagnosticCodes(t, test.yaml); len(got) != 0 {
				t.Fatalf("permission diagnostics = %v, want none", got)
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
