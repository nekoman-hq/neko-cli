package releaseworkflow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/localaction"
)

func TestConsumerWorkflowFactsPreserveConfiguredOperationOrder(t *testing.T) {
	facts, err := InspectConsumerWorkflow([]byte(pluginConsumerWorkflowFixture), true, localaction.DeclaredSteps{})
	if err != nil {
		t.Fatalf("InspectConsumerWorkflow: %v", err)
	}
	want := []ConsumerOperationID{
		ConsumerContextValidation,
		ConsumerPluginManifestValidation,
		ConsumerTests,
		ConsumerToolConfigurationCheck,
		ConsumerSnapshotBuild,
		ConsumerWorktreeValidation,
		ConsumerArtifactPackaging,
		ConsumerReleasePublication,
		ConsumerPluginIndexGeneration,
		ConsumerPluginIndexPublication,
	}
	if got := consumerOperationIDs(facts); !reflect.DeepEqual(got, want) {
		t.Fatalf("operation order = %#v, want %#v", got, want)
	}
	for _, operation := range facts.Operations {
		if operation.ToolCommand != "" && operation.ConfigReference != ".goreleaser.yaml" {
			t.Errorf("tool operation %#v has config reference %q", operation.ID, operation.ConfigReference)
		}
	}
}

func TestConsumerWorkflowFactsApplyPluginPredicatesWithoutUnitNames(t *testing.T) {
	pluginFacts, err := InspectConsumerWorkflow([]byte(pluginConsumerWorkflowFixture), true, localaction.DeclaredSteps{})
	if err != nil {
		t.Fatal(err)
	}
	normalFacts, err := InspectConsumerWorkflow([]byte(pluginConsumerWorkflowFixture), false, localaction.DeclaredSteps{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []ConsumerOperationID{
		ConsumerPluginManifestValidation,
		ConsumerPluginIndexGeneration,
		ConsumerPluginIndexPublication,
	} {
		if !HasConsumerOperation(pluginFacts, id) {
			t.Errorf("plugin facts omit %s", id)
		}
		if HasConsumerOperation(normalFacts, id) {
			t.Errorf("normal facts include plugin-only %s", id)
		}
	}
	if !HasConsumerOperation(normalFacts, ConsumerReleasePublication) {
		t.Fatal("normal facts lost the actual configured release publication")
	}
}

func TestConsumerWorkflowFactsClassifyRealToolPublication(t *testing.T) {
	content := []byte(`jobs:
  publish:
    env:
      TOOL_CONFIG: release.yaml
    steps:
      - name: Publish artifacts
        uses: goreleaser/goreleaser-action@v6
        with:
          args: release --config ${{ env.TOOL_CONFIG }} --clean
`)
	facts, err := InspectConsumerWorkflow(content, false, localaction.DeclaredSteps{})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Operations) != 1 {
		t.Fatalf("operations = %#v", facts.Operations)
	}
	operation := facts.Operations[0]
	if operation.ID != ConsumerReleasePublication || !operation.Publishes || operation.Snapshot || operation.SkipPublication || operation.ConfigReference != "release.yaml" {
		t.Fatalf("publication = %#v", operation)
	}
}

func consumerOperationIDs(facts ConsumerWorkflowFacts) []ConsumerOperationID {
	ids := make([]ConsumerOperationID, 0, len(facts.Operations))
	for _, operation := range facts.Operations {
		ids = append(ids, operation.ID)
	}
	return ids
}

const pluginConsumerWorkflowFixture = `jobs:
  validate:
    env:
      TOOL_CONFIG: .goreleaser.yaml
    steps:
      - name: Validate release context
        run: neko release ci-validate-context
      - name: Validate materialized plugin manifest
        run: jq -e '.version' plugin/manifest.json
      - name: Test repository
        run: go test ./...
      - name: Check release configuration
        uses: goreleaser/goreleaser-action@v6
        with:
          args: check --config ${{ env.TOOL_CONFIG }}
      - name: Build snapshot
        uses: goreleaser/goreleaser-action@v6
        with:
          args: build --config ${{ env.TOOL_CONFIG }} --snapshot --clean
      - name: Validate worktree
        run: git diff --exit-code
  publish:
    env:
      TOOL_CONFIG: .goreleaser.yaml
    steps:
      - name: Package artifacts
        uses: goreleaser/goreleaser-action@v6
        with:
          args: release --config ${{ env.TOOL_CONFIG }} --snapshot --skip=publish
      - name: Publish release
        run: gh release create "$RELEASE_TAG"
      - name: Generate registry
        run: .github/scripts/generate-plugin-index.sh
      - name: Publish registry
        run: .github/scripts/publish-plugin-index.sh
`

// TestConsumerOperationsResolveRenamedLocalActionsByContent proves stage
// projection classifies operations from expanded action contents, not from
// workflow filenames or local action directory names.
func TestConsumerOperationsResolveRenamedLocalActionsByContent(t *testing.T) {
	root := t.TempDir()
	writeRenamedActionFixture(t, root, "arbitrary-name/action.yml", `name: Context
description: Validate the dispatched context.
runs:
  using: composite
  steps:
    - name: Validate
      shell: bash
      run: neko release ci-validate-context --unit "$RELEASE_UNIT"
`)
	writeRenamedActionFixture(t, root, "another-name/action.yml", `name: Registry
description: Publish the plugin index.
runs:
  using: composite
  steps:
    - name: Generate
      shell: bash
      run: .github/scripts/generate-plugin-index.sh
    - name: Publish
      shell: bash
      run: .github/scripts/publish-plugin-index.sh
`)
	workflow := []byte(`jobs:
  ship:
    steps:
      - name: Guard
        run: |
          jq -e --arg version "$RELEASE_VERSION" \
            '.version == $version' \
            plugin/renamed/manifest.json >/dev/null
      - uses: ./.github/actions/arbitrary-name
      - name: Test
        run: go test ./...
      - uses: ./.github/actions/another-name
`)
	facts, err := InspectConsumerWorkflow(workflow, true, localaction.NewRepositoryActions(root))
	if err != nil {
		t.Fatal(err)
	}
	want := []ConsumerOperationID{
		ConsumerPluginManifestValidation,
		ConsumerContextValidation,
		ConsumerTests,
		ConsumerPluginIndexGeneration,
		ConsumerPluginIndexPublication,
	}
	if got := consumerOperationIDs(facts); !reflect.DeepEqual(got, want) {
		t.Fatalf("operations = %v, want %v", got, want)
	}
}

func writeRenamedActionFixture(t *testing.T, root, relativePath, content string) {
	t.Helper()
	target := filepath.Join(root, ".github", "actions", filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("create %s parent: %v", relativePath, err)
	}
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", relativePath, err)
	}
}
