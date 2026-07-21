package pipelineinspection

import (
	"strconv"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

var pipelineStageColumns = []presentation.Column{
	{Key: "label", Label: "Stage", Essential: true},
	{Key: "runtime_status", Label: "Runtime", Essential: true},
	{Key: "owner", Label: "Owner", Essential: true},
	{Key: "configuration_status", Label: "Configured"},
	{Key: "location", Label: "Location"},
	{Key: "mutation", Label: "Mutation"},
	{Key: "source", Label: "Source"},
}

func mapPipelineResult(result *pipelineResult) *plugin.Response {
	result = normalizePipelineArrays(result)
	rows := make([]map[string]any, 0, len(result.Stages))
	for _, stage := range result.Stages {
		row := map[string]any{
			"id": stage.ID, "label": stage.Label,
			"runtime_status":       stage.RuntimeStatus,
			"configuration_status": stage.ConfigurationStatus,
			"owner":                stage.Owner, "location": stage.Location,
			"mutation": stage.Mutation, "source": stage.Source,
		}
		if stage.ConditionalReason != "" {
			row["conditional_reason"] = stage.ConditionalReason
		}
		if stage.RuntimeEvidence != "" {
			row["runtime_evidence"] = stage.RuntimeEvidence
		}
		if stage.RuntimeReason != "" {
			row["runtime_reason"] = stage.RuntimeReason
		}
		if stage.RuntimeIdentity != "" {
			row["runtime_identity"] = stage.RuntimeIdentity
		}
		if stage.RuntimeConfirmedAt != "" {
			row["runtime_confirmed_at"] = stage.RuntimeConfirmedAt
		}
		rows = append(rows, row)
	}
	details := make([]presentation.Property, 0, len(result.Limitations)+6)
	if result.Execution.Present {
		details = append(details, presentation.Property{Label: "Execution Evidence", Value: result.Execution.Identity + " (" + result.Execution.State + ")"})
	}
	if result.Dispatch.Present {
		details = append(details, presentation.Property{Label: "Dispatch Evidence", Value: result.Dispatch.Identity + " (" + result.Dispatch.State + ")"})
	}
	if result.Recovery.Guidance != "" {
		details = append(details, presentation.Property{Label: "Recovery Guidance", Value: result.Recovery.Guidance})
	}
	for index, reason := range result.ManualIntervention.Reasons {
		details = append(details, presentation.Property{Label: "Manual Reason " + strconv.Itoa(index+1), Value: reason})
	}
	for index, limitation := range result.Limitations {
		details = append(details, presentation.Property{Label: limitationLabel(index), Value: limitation, Role: presentation.StyleMuted})
	}
	response := &plugin.Response{
		Status:   "success",
		Metadata: pipelineResponseMetadata(),
		Data: map[string]any{
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
			"limitations":         append(make([]string, 0, len(result.Limitations)), result.Limitations...),
		},
		RendererHint: "table",
		PresentationProperties: &presentation.Properties{
			Title: "Release Pipeline Inspection",
			Properties: []presentation.Property{
				{Label: "Unit", Value: result.Unit.ID, Emphasized: true},
				{Label: "Version", Value: result.Unit.ConfiguredVersion},
				{Label: "Status", Value: result.Status, Role: pipelineStatusRole(result.Status)},
				{Label: "Executor", Value: result.Unit.Executor},
				{Label: "Delivery", Value: result.Unit.Delivery},
				{Label: "Workflow", Value: result.Workflow.Path},
				{Label: "Execution", Value: pipelineDisplayValue(result.Execution.State, "none")},
				{Label: "Dispatch", Value: pipelineDisplayValue(result.Dispatch.State, "none")},
				{Label: "Local Git", Value: pipelineLocalGitSummary(result.LocalGit)},
				{Label: "Recovery", Value: pipelineDisplayValue(result.Recovery.Classification, "not_evaluated")},
				{Label: "Resume Eligible", Value: strconv.FormatBool(result.Recovery.ResumeEligible)},
				{Label: "Manual Intervention", Value: strconv.FormatBool(result.ManualIntervention.Required)},
			},
		},
		PresentationTable: &presentation.Table{
			Title: "Stages", Columns: append([]presentation.Column(nil), pipelineStageColumns...),
			Rows:    rows,
			Details: &presentation.Properties{Title: "Runtime and Limitations", Properties: details},
		},
	}
	if result.InvalidEvidence {
		response.ExitCode = 1
	}
	return response
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
	return result
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

func limitationLabel(index int) string {
	return "Limitation " + strconv.Itoa(index+1)
}

func pipelineDisplayValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func pipelineLocalGitSummary(observation pipelineLocalGit) string {
	if observation.ExpectedCommit == "" {
		return observation.Scope
	}
	if observation.Consistent {
		return "consistent (local only)"
	}
	return "inconsistent (local only)"
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
