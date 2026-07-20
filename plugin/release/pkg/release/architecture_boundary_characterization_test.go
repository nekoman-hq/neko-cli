package release

import (
	"reflect"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestReleaseExecutionJournalTransitionGraphContract(t *testing.T) {
	wantStates := []ReleaseExecutionJournalState{
		ReleaseExecutionPrepared,
		ReleaseExecutionPreflightValidated,
		ReleaseExecutionMaterializationApplied,
		ReleaseExecutionStateWritten,
		ReleaseExecutionReleaseFilesStaged,
		ReleaseExecutionCommitCreated,
		ReleaseExecutionTagCreated,
		ReleaseExecutionDispatchJournalPrepared,
		ReleaseExecutionCommitPushed,
		ReleaseExecutionTagPushed,
		ReleaseExecutionHandoffReady,
	}
	if !reflect.DeepEqual(releaseExecutionStateOrder, wantStates) {
		t.Fatalf("execution state order = %v, want %v", releaseExecutionStateOrder, wantStates)
	}

	wantPending := map[ReleaseExecutionJournalState]ReleaseExecutionPendingAction{
		ReleaseExecutionPrepared:                ReleaseExecutionPendingNone,
		ReleaseExecutionPreflightValidated:      ReleaseExecutionPendingNone,
		ReleaseExecutionMaterializationApplied:  ReleaseExecutionPendingApplyMaterialization,
		ReleaseExecutionStateWritten:            ReleaseExecutionPendingWriteState,
		ReleaseExecutionReleaseFilesStaged:      ReleaseExecutionPendingStageReleaseFiles,
		ReleaseExecutionCommitCreated:           ReleaseExecutionPendingCreateReleaseCommit,
		ReleaseExecutionTagCreated:              ReleaseExecutionPendingCreateUnitTag,
		ReleaseExecutionDispatchJournalPrepared: ReleaseExecutionPendingCreateDispatchJournal,
		ReleaseExecutionCommitPushed:            ReleaseExecutionPendingPushReleaseCommit,
		ReleaseExecutionTagPushed:               ReleaseExecutionPendingPushUnitTag,
		ReleaseExecutionHandoffReady:            ReleaseExecutionPendingNone,
	}
	for index, state := range wantStates {
		if got := pendingActionForConfirmedPhase(state); got != wantPending[state] {
			t.Errorf("pending action for %s = %s, want %s", state, got, wantPending[state])
		}
		for _, next := range wantStates {
			want := index+1 < len(wantStates) && next == wantStates[index+1]
			if got := state.CanTransitionTo(next); got != want {
				t.Errorf("CanTransitionTo(%s, %s) = %t, want %t", state, next, got, want)
			}
		}
	}
}

func TestDispatchJournalTransitionGraphContract(t *testing.T) {
	states := []DispatchJournalState{
		DispatchJournalPrepared,
		DispatchJournalRequestStarted,
		DispatchJournalAccepted,
		DispatchJournalRejected,
		DispatchJournalUnknown,
	}
	allowed := map[DispatchJournalState]map[DispatchJournalState]bool{
		DispatchJournalPrepared: {
			DispatchJournalRequestStarted: true,
		},
		DispatchJournalRequestStarted: {
			DispatchJournalAccepted: true,
			DispatchJournalRejected: true,
			DispatchJournalUnknown:  true,
		},
	}
	for _, state := range states {
		if !state.Valid() {
			t.Errorf("state %q is unexpectedly invalid", state)
		}
		for _, next := range states {
			want := allowed[state][next]
			if got := state.CanTransitionTo(next); got != want {
				t.Errorf("CanTransitionTo(%s, %s) = %t, want %t", state, next, got, want)
			}
		}
	}
	for _, terminal := range []DispatchJournalState{
		DispatchJournalAccepted,
		DispatchJournalRejected,
		DispatchJournalUnknown,
	} {
		for _, next := range states {
			if terminal.CanTransitionTo(next) {
				t.Errorf("terminal dispatch state %s transitions to %s", terminal, next)
			}
		}
	}
}

func TestReleaseToolIdentityAndConfigurationCandidateContract(t *testing.T) {
	tests := []struct {
		identity   string
		v1         releaseconfig.V1ReleaseSystem
		v2         releaseconfig.ExecutorType
		candidates []string
	}{
		{
			identity:   "goreleaser",
			v1:         releaseconfig.V1ReleaseTypeGoReleaser,
			v2:         releaseconfig.ExecutorGoReleaser,
			candidates: []string{".goreleaser.yml", ".goreleaser.yaml"},
		},
		{
			identity:   "jreleaser",
			v1:         releaseconfig.V1ReleaseTypeJReleaser,
			v2:         releaseconfig.ExecutorJReleaser,
			candidates: []string{"jreleaser.yml"},
		},
		{
			identity:   "release-it",
			v1:         releaseconfig.V1ReleaseTypeReleaseIt,
			v2:         releaseconfig.ExecutorReleaseIt,
			candidates: []string{".release-it.json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.identity, func(t *testing.T) {
			if string(tt.v1) != tt.identity || string(tt.v2) != tt.identity {
				t.Fatalf("schema identities differ: v1=%q v2=%q want=%q", tt.v1, tt.v2, tt.identity)
			}
			if !tt.v1.IsValid() || !tt.v2.IsValid() {
				t.Fatalf("tool identity %q is not valid in both schemas", tt.identity)
			}
			candidates, err := requiredReleaseSystemFiles(tt.identity)
			if err != nil {
				t.Fatalf("requiredReleaseSystemFiles(%q): %v", tt.identity, err)
			}
			if !reflect.DeepEqual(candidates, tt.candidates) {
				t.Fatalf("config candidates = %v, want %v", candidates, tt.candidates)
			}
			capabilities, err := ResolveExecutorCapabilities(tt.identity)
			if err != nil {
				t.Fatalf("ResolveExecutorCapabilities(%q): %v", tt.identity, err)
			}
			if capabilities.Type != tt.identity {
				t.Fatalf("capability identity = %q, want %q", capabilities.Type, tt.identity)
			}
		})
	}

	if _, err := requiredReleaseSystemFiles("semantic-release"); err == nil || !strings.Contains(err.Error(), "unknown release system") {
		t.Fatalf("unknown tool candidate error = %v", err)
	}
}

func TestActiveV2CoordinatorRemainsDirectAndCompatibilityTransactionStaysQuarantined(t *testing.T) {
	coordinator := readCommandBoundarySource(t, "github_actions_release_use_case.go")
	for _, required := range []string{
		"tokenResolver.ResolveGitHubActionsDispatchToken(ctx)",
		"planner.Plan(execCtx)",
		"preflightValidator.Validate(execCtx, planned.KnownFiles)",
		"executionPreparer.Prepare(execCtx, planned.KnownFiles, preflight)",
		"materialization.Apply(execution, planned.MaterializationPlan)",
		"stateWriter.Write(execCtx, execution, materialization)",
		"fileStager.Stage(execCtx, execution, planned.KnownFiles, state, materialization)",
		"commitCreator.Create(execCtx, execution, planned.KnownFiles)",
		"tagCreator.Create(execCtx, execution, commitSHA)",
		"dispatchPreparer.Prepare(execCtx, execution, preflight, planned.KnownFiles, commitSHA)",
		"commitPusher.Push(execCtx, execution, preflight, commitSHA)",
		"tagPusher.Push(execCtx, execution, preflight, commitSHA)",
		"workflowDispatcher.Dispatch(ctx, execCtx, execution, dispatch, token)",
		"handoffConfirmer.Confirm(execution)",
	} {
		if !strings.Contains(coordinator, required) {
			t.Errorf("active V2 coordinator is missing direct operation %q", required)
		}
	}
	for _, forbidden := range []string{
		"ReleaseTransaction",
		"MutationTracker",
		"ExecutionPhase",
		"for _, stage := range",
		"for _, step := range",
		"stage.Execute(",
		"step.Execute(",
	} {
		if strings.Contains(coordinator, forbidden) {
			t.Errorf("active V2 coordinator contains compatibility or pipeline behavior %q", forbidden)
		}
	}

	assertCompatibilityProductionReferences(t, "NewMutationTracker(", []string{"release_transaction.go"})
	assertCompatibilityProductionReferences(t, "NewReleaseTransaction(", []string{"release_transaction.go"})
	for _, path := range []string{"github_actions_release_use_case.go", "resume.go", "resume_operations.go"} {
		source := readCommandBoundarySource(t, path)
		if strings.Contains(source, "coordinator.Push(") {
			t.Errorf("%s uses the unjournaled compatibility push operation", path)
		}
	}
}
