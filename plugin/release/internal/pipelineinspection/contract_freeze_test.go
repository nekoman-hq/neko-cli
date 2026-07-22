package pipelineinspection

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestPipelineMachineVocabularyAndExitPolicyAreFrozen(t *testing.T) {
	lifecycles := []pipelineStatus{
		pipelineReady, pipelineActive, pipelineResumable, pipelineCompleted,
		pipelineBlocked, pipelineUncertain, pipelineRejected, pipelineInvalid,
	}
	if got, want := fmt.Sprint(lifecycles), "[ready active resumable completed blocked uncertain rejected invalid]"; got != want {
		t.Fatalf("lifecycle vocabulary = %s, want %s", got, want)
	}
	runtimeStatuses := []RuntimeStatus{
		RuntimeNotObserved, RuntimeNotStarted, RuntimePending, RuntimeConfirmed,
		RuntimeBlocked, RuntimeUnknown, RuntimeRejected, RuntimeInvalid,
	}
	if got, want := fmt.Sprint(runtimeStatuses), "[not_observed not_started pending confirmed blocked unknown rejected invalid]"; got != want {
		t.Fatalf("runtime vocabulary = %s, want %s", got, want)
	}
	verificationStatuses := []VerificationStatus{
		VerificationVerified, VerificationFailed, VerificationUnavailable, VerificationUnauthorized,
		VerificationRateLimited, VerificationNotChecked, VerificationUnresolved,
	}
	if got, want := fmt.Sprint(verificationStatuses), "[verified failed unavailable unauthorized rate_limited not_checked unresolved]"; got != want {
		t.Fatalf("verification vocabulary = %s, want %s", got, want)
	}
	verificationClasses := []VerificationClass{
		VerificationLocal, VerificationRemote, VerificationRuntimeRequired, VerificationMutationRequired,
	}
	if got, want := fmt.Sprint(verificationClasses), "[local remote runtime_required mutation_required]"; got != want {
		t.Fatalf("verification classes = %s, want %s", got, want)
	}

	for _, lifecycle := range lifecycles {
		result := pipelinePresentationFixture()
		result.Status = lifecycle
		result.InvalidEvidence = lifecycle == pipelineInvalid
		response := mapPipelineResult(result)
		wantExit := 0
		if lifecycle == pipelineInvalid {
			wantExit = 1
		}
		if response.ExitCode != wantExit || response.Status != "success" || response.Data["status"] != lifecycle {
			t.Errorf("lifecycle %q response = status %q, exit %d, data status %v", lifecycle, response.Status, response.ExitCode, response.Data["status"])
		}
	}

	for _, status := range verificationStatuses {
		result := pipelinePresentationFixture()
		result.Status = pipelineCompleted
		result.Verification = projectPipelineVerification(VerificationSnapshot{Facts: []VerificationFact{{
			Category: "contract", Class: VerificationRemote, Status: status,
			Subject: "release", Source: "doctor", Scope: "repository",
		}}})
		response := mapPipelineResult(result)
		if response.ExitCode != 0 || response.Data["status"] != pipelineCompleted {
			t.Errorf("verification %q changed lifecycle or exit policy: %#v", status, response)
		}
	}

	failure := mapPipelineFailure(&commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "invalid source"})
	if failure.Status != "error" || failure.ExitCode != 1 || failure.Error == nil {
		t.Fatalf("typed command failure = %#v", failure)
	}
}

