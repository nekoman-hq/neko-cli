package releaseworkflow

import (
	"reflect"
	"testing"
)

func TestConsumerWorkflowFactsPreserveConfiguredOperationOrder(t *testing.T) {
	facts, err := InspectConsumerWorkflow([]byte(pluginConsumerWorkflowFixture), true)
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
	pluginFacts, err := InspectConsumerWorkflow([]byte(pluginConsumerWorkflowFixture), true)
	if err != nil {
		t.Fatal(err)
	}
	normalFacts, err := InspectConsumerWorkflow([]byte(pluginConsumerWorkflowFixture), false)
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
	facts, err := InspectConsumerWorkflow(content, false)
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
