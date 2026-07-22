package pipelineinspection

import (
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestPipelineHealthyDefaultIsConciseAndDescribeIsComplete(t *testing.T) {
	response := transportedPipelineResponse(t, mapPipelineResult(pipelinePresentationFixture()))
	options := renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: pipelineTestWidth{width: 120, available: true},
	}
	plain := renderPipelineTransport(t, response, options)
	for _, want := range []string{"Release Pipeline Inspection", "Summary", "Lifecycle", "Verification"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("healthy default omitted %q:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{"Findings", "Verification Facts", "Configured Pipeline", "Limitations"} {
		if strings.Contains(plain, hidden) {
			t.Fatalf("healthy default exposed %q:\n%s", hidden, plain)
		}
	}

	options.Describe = true
	described := renderPipelineTransport(t, response, options)
	for _, want := range []string{
		"Summary", "Verification Facts", "Consumer workflow", "Configured Pipeline",
		"Local release preparation", "Limitations", "Remote Git freshness was not inspected.",
	} {
		if !strings.Contains(described, want) {
			t.Fatalf("healthy describe omitted %q:\n%s", want, described)
		}
	}
}

func TestPipelineDescribeDoesNotChangeCompleteJSON(t *testing.T) {
	response := transportedPipelineResponse(t, mapPipelineResult(pipelinePresentationFixture()))
	plain := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatJSON})
	described := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatJSON, Describe: true})
	if plain != described {
		t.Fatalf("describe changed complete JSON\nplain:\n%s\ndescribed:\n%s", plain, described)
	}
	for _, want := range []string{`"schema_version": 1`, `"verification"`, `"stages"`, `"limitations"`} {
		if !strings.Contains(plain, want) {
			t.Fatalf("complete JSON omitted %s: %s", want, plain)
		}
	}
	for _, forbidden := range []string{"describe_only", "human_table", "human_properties"} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("JSON exposed presentation metadata %q: %s", forbidden, plain)
		}
	}
}

func TestPipelineDescribeIncludesSafeRuntimeEvidenceSections(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Status = pipelineUncertain
	result.Execution = pipelineExecution{
		Present: true, Identity: "execution-identity", JournalCount: 1, UnresolvedCount: 1,
		Validity: "valid", State: "tag-pushed",
		Observations: []pipelineExecutionJournal{{
			Identity: "execution-identity", Reference: "execution/one.json", State: "tag-pushed", Unresolved: true, Valid: true,
		}},
	}
	result.Dispatch = pipelineDispatch{
		Present: true, Identity: "dispatch-identity", JournalCount: 1, Correlation: "exact", State: "unknown",
		Observations: []pipelineDispatchJournal{{
			Identity: "dispatch-identity", Reference: "dispatch/one.json", State: "unknown", Correlation: "exact", Valid: true,
		}},
	}
	result.LocalGit = pipelineLocalGit{
		Scope: "local_only", RemoteFreshness: "remote_not_inspected", Branch: "main", Head: "head-sha",
		ExpectedCommit: "commit-sha", CommitExists: true, CommitContentVerified: true,
		ExpectedTag: "service/v1.2.3", TagExists: true, TagTarget: "commit-sha",
		TagMatchesExpectedCommit: true, HeadContainsExpectedCommit: true, Consistent: true,
	}
	result.Recovery = pipelineRecovery{
		Evaluated: true, Classification: "interrupted-after-tag-push", ResumeEligible: false,
		ResumeRefusal: "ambiguous_dispatch", RetrySafety: "automatic_retry_prohibited",
		Guidance: "Inspect the durable dispatch outcome.", Reasons: []string{"Dispatch outcome is unknown."},
	}
	result.ManualIntervention = pipelineManualIntervention{
		Required: true, Reasons: []string{"Confirm the provider outcome before retrying."},
	}

	response := transportedPipelineResponse(t, mapPipelineResult(result))
	plain := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable})
	for _, hidden := range []string{"Execution Evidence", "Dispatch Evidence", "Local Git Evidence", "execution-identity", "dispatch-identity"} {
		if strings.Contains(plain, hidden) {
			t.Fatalf("default exposed describe evidence %q:\n%s", hidden, plain)
		}
	}
	described := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable, Describe: true})
	for _, want := range []string{
		"Execution Evidence", "execution-identity", "execution/one.json",
		"Dispatch Evidence", "dispatch-identity", "dispatch/one.json",
		"Local Git Evidence", "Expected commit", "Expected tag",
		"Recovery", "Automatic retry prohibited", "Confirm the provider outcome before retrying.",
	} {
		if !strings.Contains(described, want) {
			t.Fatalf("describe omitted safe runtime evidence %q:\n%s", want, described)
		}
	}
}
