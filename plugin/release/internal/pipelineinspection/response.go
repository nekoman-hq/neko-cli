package pipelineinspection

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

func mapPipelineResult(result *pipelineResult) *plugin.Response {
	result = normalizePipelineArrays(result)
	properties, table := pipelineHumanPresentation(result)
	response := &plugin.Response{
		Status:                 "success",
		Metadata:               pipelineResponseMetadata(),
		Data:                   pipelineResponseData(result),
		RendererHint:           "table",
		PresentationProperties: properties,
		PresentationTable:      table,
	}
	if result.InvalidEvidence {
		response.ExitCode = 1
	}
	return response
}

func pipelineResponseData(result *pipelineResult) map[string]any {
	return map[string]any{
		"schema_version":      result.SchemaVersion,
		"status":              result.Status,
		"unit":                result.Unit,
		"release":             result.Release,
		"repository":          result.Repository,
		"workflow":            result.Workflow,
		"stages":              append(make([]LifecycleStage, 0, len(result.Stages)), result.Stages...),
		"progress_inspection": result.ProgressInspection,
		"execution":           result.Execution,
		"dispatch":            result.Dispatch,
		"local_git":           result.LocalGit,
		"recovery":            result.Recovery,
		"manual_intervention": result.ManualIntervention,
		"verification":        result.Verification,
		"limitations":         append(make([]string, 0, len(result.Limitations)), result.Limitations...),
	}
}

func normalizePipelineArrays(result *pipelineResult) *pipelineResult {
	if result == nil {
		return result
	}
	if result.Release.MaterializedFiles == nil {
		result.Release.MaterializedFiles = make([]pipelineMaterializedFile, 0)
	}
	if result.Workflow.RequiredInputs == nil {
		result.Workflow.RequiredInputs = make([]string, 0)
	}
	if result.Workflow.ConsumerOperations == nil {
		result.Workflow.ConsumerOperations = make([]string, 0)
	}
	if result.Stages == nil {
		result.Stages = make([]LifecycleStage, 0)
	}
	for index := range result.Stages {
		if result.Stages[index].RuntimeStatus == "" {
			result.Stages[index].RuntimeStatus = RuntimeNotObserved
		}
	}
	if result.Execution.Observations == nil {
		result.Execution.Observations = make([]pipelineExecutionJournal, 0)
	}
	if result.Dispatch.Observations == nil {
		result.Dispatch.Observations = make([]pipelineDispatchJournal, 0)
	}
	if result.Recovery.Reasons == nil {
		result.Recovery.Reasons = make([]string, 0)
	}
	if result.ManualIntervention.Reasons == nil {
		result.ManualIntervention.Reasons = make([]string, 0)
	}
	if result.Limitations == nil {
		result.Limitations = make([]string, 0)
	}
	if result.Verification.Facts == nil {
		result.Verification.Facts = make([]VerificationFact, 0)
	}
	for index := range result.Verification.Facts {
		if result.Verification.Facts[index].References == nil {
			result.Verification.Facts[index].References = make([]string, 0)
		}
	}
	return result
}

func pipelineVerificationStatusRole(status VerificationStatus) presentation.StyleRole {
	switch status {
	case VerificationVerified:
		return presentation.StyleSuccess
	case VerificationFailed:
		return presentation.StyleError
	case VerificationNotChecked, VerificationUnresolved, VerificationUnavailable,
		VerificationUnauthorized, VerificationRateLimited:
		return presentation.StyleWarning
	default:
		return presentation.StyleDefault
	}
}

func pipelineVerificationSummaryRole(status verificationSummaryStatus) presentation.StyleRole {
	switch status {
	case verificationSummaryVerified:
		return presentation.StyleSuccess
	case verificationSummaryFailed:
		return presentation.StyleError
	case verificationSummaryPartial, verificationSummaryUnresolved, verificationSummaryNotChecked:
		return presentation.StyleWarning
	default:
		return presentation.StyleDefault
	}
}

func mapPipelineFailure(failure *commandFailure) *plugin.Response {
	return &plugin.Response{
		Status: "error", Metadata: pipelineResponseMetadata(),
		Error:    &plugin.ResponseError{Code: failure.Code, Message: failure.Message},
		ExitCode: 1,
	}
}

func pipelineResponseMetadata() plugin.ResponseMetadata {
	return plugin.ResponseMetadata{
		Plugin: metadata.PluginName, Version: metadata.Version,
		Command: pipelineCommandName, Timestamp: time.Now(),
	}
}

func pipelineStatusRole(status pipelineStatus) presentation.StyleRole {
	switch status {
	case pipelineReady, pipelineResumable, pipelineCompleted:
		return presentation.StyleSuccess
	case pipelineActive, pipelineBlocked, pipelineUncertain:
		return presentation.StyleWarning
	case pipelineRejected, pipelineInvalid:
		return presentation.StyleError
	default:
		return presentation.StyleDefault
	}
}
