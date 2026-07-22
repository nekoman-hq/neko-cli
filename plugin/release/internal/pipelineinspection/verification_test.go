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
	changedEvidence.Facts[0].Status = VerificationVerified
	changedEvidence.Facts[0].References = []string{"different-reference"}
	changedEvidence.Facts[0].ID = "caller-controlled"
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

func TestPipelineVerificationSummaryStatusContract(t *testing.T) {
	tests := []struct {
		name   string
		status VerificationStatus
		want   verificationSummaryStatus
	}{
		{name: "verified", status: VerificationVerified, want: verificationSummaryVerified},
		{name: "failed", status: VerificationFailed, want: verificationSummaryFailed},
		{name: "unavailable", status: VerificationUnavailable, want: verificationSummaryUnresolved},
		{name: "unauthorized", status: VerificationUnauthorized, want: verificationSummaryUnresolved},
		{name: "rate limited", status: VerificationRateLimited, want: verificationSummaryUnresolved},
		{name: "unresolved", status: VerificationUnresolved, want: verificationSummaryUnresolved},
		{name: "not checked", status: VerificationNotChecked, want: verificationSummaryNotChecked},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projected := projectPipelineVerification(VerificationSnapshot{Facts: []VerificationFact{{
				Category: "category", Class: VerificationLocal, Status: test.status,
				Subject: "subject", Source: "doctor", Scope: "repository",
			}}})
			if projected.Summary.Status != test.want {
				t.Fatalf("summary status = %q, want %q", projected.Summary.Status, test.want)
			}
		})
	}
}

func TestPipelineVerificationFactIDsAreUniqueForNeutralIdentities(t *testing.T) {
	facts := []VerificationFact{
		{Category: "category", Subject: "subject-a", Source: "doctor", Scope: "repository"},
		{Category: "category", Subject: "subject-b", Source: "doctor", Scope: "repository"},
		{Category: "other-category", Subject: "subject-a", Source: "doctor", Scope: "repository"},
	}
	projected := projectPipelineVerification(VerificationSnapshot{Facts: facts})
	seen := make(map[string]bool, len(projected.Facts))
	for _, fact := range projected.Facts {
		if fact.ID == "" || seen[fact.ID] {
			t.Fatalf("fact ID is empty or duplicated: %#v", projected.Facts)
		}
		seen[fact.ID] = true
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
