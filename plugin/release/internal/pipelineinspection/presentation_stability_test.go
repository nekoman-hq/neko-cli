package pipelineinspection

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func TestPipelinePresentationHumanizesEveryVerificationVocabularyValue(t *testing.T) {
	categories := map[string]string{
		"consumer_structure": "Consumer workflow", "credential_wiring": "Credential wiring",
		"dispatch_authorization": "Dispatch authorization", "goreleaser_configuration": "GoReleaser configuration",
		"installation_wiring": "Installation wiring", "publication_identity": "Publication identity",
		"remote_workflow_identity": "Remote workflow identity", "repository_variable_values": "Repository variables",
	}
	for machine, human := range categories {
		if got := humanVerificationCategory(machine); got != human {
			t.Errorf("category %q = %q, want %q", machine, got, human)
		}
	}

	classes := map[VerificationClass]string{
		VerificationLocal: "Local", VerificationRemote: "Remote",
		VerificationRuntimeRequired: "Runtime required", VerificationMutationRequired: "Mutation required",
	}
	for machine, human := range classes {
		if got := humanVerificationClass(machine); got != human {
			t.Errorf("class %q = %q, want %q", machine, got, human)
		}
	}

	statuses := map[VerificationStatus]string{
		VerificationVerified: "Verified", VerificationFailed: "Failed",
		VerificationUnavailable: "Unavailable", VerificationUnauthorized: "Unauthorized",
		VerificationRateLimited: "Rate limited", VerificationNotChecked: "Not checked",
		VerificationUnresolved: "Unresolved",
	}
	for machine, human := range statuses {
		if got := humanVerificationStatus(machine); got != human {
			t.Errorf("status %q = %q, want %q", machine, got, human)
		}
	}
}

func TestPipelinePresentationHumanizesEveryLifecycleAndRuntimeValue(t *testing.T) {
	lifecycles := map[pipelineStatus]string{
		pipelineReady: "Ready", pipelineActive: "Incomplete execution", pipelineResumable: "Resumable",
		pipelineCompleted: "Handoff completed", pipelineBlocked: "Blocked", pipelineUncertain: "Uncertain",
		pipelineRejected: "Dispatch rejected", pipelineInvalid: "Invalid evidence",
	}
	for machine, human := range lifecycles {
		if got := humanPipelineLifecycle(machine); got != human {
			t.Errorf("lifecycle %q = %q, want %q", machine, got, human)
		}
	}

	runtimes := map[RuntimeStatus]string{
		RuntimeNotObserved: "—", RuntimeNotStarted: "Not started", RuntimePending: "Pending",
		RuntimeConfirmed: "Confirmed", RuntimeRejected: "Rejected", RuntimeUnknown: "Unknown",
		RuntimeBlocked: "Blocked", RuntimeInvalid: "Invalid",
	}
	for machine, human := range runtimes {
		if got := humanPipelineRuntime(machine); got != human {
			t.Errorf("runtime %q = %q, want %q", machine, got, human)
		}
	}
}

func TestPipelinePresentationDeclaresVerificationAndStageColumnPriorities(t *testing.T) {
	assertPresentationColumns(t, pipelineVerificationColumns, []presentation.Column{
		{Key: "check", Label: "Check", Essential: true},
		{Key: "status", Label: "Status", RoleKey: "status_role", Essential: true},
		{Key: "scope", Label: "Scope", Essential: true},
		{Key: "subject", Label: "Subject"},
		{Key: "evidence", Label: "Evidence"},
	})
	assertPresentationColumns(t, pipelineStageColumns, []presentation.Column{
		{Key: "number", Label: "#"},
		{Key: "stage", Label: "Stage", Essential: true},
		{Key: "runtime", Label: "Runtime", RoleKey: "runtime_role", Essential: true},
		{Key: "owner", Label: "Owner", Essential: true},
		{Key: "location", Label: "Location"},
		{Key: "mutation", Label: "Mutation"},
		{Key: "evidence", Label: "Evidence"},
	})
}