func TestPipelineSchemaVersionOneNestedObjectKeysAreFrozen(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Unit.DisplayName = "Service"
	result.Release = pipelineRelease{
		ConfiguredVersion: "1.2.3", TagPrefix: "service/v", ConfiguredTag: "service/v1.2.3",
		MaterializedFiles: []pipelineMaterializedFile{{Path: "release.yaml", Reason: "contract"}},
	}
	result.Repository = pipelineRepository{SourceGeneration: "v2", LocalBranch: "main", LocalHead: "head", Tracking: "origin/main"}
	result.Workflow = pipelineWorkflow{
		Path: ".github/workflows/release-service.yml", Delivery: "github-actions",
		RequiredInputs: []string{"unit", "version", "tag", "release_sha"}, ReleaseTool: "goreleaser",
		ConsumerOperations: []string{"consumer-tests"}, Publication: "configured", PluginRegistry: "not_applicable",
	}
	result.Stages[0].ConditionalReason = "when selected"
	result.Stages[0].RuntimeStatus = RuntimeConfirmed
	result.Stages[0].RuntimeEvidence = "journal phase"
	result.Stages[0].RuntimeReason = "observed"
	result.Stages[0].RuntimeIdentity = "execution"
	result.Stages[0].RuntimeConfirmedAt = "2026-07-22T00:00:00Z"
	result.ProgressInspection = pipelineProgressInspection{
		ExecutionProgress: "active", JournalsInspected: true,
		ResumeEligibilityEvaluated: true, RemoteStateInspected: false,
	}
	result.Execution = pipelineExecution{
		Present: true, Identity: "execution", JournalCount: 1, UnresolvedCount: 1,
		Validity: "valid", State: "tag-created", PendingAction: "push-tag", Terminal: false,
		CreatedAt: "2026-07-22T00:00:00Z", UpdatedAt: "2026-07-22T00:01:00Z",
		JournalReference: "execution/one.json",
		Observations: []pipelineExecutionJournal{{
			Identity: "execution", Reference: "execution/one.json", State: "tag-created",
			Unresolved: true, Valid: true, Problem: "none",
		}},
	}
	result.Dispatch = pipelineDispatch{
		Present: true, Identity: "dispatch", JournalCount: 1, UnlinkedCount: 0,
		Correlation: "exact", State: "accepted", WorkflowPath: result.Workflow.Path, RunID: "42",
		Observations: []pipelineDispatchJournal{{
			Identity: "dispatch", Reference: "dispatch/one.json", State: "accepted",
			Correlation: "exact", Valid: true, Problem: "none",
		}},
	}
	result.LocalGit = pipelineLocalGit{
		Scope: "local_only", RemoteFreshness: "remote_not_inspected", Branch: "main", Head: "head",
		IndexState: "clean", WorktreeState: "clean", ExpectedCommit: "commit", CommitExists: true,
		CommitContentVerified: true, ExpectedTag: "service/v1.2.3", TagExists: true, TagTarget: "commit",
		TagMatchesExpectedCommit: true, HeadContainsExpectedCommit: true,
		IndexContainsRecoveryEvidence: true, WorktreeContainsRecoveryEvidence: true,
		Consistent: true, Problem: "none",
	}
	result.Recovery = pipelineRecovery{
		Evaluated: true, Classification: "interrupted-after-tag", SafeToContinue: true,
		ResumeEligible: true, ResumeOperation: "resume_from_tag_created", ResumeRefusal: "none",
		RetrySafety: "safe", ManualInterventionRequired: false, Guidance: "resume", Reasons: []string{"reason"},
	}
	result.ManualIntervention = pipelineManualIntervention{Required: false, Reasons: []string{"none"}}
	result.Verification = projectPipelineVerification(VerificationSnapshot{
		RemoteStatus: "complete", RemoteRequested: true, RemoteAttempted: true,
		Facts: []VerificationFact{{
			Category: "consumer_structure", Class: VerificationRemote, Status: VerificationVerified,
			Subject: "workflow", Evidence: "verified", Source: "doctor", Scope: "workflow",
			References: []string{"workflow.yml"}, Unit: "service", Workflow: "workflow.yml",
		}},
	})

	encoded, err := json.Marshal(mapPipelineResult(result).Data)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &data); err != nil {
		t.Fatal(err)
	}
	assertPipelineJSONKeys(t, data["unit"], []string{"configured_version", "delivery", "display_name", "executor", "id", "kind", "working_directory"})
	assertPipelineJSONKeys(t, data["release"], []string{"configured_tag", "configured_version", "materialized_files", "tag_prefix"})
	assertPipelineJSONKeys(t, pipelineJSONArrayElement(t, data["release"], "materialized_files", 0), []string{"path", "reason"})
	assertPipelineJSONKeys(t, data["repository"], []string{"local_branch", "local_head", "source_generation", "tracking"})
	assertPipelineJSONKeys(t, data["workflow"], []string{"consumer_operations", "delivery", "path", "plugin_registry", "publication", "release_tool", "required_inputs"})
	assertPipelineJSONKeys(t, pipelineArrayElement(t, data["stages"], 0), []string{
		"conditional_reason", "configuration_status", "id", "label", "location", "mutation", "owner",
		"runtime_confirmed_at", "runtime_evidence", "runtime_identity", "runtime_reason", "runtime_status", "source",
	})
	assertPipelineJSONKeys(t, data["progress_inspection"], []string{"execution_progress", "journals_inspected", "remote_state_inspected", "resume_eligibility_evaluated"})
	assertPipelineJSONKeys(t, data["execution"], []string{
		"created_at", "identity", "journal_count", "journal_reference", "observations", "pending_action",
		"present", "state", "terminal", "unresolved_count", "updated_at", "validity",
	})
	assertPipelineJSONKeys(t, pipelineJSONArrayElement(t, data["execution"], "observations", 0), []string{"identity", "problem", "reference", "state", "unresolved", "valid"})
	assertPipelineJSONKeys(t, data["dispatch"], []string{"correlation", "identity", "journal_count", "observations", "present", "run_id", "state", "unlinked_count", "workflow_path"})
	assertPipelineJSONKeys(t, pipelineJSONArrayElement(t, data["dispatch"], "observations", 0), []string{"correlation", "identity", "problem", "reference", "state", "valid"})
	assertPipelineJSONKeys(t, data["local_git"], []string{
		"branch", "commit_content_verified", "commit_exists", "consistent", "expected_commit", "expected_tag", "head",
		"head_contains_expected_commit", "index_contains_recovery_evidence", "index_state", "problem", "remote_freshness", "scope",
		"tag_exists", "tag_matches_expected_commit", "tag_target", "worktree_contains_recovery_evidence", "worktree_state",
	})
	assertPipelineJSONKeys(t, data["recovery"], []string{
		"classification", "evaluated", "guidance", "manual_intervention_required", "reasons", "resume_eligible",
		"resume_operation", "resume_refusal", "retry_safety", "safe_to_continue",
	})
	assertPipelineJSONKeys(t, data["manual_intervention"], []string{"reasons", "required"})
	assertPipelineJSONKeys(t, data["verification"], []string{"facts", "summary"})
	assertPipelineJSONKeys(t, pipelineJSONArrayElement(t, data["verification"], "facts", 0), []string{
		"category", "class", "evidence", "id", "references", "scope", "source", "status", "subject", "unit", "workflow",
	})
	assertPipelineJSONKeys(t, pipelineJSONObject(t, data["verification"], "summary"), []string{
		"failed", "local_status", "not_checked", "partial", "remote_attempted", "remote_requested",
		"remote_status", "status", "unresolved", "verified",
	})
}

