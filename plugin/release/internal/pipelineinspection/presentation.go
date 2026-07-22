package pipelineinspection

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

const (
	pipelineLocalPreparationGroup = "Local release preparation"
	pipelineHandoffGroup          = "Git and provider handoff"
	pipelineConsumerGroup         = "Consumer workflow"
	pipelinePluginRegistryGroup   = "Plugin registry"
)

var pipelineVerificationColumns = []presentation.Column{
	{Key: "check", Label: "Check", Essential: true},
	{Key: "status", Label: "Status", RoleKey: "status_role", Essential: true},
	{Key: "scope", Label: "Scope", Essential: true},
	{Key: "subject", Label: "Subject"},
	{Key: "evidence", Label: "Evidence"},
}

var pipelineStageColumns = []presentation.Column{
	{Key: "number", Label: "#"},
	{Key: "stage", Label: "Stage", Essential: true},
	{Key: "runtime", Label: "Runtime", RoleKey: "runtime_role", Essential: true},
	{Key: "owner", Label: "Owner", Essential: true},
	{Key: "location", Label: "Location"},
	{Key: "mutation", Label: "Mutation"},
	{Key: "evidence", Label: "Evidence"},
}

func pipelineHumanPresentation(result *pipelineResult) (*presentation.Properties, *presentation.Table) {
	stages := pipelineStagePresentation(result)
	return pipelineSummaryProperties(result), &presentation.Table{
		Title: "Verification Facts", Columns: append([]presentation.Column(nil), pipelineVerificationColumns...),
		Rows: pipelineVerificationRows(result.Verification.Facts), Following: stages,
	}
}

func pipelineSummaryProperties(result *pipelineResult) *presentation.Properties {
	properties := []presentation.Property{
		{Label: "Unit", Value: result.Unit.ID, Emphasized: true},
		{Label: "Version", Value: result.Unit.ConfiguredVersion},
		{Label: "Lifecycle", Value: humanPipelineLifecycle(result.Status), Role: pipelineStatusRole(result.Status)},
		{Label: "Executor", Value: humanPipelineExecutor(result.Unit.Executor)},
		{Label: "Delivery", Value: humanPipelineDelivery(result.Unit.Delivery)},
		{Label: "Workflow", Value: result.Workflow.Path},
		{Label: "Execution", Value: humanPipelineExecution(result)},
		{Label: "Dispatch", Value: humanPipelineDispatch(result)},
		{Label: "Recovery", Value: humanPipelineRecovery(result)},
		{Label: "Resume", Value: humanPipelineResume(result)},
		{Label: "Local Git", Value: humanPipelineLocalGit(result)},
		{Label: "Remote Git", Value: humanPipelineRemoteGit(result.LocalGit)},
		{
			Label: "Verification", Value: humanPipelineVerification(result.Verification),
			Role: pipelineHumanVerificationRole(result.Verification),
		},
		{Label: "Local verification", Value: humanVerificationClassSummary(result.Verification.Facts, VerificationLocal)},
		{Label: "Remote verification", Value: humanRemoteVerificationSummary(result.Verification)},
	}
	if hasVerificationClass(result.Verification.Facts, VerificationRuntimeRequired) {
		properties = append(properties, presentation.Property{
			Label: "Runtime checks", Value: humanDeferredVerificationSummary(result.Verification.Facts, VerificationRuntimeRequired),
		})
	}
	if hasVerificationClass(result.Verification.Facts, VerificationMutationRequired) {
		properties = append(properties, presentation.Property{
			Label: "Mutation-time checks", Value: humanDeferredVerificationSummary(result.Verification.Facts, VerificationMutationRequired),
		})
	}
	if result.ManualIntervention.Required {
		properties = append(properties, presentation.Property{
			Label: "Manual intervention", Value: "Required", Role: presentation.StyleWarning,
		})
	}
	return &presentation.Properties{
		Title: "Release Pipeline Inspection", SectionTitle: "Summary", Properties: properties,
	}
}

