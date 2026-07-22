package pipelineinspection

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPipelineHumanOutputCharacterizationCoversUntouchedStageCountsAndWidths(t *testing.T) {
	for _, count := range []int{22, 26} {
		t.Run(fmt.Sprintf("%d stages", count), func(t *testing.T) {
			result := pipelinePresentationFixture()
			result.Stages = pipelineCharacterizationStages(count)
			response := mapPipelineResult(result)

			normal := ansi.Strip(renderPipelineForTest(t, response, pipelineTestWidth{width: 120, available: true}, false))
			if !strings.Contains(normal, fmt.Sprintf("Configured stage %d", count)) || strings.Contains(normal, "not_observed") || strings.Count(normal, "runtime stages have not been observed") != 1 {
				t.Fatalf("normal-width untouched stage characterization changed:\n%s", normal)
			}

			narrow := ansi.Strip(renderPipelineForTest(t, response, pipelineTestWidth{width: 30, available: true}, false))
			if !strings.Contains(narrow, fmt.Sprintf("Configured stage %d", count)) || strings.Contains(narrow, "not_observed") || strings.Count(narrow, "No execution journal") != 1 {
				t.Fatalf("narrow-width untouched stage characterization changed:\n%s", narrow)
			}

			unknown := ansi.Strip(renderPipelineForTest(t, response, pipelineTestWidth{}, false))
			if !strings.Contains(unknown, fmt.Sprintf("Configured stage %d", count)) || strings.Contains(unknown, "not_observed") || strings.Count(unknown, "runtime stages have not been observed") != 1 {
				t.Fatalf("unknown-width untouched stage characterization changed:\n%s", unknown)
			}
		})
	}
}

func TestPipelineHumanOutputCharacterizationCoversRuntimeAndVerificationVocabulary(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Status = pipelineResumable
	result.Execution = pipelineExecution{Present: true, State: "tag-created", Observations: []pipelineExecutionJournal{}}
	result.Recovery = pipelineRecovery{Evaluated: true, Classification: "interrupted-after-tag", ResumeEligible: true, Reasons: []string{}}
	result.Stages[0].RuntimeStatus = RuntimePending
	result.Verification = projectPipelineVerification(VerificationSnapshot{
		RemoteStatus: "partial", RemoteRequested: true, RemoteAttempted: true,
		Facts: []VerificationFact{
			{Category: "consumer_structure", Class: VerificationLocal, Status: VerificationVerified, Subject: "workflow", Source: "doctor", Scope: "workflow"},
			{Category: "remote_workflow_identity", Class: VerificationRemote, Status: VerificationUnauthorized, Subject: "repository", Source: "doctor", Scope: "repository"},
			{Category: "dispatch_authorization", Class: VerificationMutationRequired, Status: VerificationNotChecked, Subject: "dispatch", Source: "doctor", Scope: "repository"},
		},
	})

	plain := ansi.Strip(renderPipelineForTest(t, mapPipelineResult(result), pipelineTestWidth{}, false))
	for _, value := range []string{
		"Lifecycle\n  Resumable", "Execution\n  Tag created", "Recovery\n  Interrupted after tag",
		"Consumer workflow", "Remote workflow identity", "Mutation required", "Not checked",
	} {
		if !strings.Contains(plain, value) {
			t.Fatalf("human-output characterization omitted %q:\n%s", value, plain)
		}
	}
	for _, forbidden := range []string{"consumer_structure", "remote_workflow_identity", "mutation_required", "not_checked", "Source: doctor"} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("human-output characterization exposed %q:\n%s", forbidden, plain)
		}
	}
}

func TestPipelineHumanOutputCharacterizationKeepsJSONMachineValues(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Stages = pipelineCharacterizationStages(22)
	result.Verification = projectPipelineVerification(VerificationSnapshot{
		RemoteStatus: "not_requested",
		Facts: []VerificationFact{{
			Category: "consumer_structure", Class: VerificationMutationRequired,
			Status: VerificationNotChecked, Subject: "workflow", Source: "doctor", Scope: "workflow",
		}},
	})
	encoded, err := json.Marshal(mapPipelineResult(result).Data)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, value := range []string{
		`"schema_version":1`, `"status":"ready"`, `"runtime_status":"not_observed"`,
		`"category":"consumer_structure"`, `"class":"mutation_required"`,
		`"status":"not_checked"`, `"remote_status":"not_requested"`, `"source":"doctor"`,
	} {
		if !strings.Contains(text, value) {
			t.Fatalf("machine-output characterization omitted %s: %s", value, text)
		}
	}
}

func pipelineCharacterizationStages(count int) []LifecycleStage {
	stages := make([]LifecycleStage, 0, count)
	for index := 0; index < count; index++ {
		stages = append(stages, LifecycleStage{
			ID: fmt.Sprintf("stage-%02d", index+1), Label: fmt.Sprintf("Configured stage %d", index+1),
			Owner: StageOwnerNekoCLI, Location: StageLocationLocalProcess,
			Mutation: MutationNone, ConfigurationStatus: StageConfigured,
		})
	}
	return stages
}