func TestPipelinePresentationHumanizesExecutorDeliveryAndStageMetadata(t *testing.T) {
	for machine, human := range map[string]string{
		"goreleaser": "GoReleaser", "jreleaser": "JReleaser", "release-it": "release-it",
	} {
		if got := humanPipelineExecutor(machine); got != human {
			t.Errorf("executor %q = %q, want %q", machine, got, human)
		}
	}
	if got := humanPipelineDelivery("github-actions"); got != "GitHub Actions" {
		t.Errorf("delivery = %q", got)
	}
	for machine, human := range map[StageOwner]string{
		StageOwnerNekoCLI: "Neko CLI", StageOwnerLocalGit: "Local Git", StageOwnerRemoteGit: "Remote Git",
		StageOwnerGitHubAPI: "GitHub API", StageOwnerConsumerWorkflow: "Consumer workflow", StageOwnerReleaseTool: "Release tool",
	} {
		if got := humanPipelineOwner(machine); got != human {
			t.Errorf("owner %q = %q, want %q", machine, got, human)
		}
	}
	for machine, human := range map[StageLocation]string{
		StageLocationLocalProcess: "Local process", StageLocationLocalRepository: "Local repository",
		StageLocationLocalGit: "Local Git", StageLocationRemoteGit: "Remote Git",
		StageLocationGitHubAPI: "GitHub API", StageLocationGitHubActionsRunner: "GitHub Actions runner",
	} {
		if got := humanPipelineLocation(machine); got != human {
			t.Errorf("location %q = %q, want %q", machine, got, human)
		}
	}
	for machine, human := range map[MutationClass]string{
		MutationNone: "None", MutationFilesystem: "Filesystem", MutationReleaseState: "Release state",
		MutationGitIndex: "Git index", MutationGitObject: "Git object", MutationGitRef: "Git ref",
		MutationRemoteGit: "Remote Git", MutationRemoteAPI: "Remote API", MutationPublication: "Publication",
	} {
		if got := humanPipelineMutation(machine); got != human {
			t.Errorf("mutation %q = %q, want %q", machine, got, human)
		}
	}
}

func TestPipelineStageGroupingUsesOrderedStageMetadataPredicates(t *testing.T) {
	stages := []LifecycleStage{
		{ID: "source-unit-resolution", Owner: StageOwnerNekoCLI, Location: StageLocationLocalProcess, Mutation: MutationNone},
		{ID: "release-commit-push", Owner: StageOwnerRemoteGit, Location: StageLocationRemoteGit, Mutation: MutationRemoteGit},
		{ID: "handoff-confirmation", Owner: StageOwnerNekoCLI, Location: StageLocationLocalRepository, Mutation: MutationReleaseState},
		{ID: "consumer-tests", Owner: StageOwnerConsumerWorkflow, Location: StageLocationGitHubActionsRunner, Mutation: MutationNone, Source: ".github/workflows/release.yml"},
		{ID: "release-publication", Owner: StageOwnerReleaseTool, Location: StageLocationGitHubActionsRunner, Mutation: MutationPublication, Source: ".github/workflows/release.yml"},
		{ID: "plugin-index-generation", Owner: StageOwnerConsumerWorkflow, Location: StageLocationGitHubActionsRunner, Mutation: MutationFilesystem, Source: ".github/workflows/release.yml"},
		{ID: "plugin-index-publication", Owner: StageOwnerConsumerWorkflow, Location: StageLocationGitHubActionsRunner, Mutation: MutationPublication, Source: ".github/workflows/release.yml"},
	}
	want := []string{
		pipelineLocalPreparationGroup, pipelineHandoffGroup, pipelineHandoffGroup,
		pipelineConsumerGroup, pipelineConsumerGroup, pipelinePluginRegistryGroup, pipelinePluginRegistryGroup,
	}
	got := make([]string, 0, len(stages))
	for _, stage := range stages {
		got = append(got, pipelineStageGroup(stage))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stage groups = %#v, want %#v", got, want)
	}
}

func TestPipelineStageRowsPreserveGlobalOrderAndHumanizeEvidence(t *testing.T) {
	stages := []LifecycleStage{
		{ID: "one", Label: "First", Owner: StageOwnerNekoCLI, Location: StageLocationLocalProcess, Mutation: MutationNone, RuntimeStatus: RuntimeConfirmed, RuntimeEvidence: "execution_journal"},
		{ID: "two", Label: "Second", Owner: StageOwnerRemoteGit, Location: StageLocationRemoteGit, Mutation: MutationRemoteGit, RuntimeStatus: RuntimeBlocked, RuntimeEvidence: "resume_policy", RuntimeReason: "unproven_commit_push"},
		{ID: "plugin-index-publication", Label: "Third", Owner: StageOwnerConsumerWorkflow, Location: StageLocationGitHubActionsRunner, Mutation: MutationPublication, RuntimeStatus: RuntimeRejected, RuntimeEvidence: "dispatch_journal"},
	}
	rows := pipelineStageRows(stages)
	if len(rows) != 3 || rows[0]["number"] != 1 || rows[0]["stage"] != "First" ||
		rows[1]["number"] != 2 || rows[1]["stage"] != "Second" ||
		rows[2]["number"] != 3 || rows[2]["stage"] != "Third" {
		t.Fatalf("stage row order changed: %#v", rows)
	}
	if rows[0]["runtime"] != "Confirmed" || rows[1]["runtime"] != "Blocked" || rows[2]["runtime"] != "Rejected" {
		t.Fatalf("runtime labels = %#v", rows)
	}
	if got := rows[1]["evidence"]; got != "Resume policy — Unproven commit push" {
		t.Fatalf("human evidence = %#v", got)
	}
}

