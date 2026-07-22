package pipelineinspection

import (
	"strconv"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

var pipelineVerificationColumns = []presentation.Column{
	{Key: "category", Label: "Category", Essential: true},
	{Key: "status", Label: "Status", RoleKey: "status_role", Essential: true},
	{Key: "class", Label: "Class", Essential: true},
	{Key: "subject", Label: "Subject"},
	{Key: "source", Label: "Source"},
}

func mapPipelineResult(result *pipelineResult) *plugin.Response {
	result = normalizePipelineArrays(result)
	response := &plugin.Response{
		Status:                 "success",
		Metadata:               pipelineResponseMetadata(),
		Data:                   pipelineResponseData(result),
		RendererHint:           "table",
		PresentationProperties: pipelineSummaryProperties(result),
		PresentationTable: &presentation.Table{
			Title: "Verification Facts", Columns: append([]presentation.Column(nil), pipelineVerificationColumns...),
			Rows:    pipelineVerificationRows(result.Verification.Facts),
			Details: &presentation.Properties{Title: "Runtime and Limitations", Properties: pipelineResultDetails(result)},
		},
	}
	if result.InvalidEvidence {
		response.ExitCode = 1
	}
	return response
}

func pipelineVerificationRows(facts []VerificationFact) []map[string]any {
	rows := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		rows = append(rows, map[string]any{
			"id": fact.ID, "category": fact.Category, "class": fact.Class,
			"status": fact.Status, "status_role": string(pipelineVerificationStatusRole(fact.Status)),
			"subject": fact.Subject, "source": fact.Source,
		})
	}
	return rows
}

func pipelineStageRows(stages []LifecycleStage) []map[string]any {
	rows := make([]map[string]any, 0, len(stages))
	for _, stage := range stages {
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
	return rows
}

func pipelineResultDetails(result *pipelineResult) []presentation.Property {
	details := make([]presentation.Property, 0, len(result.Stages)+len(result.Limitations)+6)
	for index, stage := range result.Stages {
		details = append(details, presentation.Property{
			Label: "Stage " + strconv.Itoa(index+1),
			Value: stage.Label + " (" + string(stage.RuntimeStatus) + ", " + string(stage.Owner) + ")",
		})
	}
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
	return details
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

func pipelineSummaryProperties(result *pipelineResult) *presentation.Properties {
	return &presentation.Properties{
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
			{Label: "Verification", Value: string(result.Verification.Summary.Status), Role: pipelineVerificationSummaryRole(result.Verification.Summary.Status)},
			{Label: "Verified Facts", Value: strconv.Itoa(result.Verification.Summary.Verified)},
			{Label: "Unresolved Facts", Value: strconv.Itoa(result.Verification.Summary.Unresolved)},
			{Label: "Not Checked Facts", Value: strconv.Itoa(result.Verification.Summary.NotChecked)},
			{Label: "Remote Verification", Value: result.Verification.Summary.RemoteStatus},
		},
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