func pipelineJSONObject(t *testing.T, raw json.RawMessage, field string) json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	value, ok := object[field]
	if !ok {
		t.Fatalf("object omits %q", field)
	}
	return value
}

func pipelineJSONArrayElement(t *testing.T, raw json.RawMessage, field string, index int) json.RawMessage {
	t.Helper()
	return pipelineArrayElement(t, pipelineJSONObject(t, raw, field), index)
}

func pipelineArrayElement(t *testing.T, raw json.RawMessage, index int) json.RawMessage {
	t.Helper()
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	if index < 0 || index >= len(values) {
		t.Fatalf("array index %d outside %d values", index, len(values))
	}
	return values[index]
}

func TestPipelineSchemaVersionOneOrderingRemainsDeterministic(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Stages = []LifecycleStage{{ID: "second"}, {ID: "first"}}
	result.Verification = projectPipelineVerification(VerificationSnapshot{Facts: []VerificationFact{
		{Category: "zeta", Class: VerificationRemote, Status: VerificationVerified, Subject: "z", Source: "doctor", Scope: "workflow", References: []string{"z", "a"}},
		{Category: "alpha", Class: VerificationLocal, Status: VerificationVerified, Subject: "a", Source: "doctor", Scope: "workflow", References: []string{"b", "a"}},
	}})
	response := mapPipelineResult(result)
	stages, ok := response.Data["stages"].([]LifecycleStage)
	if !ok || len(stages) != 2 || stages[0].ID != "second" || stages[1].ID != "first" {
		t.Fatalf("configured stage order changed: %#v", response.Data["stages"])
	}
	verification, ok := response.Data["verification"].(pipelineVerification)
	if !ok || len(verification.Facts) != 2 || verification.Facts[0].Category != "alpha" ||
		!reflect.DeepEqual(verification.Facts[0].References, []string{"a", "b"}) {
		t.Fatalf("verification ordering changed: %#v", response.Data["verification"])
	}
}