func TestPipelineSummaryCoversLifecycleAndNoExecutionSemantics(t *testing.T) {
	for _, status := range []pipelineStatus{
		pipelineReady, pipelineActive, pipelineResumable, pipelineCompleted,
		pipelineBlocked, pipelineUncertain, pipelineRejected, pipelineInvalid,
	} {
		t.Run(string(status), func(t *testing.T) {
			result := pipelinePresentationFixture()
			result.Status = status
			if status == pipelineInvalid {
				result.InvalidEvidence = true
			}
			properties := pipelineSummaryProperties(result)
			if got := presentationPropertyValue(t, properties, "Lifecycle"); got != humanPipelineLifecycle(status) {
				t.Fatalf("Lifecycle = %q", got)
			}
			wantExecution, wantRecovery, wantResume := "No active execution", "Not applicable", "Not applicable"
			if status == pipelineInvalid {
				wantExecution, wantRecovery, wantResume = "No valid execution selected", "Unavailable for invalid evidence", "Unavailable for invalid evidence"
			}
			for label, want := range map[string]string{"Execution": wantExecution, "Recovery": wantRecovery, "Resume": wantResume} {
				if got := presentationPropertyValue(t, properties, label); got != want {
					t.Errorf("%s = %q, want %q", label, got, want)
				}
			}
		})
	}
}

func TestPipelineVerificationSummaryExplainsDeferredAndRemoteOutcomes(t *testing.T) {
	tests := []struct {
		name   string
		remote string
		fact   VerificationFact
		want   string
	}{
		{name: "not requested", remote: "not_requested", want: "Local checks passed; remote checks not requested"},
		{name: "partial", remote: "partial", fact: VerificationFact{Class: VerificationRemote, Status: VerificationUnresolved}, want: "Local checks passed; remote checks need review"},
		{name: "unauthorized", remote: "partial", fact: VerificationFact{Class: VerificationRemote, Status: VerificationUnauthorized}, want: "Local checks passed; remote checks need review"},
		{name: "rate limited", remote: "partial", fact: VerificationFact{Class: VerificationRemote, Status: VerificationRateLimited}, want: "Local checks passed; remote checks need review"},
		{name: "failed", remote: "complete", fact: VerificationFact{Class: VerificationRemote, Status: VerificationFailed}, want: "Local checks passed; remote checks failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			facts := []VerificationFact{{Category: "consumer_structure", Class: VerificationLocal, Status: VerificationVerified}}
			requested := test.remote != "not_requested"
			if test.fact.Class != "" {
				test.fact.Category = "remote_workflow_identity"
				facts = append(facts, test.fact)
			}
			verification := projectPipelineVerification(VerificationSnapshot{
				RemoteStatus: test.remote, RemoteRequested: requested, RemoteAttempted: requested, Facts: facts,
			})
			if got := humanPipelineVerification(verification); got != test.want {
				t.Fatalf("verification summary = %q, want %q", got, test.want)
			}
			if requested && test.fact.Status != VerificationFailed {
				remote := humanRemoteVerificationSummary(verification)
				if !strings.Contains(strings.ToLower(remote), strings.ToLower(humanVerificationStatus(test.fact.Status))) {
					t.Fatalf("remote summary did not distinguish %q: %q", test.fact.Status, remote)
				}
			}
		})
	}
}

func TestPipelinePresentationSeparatesConfiguredStagesAndLimitations(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Stages = pipelineCharacterizationStages(22)
	plain := ansi.Strip(renderPipelineForTest(t, mapPipelineResult(result), pipelineTestWidth{width: 120, available: true}, false))
	configuredAt := strings.Index(plain, "Configured Pipeline")
	limitationsAt := strings.Index(plain, "Limitations")
	if configuredAt < 0 || limitationsAt <= configuredAt {
		t.Fatalf("stage and limitation sections are not ordered:\n%s", plain)
	}
	stageSection := plain[configuredAt:limitationsAt]
	limitationSection := plain[limitationsAt:]
	if strings.Contains(limitationSection, "Configured stage") || strings.Contains(plain, "Runtime and Limitations") {
		t.Fatalf("stages leaked into limitations:\n%s", plain)
	}
	if strings.Count(stageSection, "runtime stages have not been observed") != 1 || strings.Contains(stageSection, "not_observed") {
		t.Fatalf("untouched runtime was not compact:\n%s", stageSection)
	}
	for _, limitation := range []string{
		"Remote Git freshness was not inspected.",
		"Workflow execution and publication were not inspected remotely.",
		"This command is read-only and does not resume, retry, repair, or clean releases.",
	} {
		if strings.Count(limitationSection, limitation) != 1 {
			t.Errorf("limitation %q count changed:\n%s", limitation, limitationSection)
		}
	}
}

func presentationPropertyValue(t *testing.T, properties *presentation.Properties, label string) string {
	t.Helper()
	for _, property := range properties.Properties {
		if property.Label == label {
			return fmt.Sprint(property.Value)
		}
	}
	t.Fatalf("presentation property %q is missing: %#v", label, properties)
	return ""
}

func assertPresentationColumns(t *testing.T, got, want []presentation.Column) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation columns = %#v, want %#v", got, want)
	}
}
