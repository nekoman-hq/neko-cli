package pipelineinspection

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestPipelineRemoteDetailVisibilityMatrix(t *testing.T) {
	tests := []struct {
		name, status, finding string
		actionable            bool
	}{
		{name: "healthy", status: string(VerificationVerified)},
		{name: "failed", status: string(VerificationFailed), finding: "Failed", actionable: true},
		{name: "unauthorized", status: string(VerificationUnauthorized), finding: "Unauthorized", actionable: true},
		{name: "rate limited", status: string(VerificationRateLimited), finding: "Rate limited", actionable: true},
		{name: "unavailable", status: string(VerificationUnavailable), finding: "Unavailable", actionable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := pipelinePresentationFixture()
			status := VerificationStatus(test.status)
			result.Verification = projectPipelineVerification(VerificationSnapshot{
				RemoteStatus: "complete", RemoteRequested: true, RemoteAttempted: true,
				Facts: []VerificationFact{
					{Category: "consumer_structure", Class: VerificationLocal, Status: VerificationVerified, Subject: "workflow", Evidence: "Local workflow is healthy."},
					{Category: "publication_identity", Class: VerificationRemote, Status: status, Subject: "service/v1.2.3", Evidence: "Exact safe remote evidence."},
				},
			})
			response := transportedPipelineResponse(t, mapPipelineResult(result))
			plain := renderPipelineTransport(t, response, renderer.RenderOptions{
				Format: renderer.FormatTable, WidthProvider: pipelineTestWidth{width: 120, available: true},
			})
			if strings.Contains(plain, "Findings") != test.actionable {
				t.Fatalf("Findings visibility = %t, want %t:\n%s", strings.Contains(plain, "Findings"), test.actionable, plain)
			}
			if test.actionable {
				for _, want := range []string{"Publication identity", test.finding, "Exact safe remote evidence."} {
					if !strings.Contains(plain, want) {
						t.Fatalf("default omitted actionable remote detail %q:\n%s", want, plain)
					}
				}
			}
			for _, hidden := range []string{"Verification Facts", "Configured Pipeline", "Local workflow is healthy.", "Limitations"} {
				if strings.Contains(plain, hidden) {
					t.Fatalf("default exposed describe-only detail %q:\n%s", hidden, plain)
				}
			}

			described := renderPipelineTransport(t, response, renderer.RenderOptions{
				Format: renderer.FormatTable, Describe: true, WidthProvider: pipelineTestWidth{width: 120, available: true},
			})
			for _, want := range []string{
				"Verification Facts", "Configured Pipeline", "Limitations",
				"Local workflow is healthy.", "Exact safe remote evidence.",
			} {
				if !strings.Contains(described, want) {
					t.Fatalf("remote describe omitted %q:\n%s", want, described)
				}
			}
		})
	}
}

func TestPipelineLifecycleDetailVisibilityMatrix(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pipelineResult)
		want   []string
	}{
		{
			name: "blocked recovery",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineBlocked
				result.Recovery = pipelineRecovery{Evaluated: true, Guidance: "Review the blocked release evidence."}
			},
			want: []string{"Recovery", "Blocked", "Review the blocked release evidence."},
		},
		{
			name: "uncertain recovery",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineUncertain
				result.Recovery = pipelineRecovery{Evaluated: true, Guidance: "Confirm the uncertain provider outcome."}
			},
			want: []string{"Recovery", "Uncertain", "Confirm the uncertain provider outcome."},
		},
		{
			name: "rejected dispatch",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineRejected
				result.Dispatch = pipelineDispatch{
					Present: true, JournalCount: 1, State: "rejected", WorkflowPath: "release.yml",
					Observations: []pipelineDispatchJournal{{State: "rejected", Correlation: "exact", Valid: true}},
				}
			},
			want: []string{"Workflow dispatch", "Rejected", "dispatch request was rejected"},
		},
		{
			name: "invalid journal",
			mutate: func(result *pipelineResult) {
				result.Status, result.InvalidEvidence = pipelineInvalid, true
				result.Execution = pipelineExecution{JournalCount: 1, Validity: "invalid", Observations: []pipelineExecutionJournal{{Reference: "execution/invalid.json", Problem: "Execution journal is malformed."}}}
			},
			want: []string{"Execution journal", "Invalid", "Execution journal is malformed."},
		},
		{
			name: "manual intervention",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineUncertain
				result.ManualIntervention = pipelineManualIntervention{Required: true, Reasons: []string{"Confirm the durable outcome manually."}}
			},
			want: []string{"Manual intervention", "Required", "Confirm the durable outcome manually."},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := pipelinePresentationFixture()
			test.mutate(result)
			response := transportedPipelineResponse(t, mapPipelineResult(result))
			plain := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable})
			for _, want := range append([]string{"Summary", "Findings"}, test.want...) {
				if !strings.Contains(plain, want) {
					t.Fatalf("default omitted actionable lifecycle detail %q:\n%s", want, plain)
				}
			}
			for _, hidden := range []string{"Verification Facts", "Configured Pipeline", "Limitations"} {
				if strings.Contains(plain, hidden) {
					t.Fatalf("default exposed describe-only detail %q:\n%s", hidden, plain)
				}
			}

			described := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable, Describe: true})
			for _, want := range []string{"Summary", "Findings", "Verification Facts", "Configured Pipeline", "Limitations"} {
				if !strings.Contains(described, want) {
					t.Fatalf("describe omitted lifecycle detail %q:\n%s", want, described)
				}
			}
		})
	}
}

