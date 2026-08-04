package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
	"gopkg.in/yaml.v3"
)

type repositoryWorkflowBehavior struct {
	unit             string
	path             string
	goreleaserConfig string
	buildCommand     string
	packageCommand   string
	publishCommand   string
	versionVariable  string
	pluginRegistry   bool
}

func TestRepositoryDogfoodWorkflowsRetainConsumerBehavior(t *testing.T) {
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			content, root := readRepositoryWorkflow(t, behavior.path)
			jobs := integrationDoctorWorkflowJobs(root)
			if len(jobs) == 0 {
				t.Fatalf("%s has no executable jobs", behavior.path)
			}

			workflowText := string(content)
			for _, required := range []string{
				"actions/setup-go@v5",
				"goreleaser/goreleaser-action@v6",
				"go test ./...",
				"GORELEASER_CONFIG: " + behavior.goreleaserConfig,
				behavior.buildCommand,
				behavior.packageCommand,
				behavior.publishCommand,
				behavior.versionVariable,
				"secrets.GITHUB_TOKEN",
				"git diff --exit-code",
			} {
				if required != "" && !strings.Contains(workflowText, required) {
					t.Errorf("%s lost consumer behavior %q", behavior.path, required)
				}
			}
			if strings.Contains(workflowText, generatedConsumerPlaceholder) {
				t.Fatalf("%s contains the generated failing consumer placeholder", behavior.path)
			}

			if behavior.pluginRegistry {
				effective := repositoryEffectiveWorkflowText(t, root)
				assertRepositoryWorkflowOrder(t, effective,
					behavior.publishCommand,
					".github/scripts/generate-plugin-index.sh",
					".github/scripts/publish-plugin-index.sh",
				)
				for _, required := range []string{"plugin-index.json", "plugin-registry", "PLUGIN_REGISTRY_TARGET_SHA"} {
					if !strings.Contains(effective, required) {
						t.Errorf("%s lost plugin registry behavior %q", behavior.path, required)
					}
				}
			}
		})
	}
}

func TestRepositoryDogfoodWorkflowDispatchUsesSharedContract(t *testing.T) {
	canonicalInputs := releaseworkflow.CanonicalDispatchInputContract()
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			_, root := readRepositoryWorkflow(t, behavior.path)
			on := workflowMappingValue(root, "on")
			if keys := workflowMappingKeys(on); len(keys) != 1 || keys[0] != "workflow_dispatch" {
				t.Fatalf("%s triggers = %v, want only workflow_dispatch", behavior.path, keys)
			}
			inputs := workflowMappingValue(workflowMappingValue(on, "workflow_dispatch"), "inputs")
			if len(workflowMappingKeys(inputs)) != len(canonicalInputs) {
				t.Fatalf("%s dispatch inputs = %v", behavior.path, workflowMappingKeys(inputs))
			}
			for _, definition := range canonicalInputs {
				input := workflowMappingValue(inputs, definition.Name)
				required, requiredOK := workflowBool(workflowMappingValue(input, "required"))
				if input == nil || !requiredOK || !required || workflowScalar(workflowMappingValue(input, "type")) != "string" {
					t.Errorf("%s input %q is not a required string", behavior.path, definition.Name)
				}
			}
		})
	}
}

