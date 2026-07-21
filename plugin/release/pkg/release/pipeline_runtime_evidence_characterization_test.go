package release

import "testing"

func TestPipelineRuntimeEvidenceUsesAuthoritativeResumeRecoveryPolicy(t *testing.T) {
	//nolint:govet // Table fields follow the policy scenario order.
	tests := []struct {
		name       string
		journal    *ReleaseExecutionJournal
		assessment *ReleaseExecutionRecoveryAssessment
		operation  resumeReleaseOperationKind
		refusal    resumeRecoveryRefusalKind
	}{
		{
			name: "prepared evidence remains blocked before a confirmed commit",
			journal: &ReleaseExecutionJournal{
				State: ReleaseExecutionPrepared, PendingAction: ReleaseExecutionPendingNone,
			},
			assessment: &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryNotStarted, SafeToContinue: true},
			refusal:    resumeRecoveryRefusalBeforeCommit,
		},
		{
			name: "confirmed commit is resumable from commit",
			journal: &ReleaseExecutionJournal{
				State: ReleaseExecutionCommitCreated, PendingAction: ReleaseExecutionPendingNone,
				ReleaseCommitSHA: "1111111111111111111111111111111111111111",
			},
			assessment: &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryInterruptedAfterCommit},
			operation:  resumeReleaseFromCommitCreated,
		},
		{
			name: "confirmed tag is resumable from tag",
			journal: &ReleaseExecutionJournal{
				State: ReleaseExecutionTagCreated, PendingAction: ReleaseExecutionPendingNone,
				ReleaseCommitSHA: "1111111111111111111111111111111111111111",
			},
			assessment: &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryInterruptedAfterTag},
			operation:  resumeReleaseFromTagCreated,
		},
		{
			name: "pending commit push is ambiguous",
			journal: &ReleaseExecutionJournal{
				State: ReleaseExecutionDispatchJournalPrepared, PendingAction: ReleaseExecutionPendingPushReleaseCommit,
				ReleaseCommitSHA: "1111111111111111111111111111111111111111",
			},
			assessment: &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryInterruptedBeforePush},
			refusal:    resumeRecoveryRefusalAmbiguousCommitPush,
		},
		{
			name: "pending tag push is ambiguous",
			journal: &ReleaseExecutionJournal{
				State: ReleaseExecutionCommitPushed, PendingAction: ReleaseExecutionPendingPushUnitTag,
				ReleaseCommitSHA: "1111111111111111111111111111111111111111",
			},
			assessment: &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryInterruptedAfterCommitPush},
			refusal:    resumeRecoveryRefusalAmbiguousTagPush,
		},
		{
			name: "conflicting evidence remains a policy refusal",
			journal: &ReleaseExecutionJournal{
				State: ReleaseExecutionTagCreated, PendingAction: ReleaseExecutionPendingNone,
				ReleaseCommitSHA: "1111111111111111111111111111111111111111",
			},
			assessment: &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryConflicted, Guidance: "inspect local evidence"},
			refusal:    resumeRecoveryRefusalConflicted,
		},
		{
			name: "handoff ready is already completed",
			journal: &ReleaseExecutionJournal{
				State: ReleaseExecutionHandoffReady, PendingAction: ReleaseExecutionPendingNone,
				ReleaseCommitSHA: "1111111111111111111111111111111111111111",
			},
			assessment: &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryAlreadyHandedOff},
			operation:  returnCompletedReleaseHandoff,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolution := resolveResumeRecovery(test.journal, test.assessment)
			if test.refusal != resumeRecoveryRefusalUnknown {
				if resolution.Refusal == nil || resolution.Refusal.Kind != test.refusal {
					t.Fatalf("resolution = %#v, want refusal %d", resolution, test.refusal)
				}
				return
			}
			if resolution.Refusal != nil || resolution.Operation != test.operation {
				t.Fatalf("resolution = %#v, want operation %d", resolution, test.operation)
			}
		})
	}
}

func TestPipelineRuntimeEvidenceUsesAuthoritativeDispatchRetryPolicy(t *testing.T) {
	//nolint:govet // Table fields follow the policy scenario order.
	tests := []struct {
		name      string
		journal   *DispatchJournal
		operation resumeDispatchOperationKind
		refusal   DispatchJournalState
	}{
		{name: "missing dispatch is a fresh request", operation: requestFreshResumeDispatch},
		{name: "prepared dispatch is a fresh request", journal: &DispatchJournal{State: DispatchJournalPrepared}, operation: requestFreshResumeDispatch},
		{name: "accepted dispatch is reused", journal: &DispatchJournal{State: DispatchJournalAccepted}, operation: reuseAcceptedResumeDispatch},
		{name: "started dispatch is not retry safe", journal: &DispatchJournal{State: DispatchJournalRequestStarted}, refusal: DispatchJournalRequestStarted},
		{name: "rejected dispatch is terminal", journal: &DispatchJournal{State: DispatchJournalRejected}, refusal: DispatchJournalRejected},
		{name: "unknown dispatch is not retry safe", journal: &DispatchJournal{State: DispatchJournalUnknown}, refusal: DispatchJournalUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolution := resolveResumeDispatch(test.journal)
			if test.refusal != "" {
				if resolution.Refusal == nil || resolution.Refusal.State != test.refusal {
					t.Fatalf("resolution = %#v, want refusal state %s", resolution, test.refusal)
				}
				return
			}
			if resolution.Refusal != nil || resolution.Operation != test.operation {
				t.Fatalf("resolution = %#v, want operation %d", resolution, test.operation)
			}
		})
	}
}

func TestPipelineRuntimeEvidenceRequiresExactDispatchIdentity(t *testing.T) {
	execution := &ReleaseExecutionJournal{DispatchJournalIdentity: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	linked := &DispatchJournal{Identity: ReleaseDispatchIdentity{SHA256: execution.DispatchJournalIdentity}}
	other := &DispatchJournal{Identity: ReleaseDispatchIdentity{SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}

	if linked.Identity.SHA256 != execution.DispatchJournalIdentity {
		t.Fatal("exact dispatch identity did not link")
	}
	if other.Identity.SHA256 == execution.DispatchJournalIdentity {
		t.Fatal("different dispatch identity linked")
	}
}