func TestPipelineDescribePreservesAllStageGroupsAndGlobalOrder(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Stages = []LifecycleStage{
		{ID: "plan", Label: "Plan release", Owner: StageOwnerNekoCLI, Location: StageLocationLocalProcess, Mutation: MutationNone, ConfigurationStatus: StageConfigured},
		{ID: "push", Label: "Push tag", Owner: StageOwnerRemoteGit, Location: StageLocationRemoteGit, Mutation: MutationRemoteGit, ConfigurationStatus: StageConfigured},
		{ID: "publish", Label: "Publish artifacts", Owner: StageOwnerConsumerWorkflow, Location: StageLocationGitHubActionsRunner, Mutation: MutationPublication, ConfigurationStatus: StageConfigured},
		{ID: "plugin-index-publication", Label: "Publish plugin index", Owner: StageOwnerConsumerWorkflow, Location: StageLocationGitHubActionsRunner, Mutation: MutationPublication, ConfigurationStatus: StageConfigured},
	}
	response := transportedPipelineResponse(t, mapPipelineResult(result))
	plain := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable})
	if strings.Contains(plain, "Configured Pipeline") {
		t.Fatalf("default exposed full stage inventory:\n%s", plain)
	}
	described := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable, Describe: true})
	configuredAt := strings.Index(described, "Configured Pipeline")
	if configuredAt < 0 {
		t.Fatalf("describe omitted configured pipeline:\n%s", described)
	}
	stageOutput := described[configuredAt:]
	previous := -1
	for _, value := range []string{"Local release preparation", "Git and provider handoff", "Consumer workflow", "Plugin registry"} {
		index := strings.Index(stageOutput, value)
		if index <= previous {
			t.Fatalf("stage group %q missing or out of global order:\n%s", value, described)
		}
		previous = index
	}
}

func TestPipelineDetailVisibilityIsResponsiveDeterministicAndANSIFree(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Verification = projectPipelineVerification(VerificationSnapshot{Facts: []VerificationFact{{
		Category: "publication_identity", Class: VerificationRemote, Status: VerificationFailed,
		Subject: "service/v1.2.3", Evidence: "Expected release asset is missing.",
	}}})
	response := transportedPipelineResponse(t, mapPipelineResult(result))
	for _, describeValue := range []bool{false, true} {
		output := renderPipelineTransport(t, response, renderer.RenderOptions{
			Format: renderer.FormatTable, Describe: describeValue,
			WidthProvider: pipelineTestWidth{width: 72, available: true},
		})
		assertPipelineLinesFit(t, output, 72)
		if !strings.Contains(output, "Summary") || !strings.Contains(output, "Findings") {
			t.Fatalf("narrow output omitted essential sections:\n%s", output)
		}
		if strings.Contains(output, "Verification Facts") != describeValue {
			t.Fatalf("narrow describe visibility mismatch:\n%s", output)
		}

		unknownOptions := renderer.RenderOptions{Format: renderer.FormatTable, Describe: describeValue, WidthProvider: pipelineTestWidth{}}
		first := renderPipelineTransport(t, response, unknownOptions)
		second := renderPipelineTransport(t, response, unknownOptions)
		if first != second || !strings.Contains(first, "Details: Expected release asset") {
			t.Fatalf("unknown-width output is not deterministic vertical output:\n%s", first)
		}
	}

	t.Setenv("NO_COLOR", "1")
	var noColor bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: renderer.FormatTable, Describe: true}, &noColor); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(noColor.String(), "\x1b[") {
		t.Fatalf("NO_COLOR output contains ANSI: %q", noColor.String())
	}

	redirect, err := os.CreateTemp(t.TempDir(), "pipeline-output-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if renderErr := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: renderer.FormatTable, Describe: true}, redirect); renderErr != nil {
		t.Fatal(renderErr)
	}
	if closeErr := redirect.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	redirected, err := os.ReadFile(redirect.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(redirected), "\x1b[") {
		t.Fatalf("redirected output contains ANSI: %q", redirected)
	}
	if !strings.Contains(ansi.Strip(string(redirected)), "Verification Facts") {
		t.Fatalf("redirected describe output omitted structured details: %s", redirected)
	}
}

func TestPipelineDescribeDoesNotChangeRawJSON(t *testing.T) {
	machine, err := json.Marshal(mapPipelineResult(pipelinePresentationFixture()).Data)
	if err != nil {
		t.Fatal(err)
	}
	response := mapPipelineResult(pipelinePresentationFixture())
	response.RendererHint = "raw-json"
	response.Data = map[string]any{"raw": string(machine)}
	plain := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable})
	described := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable, Describe: true})
	if described != plain {
		t.Fatalf("describe changed raw JSON\nplain: %s\ndescribe: %s", plain, described)
	}
	for _, forbidden := range []string{"describe_only", "human_table", "human_properties", "\x1b["} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("raw JSON exposed %q: %s", forbidden, plain)
		}
	}
}