func pipelineStagePresentation(result *pipelineResult) *presentation.Table {
	table := &presentation.Table{
		Title: "Configured Pipeline", Columns: append([]presentation.Column(nil), pipelineStageColumns...),
		Rows: pipelineStageRows(result.Stages), GroupKey: "group",
		Details: pipelineLimitationPresentation(result.Limitations),
	}
	if allPipelineStagesUnobserved(result.Stages) {
		switch {
		case !result.ProgressInspection.JournalsInspected:
			table.Note = "Execution journals were not inspected; runtime stages have not been observed."
		case result.Execution.JournalCount == 0:
			table.Note = "No execution journal was found; runtime stages have not been observed."
		default:
			table.Note = "No valid execution journal was selected; runtime stages have not been observed."
		}
	}
	return table
}

func pipelineVerificationRows(facts []VerificationFact) []map[string]any {
	rows := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		rows = append(rows, map[string]any{
			"check":       humanVerificationCategory(fact.Category),
			"status":      humanVerificationStatus(fact.Status),
			"status_role": string(pipelineVerificationStatusRole(fact.Status)),
			"scope":       humanVerificationClass(fact.Class),
			"subject":     fact.Subject,
			"evidence":    fact.Evidence,
		})
	}
	return rows
}

func pipelineStageRows(stages []LifecycleStage) []map[string]any {
	rows := make([]map[string]any, 0, len(stages))
	for index, stage := range stages {
		rows = append(rows, map[string]any{
			"group":        pipelineStageGroup(stage),
			"number":       index + 1,
			"stage":        stage.Label,
			"runtime":      humanPipelineRuntime(stage.RuntimeStatus),
			"runtime_role": string(pipelineRuntimeStatusRole(stage.RuntimeStatus)),
			"owner":        humanPipelineOwner(stage.Owner),
			"location":     humanPipelineLocation(stage.Location),
			"mutation":     humanPipelineMutation(stage.Mutation),
			"evidence":     humanPipelineStageEvidence(stage),
		})
	}
	return rows
}

func pipelineStageGroup(stage LifecycleStage) string {
	switch {
	case stage.ID == "plugin-index-generation" || stage.ID == "plugin-index-publication":
		return pipelinePluginRegistryGroup
	case stage.Owner == StageOwnerConsumerWorkflow || stage.Owner == StageOwnerReleaseTool:
		return pipelineConsumerGroup
	case stage.Location == StageLocationRemoteGit || stage.Location == StageLocationGitHubAPI ||
		stage.Mutation == MutationRemoteGit || stage.Mutation == MutationRemoteAPI ||
		stage.ID == "handoff-confirmation":
		return pipelineHandoffGroup
	default:
		return pipelineLocalPreparationGroup
	}
}

func pipelineLimitationPresentation(limitations []string) *presentation.Properties {
	properties := make([]presentation.Property, 0, len(limitations))
	seen := make(map[string]struct{}, len(limitations))
	for _, limitation := range limitations {
		value := humanPipelineLimitation(limitation)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		properties = append(properties, presentation.Property{
			Label: strconv.Itoa(len(properties) + 1), Value: value, Role: presentation.StyleMuted,
		})
	}
	if len(properties) == 0 {
		return nil
	}
	return &presentation.Properties{Title: "Limitations", Properties: properties}
}

func humanPipelineLimitation(value string) string {
	switch {
	case strings.Contains(value, "remote Git freshness"):
		return "Remote Git freshness was not inspected."
	case strings.Contains(value, "Workflow execution and publication") ||
		strings.Contains(value, "Remote workflow and publication"):
		return "Workflow execution and publication were not inspected remotely."
	case strings.Contains(value, "read-only"):
		return "This command is read-only and does not resume, retry, repair, or clean releases."
	case strings.Contains(value, "Execution journals"):
		return "Execution journals and runtime progress were not inspected."
	default:
		return value
	}
}

func humanPipelineLifecycle(status pipelineStatus) string {
	switch status {
	case pipelineReady:
		return "Ready"
	case pipelineActive:
		return "Incomplete execution"
	case pipelineResumable:
		return "Resumable"
	case pipelineCompleted:
		return "Handoff completed"
	case pipelineBlocked:
		return "Blocked"
	case pipelineUncertain:
		return "Uncertain"
	case pipelineRejected:
		return "Dispatch rejected"
	case pipelineInvalid:
		return "Invalid evidence"
	default:
		return humanMachineValue(string(status))
	}
}

