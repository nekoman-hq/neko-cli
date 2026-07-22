package pipelineinspection

import (
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestPipelineDefaultShowsIndividualActionableVerificationFindings(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Verification = projectPipelineVerification(VerificationSnapshot{
		RemoteStatus: "partial", RemoteRequested: true, RemoteAttempted: true,
		Facts: []VerificationFact{
			{Category: "consumer_structure", Class: VerificationLocal, Status: VerificationVerified, Subject: "workflow", Evidence: "Healthy local workflow."},
			{Category: "publication_identity", Class: VerificationRemote, Status: VerificationFailed, Subject: "service/v1.2.3", Evidence: "Expected release asset is missing."},
			{Category: "remote_workflow_identity", Class: VerificationRemote, Status: VerificationUnauthorized, Subject: "owner/repository", Evidence: "A read token is required."},
			{Category: "repository_variable_values", Class: VerificationRemote, Status: VerificationRateLimited, Subject: "VERSION", Evidence: "GitHub request was rate limited."},
			{Category: "credential_wiring", Class: VerificationRemote, Status: VerificationUnavailable, Subject: "workflow", Evidence: "Credential metadata is unavailable."},
			{Category: "dispatch_authorization", Class: VerificationMutationRequired, Status: VerificationUnresolved, Subject: "workflow", Evidence: "Checked only during dispatch."},
			{Category: "installation_wiring", Class: VerificationRemote, Status: VerificationNotChecked, Subject: "workflow", Evidence: "Remote check was not attempted."},
		},
	})
	response := transportedPipelineResponse(t, mapPipelineResult(result))
	plain := renderPipelineTransport(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: pipelineTestWidth{width: 120, available: true},
	})
	for _, want := range []string{
		"Findings", "Publication identity", "Failed", "Expected release asset is missing.",
		"Remote workflow identity", "Unauthorized", "A read token is required.",
		"Repository variables", "Rate limited", "GitHub request was rate limited.",
		"Credential wiring", "Unavailable", "Credential metadata is unavailable.",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("default omitted actionable verification finding %q:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{
		"Verification Facts", "Configured Pipeline", "Healthy local workflow.",
		"Checked only during dispatch.", "Remote check was not attempted.",
	} {
		if strings.Contains(plain, hidden) {
			t.Fatalf("default exposed non-actionable inventory %q:\n%s", hidden, plain)
		}
	}

	described := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable, Describe: true})
	for _, want := range []string{
		"Findings", "Verification Facts", "Healthy local workflow.",
		"Checked only during dispatch.", "Remote check was not attempted.", "Configured Pipeline",
	} {
		if !strings.Contains(described, want) {
			t.Fatalf("describe omitted complete finding/fact inventory %q:\n%s", want, described)
		}
	}
}

func TestPipelineDefaultShowsActionableRuntimeFindings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pipelineResult)
		want   []string
	}{
		{
			name: "invalid journal",
			mutate: func(result *pipelineResult) {
				result.Status, result.InvalidEvidence = pipelineInvalid, true
				result.Execution = pipelineExecution{JournalCount: 1, Validity: "invalid", Observations: []pipelineExecutionJournal{{Reference: "execution/broken.json", Valid: false, Problem: "Execution journal is malformed."}}}
			},
			want: []string{"Execution journal", "Invalid", "Execution journal is malformed."},
		},
		{
			name: "multiple unresolved executions",
			mutate: func(result *pipelineResult) {
				result.Status, result.InvalidEvidence = pipelineInvalid, true
				result.Execution.Validity, result.Execution.UnresolvedCount = "conflict", 2
			},
			want: []string{"Execution journals", "Invalid", "Multiple unresolved executions"},
		},
		{
			name: "rejected dispatch",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineRejected
				result.Dispatch = pipelineDispatch{Present: true, State: "rejected", Correlation: "exact", Observations: []pipelineDispatchJournal{}}
			},
			want: []string{"Workflow dispatch", "Rejected", "dispatch request was rejected"},
		},
		{
			name: "unknown dispatch",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineUncertain
				result.Dispatch = pipelineDispatch{Present: true, State: "unknown", Correlation: "exact", Observations: []pipelineDispatchJournal{}}
			},
			want: []string{"Workflow dispatch", "Unknown", "dispatch outcome is unknown"},
		},
		{
			name: "missing commit and mismatched tag",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineBlocked
				result.LocalGit = pipelineLocalGit{Scope: "local_only", ExpectedCommit: "commit-sha", ExpectedTag: "service/v1.2.3", TagExists: true, TagTarget: "other-sha", Consistent: false}
			},
			want: []string{"Expected release commit", "Missing", "Expected unit tag", "Mismatched"},
		},
		{
			name: "invalid local Git evidence",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineBlocked
				result.LocalGit = pipelineLocalGit{Scope: "local_only", Consistent: false, Problem: "Local Git evidence contradicts the execution journal."}
			},
			want: []string{"Local Git evidence", "Invalid", "Local Git evidence contradicts the execution journal."},
		},
		{
			name: "blocked recovery",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineBlocked
				result.Execution.Present = true
				result.Recovery = pipelineRecovery{Evaluated: true, Classification: "manual-recovery-required", Guidance: "Inspect local evidence before continuing.", Reasons: []string{"Commit push is unproven."}}
			},
			want: []string{"Recovery", "Blocked", "Inspect local evidence before continuing."},
		},
		{
			name: "manual intervention",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineUncertain
				result.ManualIntervention = pipelineManualIntervention{Required: true, Reasons: []string{"Confirm the provider outcome before retrying."}}
			},
			want: []string{"Manual intervention", "Required", "Confirm the provider outcome before retrying."},
		},
		{
			name: "manual intervention without a recorded reason",
			mutate: func(result *pipelineResult) {
				result.Status = pipelineBlocked
				result.ManualIntervention = pipelineManualIntervention{Required: true, Reasons: []string{}}
			},
			want: []string{"Manual intervention", "Required", "Manual review is required before continuing."},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := pipelinePresentationFixture()
			test.mutate(result)
			response := transportedPipelineResponse(t, mapPipelineResult(result))
			plain := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatTable})
			for _, want := range append([]string{"Findings"}, test.want...) {
				if !strings.Contains(plain, want) {
					t.Fatalf("default omitted actionable runtime finding %q:\n%s", want, plain)
				}
			}
			if strings.Contains(plain, "Configured Pipeline") || strings.Contains(plain, "Verification Facts") {
				t.Fatalf("default exposed describe inventory:\n%s", plain)
			}
		})
	}
}
