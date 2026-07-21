package pipelineinspection

import "testing"

func TestPipelineRuntimeStatusProjectionUsesExplicitSafetyOrder(t *testing.T) {
	//nolint:govet // Table fields follow the scenario description order.
	tests := []struct {
		name     string
		result   pipelineResult
		selected *RuntimeExecutionObservation
		want     pipelineStatus
		manual   bool
	}{
		{name: "invalid", result: pipelineResult{InvalidEvidence: true, Dispatch: pipelineDispatch{State: "rejected"}}, want: pipelineInvalid, manual: true},
		{name: "rejected", result: pipelineResult{Dispatch: pipelineDispatch{State: "rejected"}}, selected: runtimeSelected(false), want: pipelineRejected, manual: true},
		{name: "uncertain dispatch", result: pipelineResult{Dispatch: pipelineDispatch{State: "unknown"}}, selected: runtimeSelected(false), want: pipelineUncertain, manual: true},
		{name: "uncertain recovery", selected: &RuntimeExecutionObservation{Unresolved: true, Recovery: RuntimeRecoveryObservation{Uncertain: true}}, want: pipelineUncertain, manual: true},
		{name: "blocked", selected: &RuntimeExecutionObservation{Unresolved: true, Recovery: RuntimeRecoveryObservation{ManualIntervention: true}}, want: pipelineBlocked, manual: true},
		{name: "resumable", selected: &RuntimeExecutionObservation{Unresolved: true, Recovery: RuntimeRecoveryObservation{ResumeEligible: true}}, want: pipelineResumable},
		{name: "active", selected: runtimeSelected(true), want: pipelineActive},
		{name: "completed", result: pipelineResult{Dispatch: pipelineDispatch{State: "accepted"}}, selected: runtimeSelected(false), want: pipelineCompleted},
		{name: "ready", want: pipelineReady},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.result
			result.Recovery.Reasons = make([]string, 0)
			finalizePipelineRuntimeStatus(&result, test.selected)
			if result.Status != test.want || result.ManualIntervention.Required != test.manual {
				t.Fatalf("status/manual = %s/%t, want %s/%t", result.Status, result.ManualIntervention.Required, test.want, test.manual)
			}
		})
	}
}

func runtimeSelected(unresolved bool) *RuntimeExecutionObservation {
	return &RuntimeExecutionObservation{Unresolved: unresolved}
}