func humanPipelineExecutor(value string) string {
	switch value {
	case "goreleaser":
		return "GoReleaser"
	case "jreleaser":
		return "JReleaser"
	case "release-it":
		return "release-it"
	default:
		return humanMachineValue(value)
	}
}

func humanPipelineDelivery(value string) string {
	if value == "github-actions" {
		return "GitHub Actions"
	}
	return humanMachineValue(value)
}

func humanPipelineExecution(result *pipelineResult) string {
	if !result.Execution.Present {
		if result.InvalidEvidence {
			return "No valid execution selected"
		}
		return "No active execution"
	}
	return humanMachineValue(result.Execution.State)
}

func humanPipelineDispatch(result *pipelineResult) string {
	if !result.Dispatch.Present {
		if result.InvalidEvidence && result.Dispatch.JournalCount > 0 {
			return "No valid dispatch evidence"
		}
		return "No dispatch evidence"
	}
	return humanMachineValue(result.Dispatch.State)
}

func humanPipelineRecovery(result *pipelineResult) string {
	if !result.Execution.Present {
		if result.InvalidEvidence {
			return "Unavailable for invalid evidence"
		}
		return "Not applicable"
	}
	if !result.Recovery.Evaluated {
		return "Not evaluated"
	}
	return humanMachineValue(result.Recovery.Classification)
}

func humanPipelineResume(result *pipelineResult) string {
	if !result.Execution.Present {
		if result.InvalidEvidence {
			return "Unavailable for invalid evidence"
		}
		return "Not applicable"
	}
	if !result.Recovery.Evaluated {
		return "Not evaluated"
	}
	if result.Recovery.ResumeEligible {
		return "Eligible"
	}
	return "Not eligible"
}

func humanPipelineLocalGit(result *pipelineResult) string {
	observation := result.LocalGit
	if result.InvalidEvidence {
		return "Local evidence needs review"
	}
	if observation.ExpectedCommit != "" && !observation.Consistent {
		return "Inconsistent local evidence"
	}
	if observation.Scope == "local_only" {
		return "Inspected locally"
	}
	return humanMachineValue(observation.Scope)
}

func humanPipelineRemoteGit(observation pipelineLocalGit) string {
	if observation.RemoteFreshness == "remote_not_inspected" || observation.RemoteFreshness == "" {
		return "Not inspected"
	}
	return humanMachineValue(observation.RemoteFreshness)
}

func humanPipelineVerification(verification pipelineVerification) string {
	local := verificationFactsForClass(verification.Facts, VerificationLocal)
	switch verificationSummaryStatusForFacts(local) {
	case verificationSummaryFailed:
		return "Local checks failed"
	case verificationSummaryPartial, verificationSummaryUnresolved:
		return "Local checks need review"
	case verificationSummaryNotChecked:
		return "Local checks not performed"
	}
	if !verification.Summary.RemoteRequested {
		return "Local checks passed; remote checks not requested"
	}
	remote := verificationFactsForClass(verification.Facts, VerificationRemote)
	switch verificationSummaryStatusForFacts(remote) {
	case verificationSummaryFailed:
		return "Local checks passed; remote checks failed"
	case verificationSummaryPartial, verificationSummaryUnresolved:
		return "Local checks passed; remote checks need review"
	case verificationSummaryNotChecked:
		return "Local checks passed; remote checks not completed"
	}
	switch verification.Summary.RemoteStatus {
	case "complete":
		return "Local and remote checks completed"
	case "partial":
		return "Local checks passed; remote verification partial"
	case "unavailable":
		return "Local checks passed; remote verification unavailable"
	default:
		return "Local checks passed; remote verification " + strings.ToLower(humanMachineValue(verification.Summary.RemoteStatus))
	}
}

func pipelineHumanVerificationRole(verification pipelineVerification) presentation.StyleRole {
	local := verificationFactsForClass(verification.Facts, VerificationLocal)
	localStatus := verificationSummaryStatusForFacts(local)
	if localStatus == verificationSummaryFailed {
		return presentation.StyleError
	}
	if localStatus != verificationSummaryVerified {
		return presentation.StyleWarning
	}
	if !verification.Summary.RemoteRequested {
		return presentation.StyleSuccess
	}
	remoteStatus := verificationSummaryStatusForFacts(verificationFactsForClass(verification.Facts, VerificationRemote))
	switch remoteStatus {
	case verificationSummaryFailed:
		return presentation.StyleError
	case verificationSummaryVerified:
		return presentation.StyleSuccess
	default:
		return presentation.StyleWarning
	}
}