func repositoryWorkflowBehaviors() []repositoryWorkflowBehavior {
	return []repositoryWorkflowBehavior{
		{
			unit:             "cli",
			path:             ".github/workflows/release-neko-cli.yml",
			goreleaserConfig: ".goreleaser.cli.yaml",
			buildCommand:     "build --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --single-target",
			packageCommand:   "check --config ${{ env.GORELEASER_CONFIG }}",
			publishCommand:   "release --config ${{ env.GORELEASER_CONFIG }} --clean",
			versionVariable:  "CLI_VERSION",
		},
		{
			unit:             "plugin-release",
			path:             ".github/workflows/release-plugin-release.yml",
			goreleaserConfig: ".goreleaser.plugin-release.yaml",
			buildCommand:     "build --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --single-target",
			packageCommand:   "release --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --skip=publish",
			publishCommand:   "gh release create \"$RELEASE_TAG\"",
			versionVariable:  "PLUGIN_RELEASE_VERSION",
			pluginRegistry:   true,
		},
		{
			unit:             "plugin-ui",
			path:             ".github/workflows/release-plugin-ui.yml",
			goreleaserConfig: ".goreleaser.plugin-ui.yaml",
			buildCommand:     "build --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --single-target",
			packageCommand:   "release --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --skip=publish",
			publishCommand:   "gh release create \"$RELEASE_TAG\"",
			versionVariable:  "PLUGIN_UI_VERSION",
			pluginRegistry:   true,
		},
	}
}

func readRepositoryWorkflow(t *testing.T, relativePath string) ([]byte, *yaml.Node) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryRootForSelfMigrationTest(), filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("parse %s: %v", relativePath, err)
	}
	root := workflowDocumentRoot(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		t.Fatalf("%s root is not a YAML mapping", relativePath)
	}
	return content, root
}

// TestRepositoryDogfoodWorkflowsDelegateSharedStepsToLocalActions keeps the
// shared release steps in exactly one place. The workflows invoke the
// repository-local composite actions; the action files own the implementation.
func TestRepositoryDogfoodWorkflowsDelegateSharedStepsToLocalActions(t *testing.T) {
	sharedInvocations := []string{
		"uses: ./.github/actions/setup-source-neko-toolchain",
		"uses: ./.github/actions/validate-neko-release-context",
	}
	duplicatedImplementations := []string{
		"neko release ci-validate-context",
		"go build",
		".github/scripts/generate-plugin-index.sh",
		".github/scripts/publish-plugin-index.sh",
	}
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			content, _ := readRepositoryWorkflow(t, behavior.path)
			workflow := string(content)
			invocations := sharedInvocations
			if behavior.pluginRegistry {
				invocations = append(append([]string(nil), sharedInvocations...), "uses: ./.github/actions/publish-plugin-index")
			}
			for _, invocation := range invocations {
				if strings.Count(workflow, invocation) != 1 {
					t.Errorf("%s must invoke %q exactly once", behavior.path, invocation)
				}
			}
			for _, implementation := range duplicatedImplementations {
				if strings.Contains(workflow, implementation) {
					t.Errorf("%s duplicates local action implementation %q", behavior.path, implementation)
				}
			}
		})
	}
	for path, implementation := range map[string]string{
		".github/actions/setup-source-neko-toolchain/action.yml":   "go build",
		".github/actions/validate-neko-release-context/action.yml": "neko release ci-validate-context",
		".github/actions/publish-plugin-index/action.yml":          ".github/scripts/publish-plugin-index.sh",
	} {
		content, err := os.ReadFile(filepath.Join(repositoryRootForSelfMigrationTest(), filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(content), implementation) {
			t.Errorf("%s no longer owns %q", path, implementation)
		}
	}
}

// repositoryEffectiveWorkflowText renders every effective step of one workflow
// in execution order, including the steps its repository-local actions run.
func repositoryEffectiveWorkflowText(t *testing.T, root *yaml.Node) string {
	t.Helper()
	jobs := integrationDoctorWorkflowJobs(root)
	effective := make([]string, 0, len(jobs))
	for _, job := range jobs {
		effective = append(effective, integrationDoctorEffectiveJobText(job))
	}
	return strings.Join(effective, "")
}

func assertRepositoryWorkflowOrder(t *testing.T, content string, fragments ...string) {
	t.Helper()
	previous := -1
	for _, fragment := range fragments {
		index := strings.Index(content, fragment)
		if index < 0 {
			t.Fatalf("workflow is missing ordered fragment %q", fragment)
		}
		if index <= previous {
			t.Fatalf("workflow fragment %q is out of order", fragment)
		}
		previous = index
	}
}
