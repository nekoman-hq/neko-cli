package release

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/localaction"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
	"gopkg.in/yaml.v3"
)

const generatedConsumerPlaceholder = "Replace this generated step with consumer-owned build and publication commands."

type integrationDoctorReadiness string

const integrationDoctorReady integrationDoctorReadiness = "ready"

type integrationDoctorSeverity string

const integrationDoctorNotVerifiable integrationDoctorSeverity = "not_verifiable"

type integrationDoctorVerificationState string

const (
	integrationDoctorVerified     integrationDoctorVerificationState = "verified"
	integrationDoctorUnverifiable integrationDoctorVerificationState = "not_verifiable"
)

type integrationDoctorLimitationClass string

type integrationDoctorSummary struct {
	Errors          int `json:"errors"`
	Warnings        int `json:"warnings"`
	Recommendations int `json:"recommendations"`
	NotVerifiable   int `json:"not_verifiable"`
	Verified        int `json:"verified"`
}

type integrationDoctorUnit struct {
	ID string `json:"id"`
}

type integrationDoctorWorkflow struct {
	Path           string   `json:"path"`
	Classification string   `json:"classification"`
	Units          []string `json:"units"`
	Exists         bool     `json:"exists"`
}

type integrationDoctorVerification struct {
	Subject         string                             `json:"subject"`
	Category        string                             `json:"category"`
	State           integrationDoctorVerificationState `json:"state"`
	Evidence        string                             `json:"evidence"`
	Unit            string                             `json:"unit,omitempty"`
	Workflow        string                             `json:"workflow,omitempty"`
	LimitationClass integrationDoctorLimitationClass   `json:"limitation_class,omitempty"`
	References      []string                           `json:"references"`
}

type integrationDoctorDiagnostic struct {
	Severity    integrationDoctorSeverity `json:"severity"`
	Scope       string                    `json:"scope"`
	Unit        string                    `json:"unit,omitempty"`
	Workflow    string                    `json:"workflow,omitempty"`
	Code        string                    `json:"code"`
	Message     string                    `json:"message"`
	Remediation string                    `json:"remediation"`
}

type integrationDoctorResult struct {
	Readiness     integrationDoctorReadiness      `json:"readiness"`
	Units         []integrationDoctorUnit         `json:"units"`
	Workflows     []integrationDoctorWorkflow     `json:"workflows"`
	Verifications []integrationDoctorVerification `json:"verifications"`
	Diagnostics   []integrationDoctorDiagnostic   `json:"diagnostics"`
	Summary       integrationDoctorSummary        `json:"summary"`
}

func runIntegrationDoctor(t *testing.T, root workspace.RepositoryRoot, flags map[string]any) *plugin.Response {
	t.Helper()
	response, err := HandleDoctorAt(root, plugin.Request{Command: "doctor", Flags: flags})
	if err != nil {
		t.Fatalf("HandleDoctorAt: %v", err)
	}
	return response
}

func integrationDoctorResultFromResponse(t *testing.T, response *plugin.Response) integrationDoctorResult {
	t.Helper()
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("marshal Doctor response data: %v", err)
	}
	result := integrationDoctorResult{}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatalf("decode Doctor response data: %v", err)
	}
	return result
}

func assertIntegrationDoctorCodes(t *testing.T, diagnostics []integrationDoctorDiagnostic, codes ...string) {
	t.Helper()
	for _, code := range codes {
		found := false
		for _, diagnostic := range diagnostics {
			found = found || diagnostic.Code == code
		}
		if !found {
			t.Fatalf("diagnostic %s missing from %#v", code, diagnostics)
		}
	}
}

func assertIntegrationDoctorCodeAbsent(t *testing.T, diagnostics []integrationDoctorDiagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			t.Fatalf("unexpected diagnostic %s in %#v", code, diagnostics)
		}
	}
}

func newIntegrationDoctorRepository(t *testing.T, units map[string]string) workspace.RepositoryRoot {
	t.Helper()
	return newWorkflowScaffoldRepository(t, units)
}

func customIntegrationDoctorWorkflow(t *testing.T) []byte {
	t.Helper()
	workflow, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("render canonical workflow: %v", err)
	}
	placeholder := []byte("          echo \"::error::" + generatedConsumerPlaceholder + "\" >&2\n          exit 1")
	consumer := []byte("          echo \"publish $RELEASE_TAG\"")
	custom := bytes.Replace(workflow, placeholder, consumer, 1)
	if bytes.Equal(custom, workflow) {
		t.Fatal("canonical consumer placeholder was not replaced")
	}
	return custom
}

func writeIntegrationDoctorWorkflow(t *testing.T, root workspace.RepositoryRoot, relativePath string, content []byte) string {
	t.Helper()
	path := filepath.Join(root.Path(), filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create workflow parent: %v", err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	return path
}

type integrationDoctorWorkflowStep struct {
	node   *yaml.Node
	name   string
	id     string
	uses   string
	run    string
	action localaction.Origin
}

type integrationDoctorWorkflowJob struct {
	node        *yaml.Node
	permissions *yaml.Node
	id          string
	steps       []integrationDoctorWorkflowStep
}

// integrationDoctorWorkflowJobs reads the effective steps of one repository
// workflow, expanding the repository-local composite actions it invokes.
func integrationDoctorWorkflowJobs(root *yaml.Node) []integrationDoctorWorkflowJob {
	jobsNode := workflowMappingValue(root, "jobs")
	jobs := make([]integrationDoctorWorkflowJob, 0)
	if jobsNode == nil || jobsNode.Kind != yaml.MappingNode {
		return jobs
	}
	expander := localaction.NewRepositoryActions(repositoryRootForSelfMigrationTest())
	for index := 0; index+1 < len(jobsNode.Content); index += 2 {
		jobNode := jobsNode.Content[index+1]
		stepsNode := workflowMappingValue(jobNode, "steps")
		job := integrationDoctorWorkflowJob{
			id: jobsNode.Content[index].Value, node: jobNode,
			permissions: workflowMappingValue(jobNode, "permissions"),
		}
		if stepsNode != nil && stepsNode.Kind == yaml.SequenceNode {
			for _, effective := range expander.Expand(stepsNode.Content) {
				job.steps = append(job.steps, integrationDoctorWorkflowStep{
					node:   effective.Node,
					name:   workflowScalar(workflowMappingValue(effective.Node, "name")),
					id:     workflowScalar(workflowMappingValue(effective.Node, "id")),
					uses:   workflowScalar(workflowMappingValue(effective.Node, "uses")),
					run:    workflowScalar(workflowMappingValue(effective.Node, "run")),
					action: effective.Origin,
				})
			}
		}
		jobs = append(jobs, job)
	}
	return jobs
}

// integrationDoctorEffectiveJobText renders every effective step of one job in
// execution order.
func integrationDoctorEffectiveJobText(job integrationDoctorWorkflowJob) string {
	text := make([]string, 0, len(job.steps))
	for _, step := range job.steps {
		text = append(text, workflowNodeText(step.node))
	}
	return strings.Join(text, "")
}

func workflowDocumentRoot(document *yaml.Node) *yaml.Node {
	if document != nil && document.Kind == yaml.DocumentNode && len(document.Content) == 1 {
		return document.Content[0]
	}
	return document
}

func workflowMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func workflowMappingKeys(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		keys = append(keys, node.Content[index].Value)
	}
	return keys
}

func workflowScalar(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}

func workflowBool(node *yaml.Node) (bool, bool) {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
		return false, false
	}
	return node.Value == "true", true
}

func workflowNodeText(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return ""
	}
	_ = encoder.Close()
	return output.String()
}