func humanVerificationClassSummary(facts []VerificationFact, class VerificationClass) string {
	selected := verificationFactsForClass(facts, class)
	if len(selected) == 0 {
		return "No checks"
	}
	return humanVerificationCounts(selected)
}

func humanRemoteVerificationSummary(verification pipelineVerification) string {
	if !verification.Summary.RemoteRequested {
		return "Not requested"
	}
	status := humanMachineValue(verification.Summary.RemoteStatus)
	facts := verificationFactsForClass(verification.Facts, VerificationRemote)
	if len(facts) == 0 {
		if verification.Summary.RemoteAttempted {
			return status
		}
		return "Not attempted"
	}
	return status + " — " + humanVerificationCounts(facts)
}

func humanDeferredVerificationSummary(facts []VerificationFact, class VerificationClass) string {
	selected := verificationFactsForClass(facts, class)
	if len(selected) == 0 {
		return "Not applicable"
	}
	for _, fact := range selected {
		if fact.Status != VerificationNotChecked && fact.Status != VerificationUnresolved {
			return humanVerificationCounts(selected)
		}
	}
	if class == VerificationRuntimeRequired {
		return "Not observed"
	}
	return "Not performed"
}

func humanVerificationCounts(facts []VerificationFact) string {
	order := []VerificationStatus{
		VerificationVerified, VerificationFailed, VerificationUnavailable, VerificationUnauthorized,
		VerificationRateLimited, VerificationUnresolved, VerificationNotChecked,
	}
	counts := make(map[VerificationStatus]int, len(order))
	for _, fact := range facts {
		counts[fact.Status]++
	}
	parts := make([]string, 0, len(order))
	for _, status := range order {
		if counts[status] == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d %s", counts[status], strings.ToLower(humanVerificationStatus(status))))
	}
	return strings.Join(parts, ", ")
}

func verificationFactsForClass(facts []VerificationFact, class VerificationClass) []VerificationFact {
	selected := make([]VerificationFact, 0)
	for _, fact := range facts {
		if fact.Class == class {
			selected = append(selected, fact)
		}
	}
	return selected
}

func hasVerificationClass(facts []VerificationFact, class VerificationClass) bool {
	for _, fact := range facts {
		if fact.Class == class {
			return true
		}
	}
	return false
}

func verificationSummaryStatusForFacts(facts []VerificationFact) verificationSummaryStatus {
	counts := pipelineVerificationCounts{}
	for _, fact := range facts {
		counts.add(fact.Status)
	}
	return pipelineVerificationSummaryStatus(counts.verified, counts.unresolved, counts.failed, counts.notChecked)
}

func humanVerificationCategory(category string) string {
	switch category {
	case "consumer_structure":
		return "Consumer workflow"
	case "credential_wiring":
		return "Credential wiring"
	case "dispatch_authorization":
		return "Dispatch authorization"
	case "goreleaser_configuration":
		return "GoReleaser configuration"
	case "installation_wiring":
		return "Installation wiring"
	case "publication_identity":
		return "Publication identity"
	case "remote_workflow_identity":
		return "Remote workflow identity"
	case "repository_variable_values":
		return "Repository variables"
	default:
		return humanMachineValue(category)
	}
}

func humanVerificationClass(class VerificationClass) string {
	switch class {
	case VerificationLocal:
		return "Local"
	case VerificationRemote:
		return "Remote"
	case VerificationRuntimeRequired:
		return "Runtime required"
	case VerificationMutationRequired:
		return "Mutation required"
	default:
		return humanMachineValue(string(class))
	}
}

func humanVerificationStatus(status VerificationStatus) string {
	switch status {
	case VerificationVerified:
		return "Verified"
	case VerificationFailed:
		return "Failed"
	case VerificationUnavailable:
		return "Unavailable"
	case VerificationUnauthorized:
		return "Unauthorized"
	case VerificationRateLimited:
		return "Rate limited"
	case VerificationNotChecked:
		return "Not checked"
	case VerificationUnresolved:
		return "Unresolved"
	default:
		return humanMachineValue(string(status))
	}
}

