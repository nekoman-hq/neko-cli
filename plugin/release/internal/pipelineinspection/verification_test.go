package pipelineinspection

import (
	"reflect"
	"strings"
	"testing"
)

func TestPipelineVerificationProjectionUsesStableNeutralFactIDs(t *testing.T) {
	snapshot := VerificationSnapshot{
		RemoteStatus: "not_requested",
		Facts: []VerificationFact{
			{
				Category: "remote_workflow_identity", Class: VerificationRemote, Status: VerificationNotChecked,
				Subject: ".github/workflows/release.yml", Evidence: "Remote state was not checked.",
				Source: "doctor", Scope: "workflow", Workflow: ".github/workflows/release.yml",
				References: []string{".github/workflows/release.yml", ".github/workflows/release.yml"},
			},
			{
				Category: "consumer_structure", Class: VerificationLocal, Status: VerificationVerified,
				Subject: ".github/workflows/release.yml", Evidence: "Local structure was verified.",
				Source: "doctor", Scope: "workflow", Workflow: ".github/workflows/release.yml",
				References: []string{".goreleaser.yaml", ".github/workflows/release.yml"},
			},
		},
	}
	first := projectPipelineVerification(snapshot)
	changedEvidence := snapshot
	changedEvidence.Facts = append([]VerificationFact(nil), snapshot.Facts...)
	changedEvidence.Facts[0].Evidence = "A different non-identity message."
	second := projectPipelineVerification(changedEvidence)

	if len(first.Facts) != 2 || first.Facts[0].Category != "consumer_structure" {
		t.Fatalf("facts are not deterministically ordered: %#v", first.Facts)
	}
	if first.Facts[1].ID != second.Facts[1].ID || !strings.HasPrefix(first.Facts[1].ID, "verification-") {
		t.Fatalf("fact ID depends on volatile evidence: %q / %q", first.Facts[1].ID, second.Facts[1].ID)
	}
	if got := first.Facts[1].References; !reflect.DeepEqual(got, []string{".github/workflows/release.yml"}) {
		t.Fatalf("references = %#v", got)
	}
	if first.Summary.Status != verificationSummaryPartial || first.Summary.Verified != 1 ||
		first.Summary.NotChecked != 1 || first.Summary.Unresolved != 0 || !first.Summary.Partial {
		t.Fatalf("summary = %#v", first.Summary)
	}
}

func TestPipelineVerificationDoesNotChangeLifecycleStatus(t *testing.T) {
	result := pipelinePresentationFixture()
	want := result.Status
	result.Verification = projectPipelineVerification(VerificationSnapshot{
		RemoteStatus: "partial", RemoteRequested: true, RemoteAttempted: true,
		Facts: []VerificationFact{{
			Category: "remote_workflow_identity", Class: VerificationRemote, Status: VerificationFailed,
			Subject: "owner/repository", Source: "doctor", Scope: "repository", References: []string{},
		}},
	})
	if result.Status != want || result.Verification.Summary.Status != verificationSummaryFailed {
		t.Fatalf("lifecycle=%q verification=%q, want independent %q/failed", result.Status, result.Verification.Summary.Status, want)
	}
	if result.Verification.Summary.RemoteStatus != "partial" || !result.Verification.Summary.RemoteRequested ||
		!result.Verification.Summary.RemoteAttempted {
		t.Fatalf("remote summary = %#v", result.Verification.Summary)
	}
}