func humanPipelineRuntime(status RuntimeStatus) string {
	switch status {
	case RuntimeNotObserved:
		return "—"
	case RuntimeNotStarted:
		return "Not started"
	case RuntimePending:
		return "Pending"
	case RuntimeConfirmed:
		return "Confirmed"
	case RuntimeRejected:
		return "Rejected"
	case RuntimeUnknown:
		return "Unknown"
	case RuntimeBlocked:
		return "Blocked"
	case RuntimeInvalid:
		return "Invalid"
	default:
		return humanMachineValue(string(status))
	}
}

func pipelineRuntimeStatusRole(status RuntimeStatus) presentation.StyleRole {
	switch status {
	case RuntimeConfirmed:
		return presentation.StyleSuccess
	case RuntimePending, RuntimeNotStarted, RuntimeUnknown, RuntimeBlocked:
		return presentation.StyleWarning
	case RuntimeRejected, RuntimeInvalid:
		return presentation.StyleError
	case RuntimeNotObserved:
		return presentation.StyleMuted
	default:
		return presentation.StyleDefault
	}
}

func humanPipelineOwner(owner StageOwner) string {
	switch owner {
	case StageOwnerNekoCLI:
		return "Neko CLI"
	case StageOwnerLocalGit:
		return "Local Git"
	case StageOwnerRemoteGit:
		return "Remote Git"
	case StageOwnerGitHubAPI:
		return "GitHub API"
	case StageOwnerConsumerWorkflow:
		return "Consumer workflow"
	case StageOwnerReleaseTool:
		return "Release tool"
	default:
		return humanMachineValue(string(owner))
	}
}

func humanPipelineLocation(location StageLocation) string {
	switch location {
	case StageLocationLocalProcess:
		return "Local process"
	case StageLocationLocalRepository:
		return "Local repository"
	case StageLocationLocalGit:
		return "Local Git"
	case StageLocationRemoteGit:
		return "Remote Git"
	case StageLocationGitHubAPI:
		return "GitHub API"
	case StageLocationGitHubActionsRunner:
		return "GitHub Actions runner"
	default:
		return humanMachineValue(string(location))
	}
}

func humanPipelineMutation(mutation MutationClass) string {
	switch mutation {
	case MutationNone:
		return "None"
	case MutationFilesystem:
		return "Filesystem"
	case MutationReleaseState:
		return "Release state"
	case MutationGitIndex:
		return "Git index"
	case MutationGitObject:
		return "Git object"
	case MutationGitRef:
		return "Git ref"
	case MutationRemoteGit:
		return "Remote Git"
	case MutationRemoteAPI:
		return "Remote API"
	case MutationPublication:
		return "Publication"
	default:
		return humanMachineValue(string(mutation))
	}
}

func humanPipelineStageEvidence(stage LifecycleStage) string {
	var evidence string
	switch stage.RuntimeEvidence {
	case "execution_journal":
		evidence = "Execution journal"
	case "dispatch_journal":
		evidence = "Dispatch journal"
	case "local_git":
		evidence = "Local Git"
	case "resume_policy":
		evidence = "Resume policy"
	case "":
		return "—"
	default:
		evidence = humanMachineValue(stage.RuntimeEvidence)
	}
	if stage.RuntimeReason != "" {
		return evidence + " — " + humanPipelineReason(stage.RuntimeReason)
	}
	return evidence
}

func humanPipelineReason(reason string) string {
	if !strings.ContainsAny(reason, " \t") && strings.ContainsAny(reason, "_-") {
		return humanMachineValue(reason)
	}
	return reason
}

func allPipelineStagesUnobserved(stages []LifecycleStage) bool {
	if len(stages) == 0 {
		return false
	}
	for _, stage := range stages {
		if stage.RuntimeStatus != RuntimeNotObserved {
			return false
		}
	}
	return true
}

func humanMachineValue(value string) string {
	value = strings.TrimSpace(strings.NewReplacer("_", " ", "-", " ").Replace(value))
	if value == "" {
		return "Not available"
	}
	runes := []rune(strings.ToLower(value))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
