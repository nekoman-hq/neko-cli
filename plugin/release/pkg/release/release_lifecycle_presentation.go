package release

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const legacyReleaseConfigFile = ".release.neko.json"

// releaseLifecyclePresentationFacts is a presentation-only projection of the
// established release outcome variants. It does not participate in planning,
// execution, recovery, or the machine response.
//
//nolint:govet // Fields follow the human lifecycle order.
type releaseLifecyclePresentationFacts struct {
	command                   string
	source                    string
	unit                      string
	currentVersion            string
	nextVersion               string
	tag                       string
	executor                  string
	delivery                  string
	workflow                  string
	workingDirectory          string
	unitRoot                  string
	status                    string
	handoff                   string
	nextAction                string
	commitMessage             string
	commitSHA                 string
	executionJournal          string
	dispatchJournal           string
	executionState            string
	dispatchState             string
	ownership                 string
	gitOwnership              string
	stateGuarantee            string
	materializedFiles         []string
	knownReleaseFiles         []string
	materializationBlocker    string
	dispatch                  *ReleaseDispatchDryRunSummary
	dryRun                    bool
	gitHubActionsResult       bool
	legacyCompatibilityResult bool
}

func attachReleaseLifecyclePresentation(response *plugin.Response, command string, outcome ReleaseCommandOutcome) {
	if response == nil {
		return
	}
	facts, ok := releaseLifecycleFacts(command, outcome)
	if !ok {
		return
	}
	response.PresentationProperties = &presentation.Properties{
		Title:      "Release Summary",
		Properties: releaseLifecycleSummary(facts),
	}
	response.PresentationTable = releaseLifecycleTables(facts)
}

func releaseLifecycleFacts(command string, outcome ReleaseCommandOutcome) (releaseLifecyclePresentationFacts, bool) {
	switch result := outcome.(type) {
	case *V1ReleasePreview:
		return v1ReleasePreviewPresentationFacts(command, result), true
	case *V1ReleaseCompleted:
		return v1ReleaseCompletedPresentationFacts(command, result), true
	case *V2ReleasePreview:
		return v2ReleasePreviewPresentationFacts(command, result), true
	case *GitHubActionsReleaseResult:
		return githubActionsReleasePresentationFacts(command, result), true
	default:
		return releaseLifecyclePresentationFacts{}, false
	}
}

func v1ReleasePreviewPresentationFacts(command string, result *V1ReleasePreview) releaseLifecyclePresentationFacts {
	return releaseLifecyclePresentationFacts{
		command:                   command,
		source:                    string(config.SourceFormatV1),
		unit:                      "default",
		currentVersion:            result.CurrentVersion,
		nextVersion:               result.NextVersion,
		tag:                       "v" + result.NextVersion,
		executor:                  result.ReleaseSystem,
		delivery:                  string(config.DeliveryLocal),
		workingDirectory:          ".",
		unitRoot:                  ".",
		status:                    "Preview",
		handoff:                   "Not started",
		nextAction:                "Review the preview, then run the same release command without --dry-run when ready.",
		materializedFiles:         []string{legacyReleaseConfigFile},
		knownReleaseFiles:         []string{legacyReleaseConfigFile},
		dryRun:                    true,
		legacyCompatibilityResult: true,
	}
}

func v1ReleaseCompletedPresentationFacts(command string, result *V1ReleaseCompleted) releaseLifecyclePresentationFacts {
	return releaseLifecyclePresentationFacts{
		command:                   command,
		source:                    string(config.SourceFormatV1),
		unit:                      "default",
		currentVersion:            result.PreviousVersion,
		nextVersion:               result.NextVersion,
		tag:                       "v" + result.NextVersion,
		executor:                  result.ReleaseSystem,
		delivery:                  string(config.DeliveryLocal),
		workingDirectory:          ".",
		unitRoot:                  ".",
		status:                    "Released successfully",
		handoff:                   "Completed by the configured V1 release tool",
		nextAction:                "Verify the release artifacts produced by the configured V1 release tool.",
		materializedFiles:         []string{legacyReleaseConfigFile},
		knownReleaseFiles:         []string{legacyReleaseConfigFile},
		legacyCompatibilityResult: true,
	}
}

func v2ReleasePreviewPresentationFacts(command string, result *V2ReleasePreview) releaseLifecyclePresentationFacts {
	handoff := "Not started"
	nextAction := "Review the preview, then run the same release command without --dry-run when ready."
	if result.MaterializationBlockedReason != "" {
		handoff = "Blocked before execution"
		nextAction = "Resolve the materialized-file blocker before starting the release."
	}
	return releaseLifecyclePresentationFacts{
		command:                command,
		source:                 string(config.SourceFormatV2),
		unit:                   result.UnitID,
		currentVersion:         result.CurrentVersion,
		nextVersion:            result.NextVersion,
		tag:                    result.Tag,
		executor:               result.Executor,
		delivery:               result.Delivery,
		workflow:               result.Workflow,
		workingDirectory:       result.WorkingDirectory,
		unitRoot:               result.UnitRoot,
		status:                 "Preview",
		handoff:                handoff,
		nextAction:             nextAction,
		commitMessage:          result.CommitMessage,
		ownership:              result.OwnershipSummary,
		gitOwnership:           result.V2GitOwnership,
		stateGuarantee:         result.StateGuarantee,
		materializedFiles:      append([]string(nil), result.MaterializedFilePaths...),
		knownReleaseFiles:      append([]string(nil), result.KnownReleaseFilePaths...),
		materializationBlocker: result.MaterializationBlockedReason,
		dispatch:               result.Dispatch,
		dryRun:                 true,
	}
}

func githubActionsReleasePresentationFacts(command string, result *GitHubActionsReleaseResult) releaseLifecyclePresentationFacts {
	return releaseLifecyclePresentationFacts{
		command:             command,
		source:              string(config.SourceFormatV2),
		unit:                result.Unit,
		currentVersion:      "not retained in the established outcome",
		nextVersion:         result.Version,
		tag:                 result.Tag,
		executor:            "configured V2 release tool",
		delivery:            string(config.DeliveryGitHubActions),
		workflow:            result.Workflow,
		status:              releaseLifecycleReadableValue(string(result.ExecutionState)),
		handoff:             releaseLifecycleHandoff(result),
		nextAction:          result.RecoveryGuidance,
		commitSHA:           result.CommitSHA,
		executionJournal:    result.ExecutionJournalPath,
		dispatchJournal:     result.DispatchJournalPath,
		executionState:      string(result.ExecutionState),
		dispatchState:       string(result.DispatchState),
		gitHubActionsResult: true,
	}
}

func releaseLifecycleSummary(facts releaseLifecyclePresentationFacts) []presentation.Property {
	properties := []presentation.Property{
		{Label: "Requested change", Value: releaseLifecycleReadableValue(facts.command), Emphasized: true},
		{Label: "Unit", Value: facts.unit, Emphasized: true},
		{Label: "Previous version", Value: facts.currentVersion},
		{Label: releaseLifecycleVersionLabel(facts), Value: facts.nextVersion, Emphasized: true},
		{Label: "Resulting tag", Value: facts.tag},
		{Label: "Executor", Value: facts.executor},
		{Label: "Delivery", Value: facts.delivery},
		{Label: "Lifecycle outcome", Value: facts.status, Role: releaseLifecycleOutcomeRole(facts)},
		{Label: "Handoff state", Value: facts.handoff, Role: releaseLifecycleHandoffRole(facts)},
	}
	if facts.dryRun {
		properties = append(properties,
			presentation.Property{Label: "Dry run", Value: "Yes; preview only", Role: presentation.StyleInfo},
			presentation.Property{
				Label: "Mutation boundary",
				Value: "No release tool, journal, commit, tag, push, or dispatch was started",
				Role:  presentation.StyleMuted,
			},
		)
	}
	return append(properties, presentation.Property{
		Label: "Next action", Value: facts.nextAction, Emphasized: true,
	})
}

func releaseLifecycleVersionLabel(facts releaseLifecyclePresentationFacts) string {
	if facts.dryRun {
		return "Planned version"
	}
	return "Resulting version"
}

func releaseLifecycleOutcomeRole(facts releaseLifecyclePresentationFacts) presentation.StyleRole {
	if facts.materializationBlocker != "" {
		return presentation.StyleError
	}
	if facts.dryRun {
		return presentation.StyleInfo
	}
	return presentation.StyleSuccess
}

func releaseLifecycleHandoffRole(facts releaseLifecyclePresentationFacts) presentation.StyleRole {
	if facts.materializationBlocker != "" || strings.Contains(strings.ToLower(facts.handoff), "rejected") {
		return presentation.StyleError
	}
	if facts.dryRun {
		return presentation.StyleMuted
	}
	return presentation.StyleSuccess
}

func releaseLifecycleTables(facts releaseLifecyclePresentationFacts) *presentation.Table {
	tables := make([]*presentation.Table, 0, 7)
	if facts.dryRun {
		tables = append(tables,
			releaseLifecycleOperationsTable(facts, false),
			releaseLifecycleMaterializedFilesTable(facts),
		)
	} else {
		tables = append(tables, releaseLifecycleOperationsTable(facts, true))
	}
	tables = append(tables,
		releaseLifecycleSourceTable(facts),
		releaseLifecycleDeclaredFilesTable(facts),
		releaseLifecycleEvidenceTable(facts),
		releaseLifecycleGitHandoffTable(facts),
		releaseLifecycleLimitationsTable(facts),
	)
	return chainReleaseLifecycleTables(tables)
}

func chainReleaseLifecycleTables(tables []*presentation.Table) *presentation.Table {
	var first *presentation.Table
	var tail *presentation.Table
	for _, table := range tables {
		if table == nil {
			continue
		}
		if first == nil {
			first = table
			tail = table
			continue
		}
		tail.Following = table
		tail = table
	}
	return first
}

func releaseLifecycleOperationsTable(facts releaseLifecyclePresentationFacts, describeOnly bool) *presentation.Table {
	rows := releaseLifecycleOperationRows(facts)
	if len(rows) == 0 {
		return nil
	}
	return &presentation.Table{
		Title: "Operations", DescribeOnly: describeOnly,
		Columns: []presentation.Column{
			{Key: "action", Label: "Action", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "scope", Label: "Scope"},
		},
		Rows: rows,
	}
}

func releaseLifecycleOperationRows(facts releaseLifecyclePresentationFacts) []map[string]any {
	result := "Planned"
	if !facts.dryRun {
		result = "Completed"
	}
	rows := []map[string]any{
		{
			"action": "Resolve release identity",
			"status": fmt.Sprintf("%s \u2192 %s (%s)", facts.currentVersion, facts.nextVersion, facts.command),
			"scope":  facts.unit,
		},
	}
	if facts.legacyCompatibilityResult {
		return append(rows,
			map[string]any{"action": "Prepare V1 release configuration", "status": result, "scope": legacyReleaseConfigFile},
			map[string]any{"action": "Invoke configured release tool", "status": result, "scope": facts.executor},
			map[string]any{"action": "Complete release lifecycle", "status": facts.status, "scope": "V1 compatibility"},
		)
	}
	rows = append(rows,
		map[string]any{
			"action": "Prepare materialized files",
			"status": releaseLifecycleCountStatus(len(facts.materializedFiles), result),
			"scope":  "release commit",
		},
		map[string]any{"action": "Create release commit", "status": result, "scope": lifecycleFallback(facts.commitMessage, "established result")},
		map[string]any{"action": "Create unit tag", "status": result, "scope": facts.tag},
		map[string]any{"action": "Push release commit and unit tag", "status": result, "scope": facts.delivery},
	)
	if facts.delivery == string(config.DeliveryGitHubActions) {
		rows = append(rows, map[string]any{
			"action": "Prepare workflow handoff", "status": facts.handoff,
			"scope": releasePlanSafePath(facts.workflow),
		})
	}
	return rows
}

func releaseLifecycleCountStatus(count int, status string) string {
	if count == 0 {
		return "No file changes"
	}
	return fmt.Sprintf("%d %s", count, strings.ToLower(status))
}

func releaseLifecycleMaterializedFilesTable(facts releaseLifecyclePresentationFacts) *presentation.Table {
	rows := make([]map[string]any, 0, len(facts.materializedFiles)+1)
	for _, path := range facts.materializedFiles {
		rows = append(rows, map[string]any{
			"path": releaseLifecycleSafePath(path), "action": "Planned", "status": "Dry run",
		})
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]any{
			"path": "No materialized file changes", "action": "None", "status": "Dry run",
		})
	}
	if facts.materializationBlocker != "" {
		rows = append(rows, map[string]any{
			"path": "Materialization", "action": "Blocked", "status": facts.materializationBlocker,
		})
	}
	return &presentation.Table{
		Title: "Materialized Files",
		Columns: []presentation.Column{
			{Key: "path", Label: "Path", Essential: true},
			{Key: "action", Label: "Action", Essential: true},
			{Key: "status", Label: "Status"},
		},
		Rows: rows,
		Note: "Mutation boundary: dry-run reports the plan and does not write these files.",
	}
}

func releaseLifecycleSourceTable(facts releaseLifecyclePresentationFacts) *presentation.Table {
	return &presentation.Table{
		Title: "Source and Configuration", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "source", Label: "Source", Essential: true},
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "ownership", Label: "Ownership", Essential: true},
			{Key: "executor", Label: "Executor"},
			{Key: "delivery", Label: "Delivery"},
			{Key: "workflow", Label: "Workflow"},
		},
		Rows: []map[string]any{{
			"source": releaseLifecycleReadableValue(facts.source), "unit": facts.unit,
			"ownership": releaseLifecycleSourceOwnership(facts),
			"executor":  facts.executor, "delivery": facts.delivery,
			"workflow": releasePlanSafePath(facts.workflow),
		}},
		Details: &presentation.Properties{
			SectionTitle: "Resolved release identity",
			Properties: []presentation.Property{
				{Label: "Release identity", Value: facts.unit + "@" + facts.nextVersion},
				{Label: "Current version", Value: facts.currentVersion},
				{Label: "Requested change", Value: facts.command},
				{Label: "Resulting version", Value: facts.nextVersion},
				{Label: "Tag prefix", Value: releaseLifecycleTagPrefix(facts)},
				{Label: "Resulting tag", Value: facts.tag},
				{Label: "Working directory", Value: releasePlanSafePath(facts.workingDirectory)},
				{Label: "Unit root", Value: releaseLifecycleSafeUnitRoot(facts)},
			},
		},
	}
}

func releaseLifecycleDeclaredFilesTable(facts releaseLifecyclePresentationFacts) *presentation.Table {
	rows := make([]map[string]any, 0, len(facts.knownReleaseFiles))
	for _, path := range facts.knownReleaseFiles {
		rows = append(rows, map[string]any{
			"path": releaseLifecycleSafePath(path), "kind": "Known release file", "ownership": "Release lifecycle",
		})
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]any{
			"path": "Not retained in the established outcome", "kind": "Outcome boundary", "ownership": "Release lifecycle",
		})
	}
	return &presentation.Table{
		Title: "Declared Release Files", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "path", Label: "Path", Essential: true},
			{Key: "kind", Label: "Kind", Essential: true},
			{Key: "ownership", Label: "Ownership"},
		},
		Rows: rows,
	}
}

func releaseLifecycleEvidenceTable(facts releaseLifecyclePresentationFacts) *presentation.Table {
	rows := []map[string]any{
		{"evidence": "Lifecycle result", "status": facts.status, "value": facts.handoff},
		{"evidence": "Release commit", "status": releaseLifecycleEvidenceStatus(facts.commitSHA, facts.dryRun), "value": lifecycleFallback(facts.commitSHA, "not retained")},
		{"evidence": "Execution journal", "status": releaseLifecycleEvidenceStatus(facts.executionJournal, facts.dryRun), "value": releaseLifecycleSafePath(facts.executionJournal)},
	}
	if facts.dispatch != nil || facts.gitHubActionsResult {
		rows = append(rows, map[string]any{
			"evidence": "Dispatch journal",
			"status":   releaseLifecycleDispatchEvidenceStatus(facts),
			"value":    releaseLifecycleDispatchJournalValue(facts),
		})
	}
	return &presentation.Table{
		Title: "Execution Evidence", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "evidence", Label: "Evidence", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "value", Label: "Value"},
		},
		Rows: rows,
	}
}

func releaseLifecycleEvidenceStatus(value string, dryRun bool) string {
	if dryRun {
		return "Not created"
	}
	if strings.TrimSpace(value) == "" {
		return "Not retained"
	}
	return "Recorded"
}

func releaseLifecycleDispatchEvidenceStatus(facts releaseLifecyclePresentationFacts) string {
	if facts.dryRun {
		return "Not created"
	}
	return releaseLifecycleReadableValue(facts.dispatchState)
}

func releaseLifecycleDispatchJournalValue(facts releaseLifecyclePresentationFacts) string {
	if facts.dispatch != nil {
		return releaseLifecycleSafePath(facts.dispatch.JournalLocation)
	}
	return releaseLifecycleSafePath(facts.dispatchJournal)
}

func releaseLifecycleGitHandoffTable(facts releaseLifecyclePresentationFacts) *presentation.Table {
	commitStatus := "Not started"
	tagStatus := "Not started"
	pushStatus := "Not started"
	dispatchStatus := "Not started"
	if !facts.dryRun {
		commitStatus = releaseLifecycleEvidenceStatus(facts.commitSHA, false)
		tagStatus = lifecycleFallback(facts.executionState, "not retained")
		pushStatus = lifecycleFallback(facts.executionState, "not retained")
		dispatchStatus = lifecycleFallback(facts.dispatchState, facts.handoff)
	}
	rows := []map[string]any{
		{"action": "Release commit", "status": releaseLifecycleReadableValue(commitStatus), "evidence": lifecycleFallback(facts.commitMessage, releaseLifecycleShortSHA(facts.commitSHA))},
		{"action": "Unit tag", "status": releaseLifecycleReadableValue(tagStatus), "evidence": facts.tag},
		{"action": "Push", "status": releaseLifecycleReadableValue(pushStatus), "evidence": "release commit, then unit tag"},
	}
	if facts.delivery == string(config.DeliveryGitHubActions) {
		rows = append(rows, map[string]any{
			"action": "Workflow handoff", "status": releaseLifecycleReadableValue(dispatchStatus),
			"evidence": releasePlanSafePath(facts.workflow),
		})
	}
	return &presentation.Table{
		Title: "Git and Handoff", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "action", Label: "Action", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "evidence", Label: "Evidence"},
		},
		Rows: rows,
		Details: &presentation.Properties{
			SectionTitle: "Ownership and next action",
			Properties: []presentation.Property{
				{Label: "Tool ownership", Value: lifecycleFallback(facts.ownership, "not retained in the established outcome")},
				{Label: "Git ownership", Value: lifecycleFallback(facts.gitOwnership, "not retained in the established outcome")},
				{Label: "State guarantee", Value: lifecycleFallback(facts.stateGuarantee, "not retained in the established outcome")},
				{Label: "Handoff state", Value: facts.handoff},
				{Label: "Next action", Value: facts.nextAction},
			},
		},
	}
}

func releaseLifecycleLimitationsTable(facts releaseLifecyclePresentationFacts) *presentation.Table {
	rows := []map[string]any{
		{
			"limitation": "Outcome boundary",
			"statement":  "Presentation uses only facts retained by the established release outcome.",
		},
		{
			"limitation": "Remote evidence",
			"statement":  "Remote tags, pushes, releases, and workflow runs are not inspected by presentation mapping.",
		},
	}
	if facts.dryRun {
		rows = append(rows, map[string]any{
			"limitation": "Dry-run boundary",
			"statement":  "No release tool, journal, commit, tag, push, dispatch, upload, or publication is started.",
		})
	}
	if facts.legacyCompatibilityResult {
		rows = append(rows, map[string]any{
			"limitation": "V1 evidence",
			"statement":  "The established V1 outcome does not retain journal, push, or dispatch evidence.",
		})
	}
	if facts.materializationBlocker != "" {
		rows = append(rows, map[string]any{
			"limitation": "Materialization blocker", "statement": facts.materializationBlocker,
		})
	}
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "limitation", Label: "Limitation", Essential: true},
			{Key: "statement", Label: "Statement", Essential: true},
		},
		Rows: rows,
	}
}

func releaseLifecycleSourceOwnership(facts releaseLifecyclePresentationFacts) string {
	if facts.legacyCompatibilityResult {
		return "Legacy V1 compatibility lifecycle"
	}
	return "Authoritative V2 release lifecycle"
}

func releaseLifecycleTagPrefix(facts releaseLifecyclePresentationFacts) string {
	if facts.legacyCompatibilityResult {
		return "v"
	}
	if facts.nextVersion != "" && strings.HasSuffix(facts.tag, facts.nextVersion) {
		return strings.TrimSuffix(facts.tag, facts.nextVersion)
	}
	return "not retained in the established outcome"
}

func releaseLifecycleSafeUnitRoot(facts releaseLifecyclePresentationFacts) string {
	if !filepath.IsAbs(facts.unitRoot) {
		return releasePlanSafePath(facts.unitRoot)
	}
	if strings.TrimSpace(facts.workingDirectory) == "." {
		return "repository root"
	}
	return releasePlanSafePath(facts.workingDirectory)
}

func releaseLifecycleSafePath(value string) string {
	value = strings.TrimSpace(value)
	if !filepath.IsAbs(value) {
		return releasePlanSafePath(value)
	}
	slashed := filepath.ToSlash(filepath.Clean(value))
	for _, marker := range []string{"/.git/", "/.neko/"} {
		if index := strings.LastIndex(slashed, marker); index >= 0 {
			return strings.TrimPrefix(slashed[index+1:], "/")
		}
	}
	return "repository-local path"
}

func releaseLifecycleReadableValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Not recorded"
	}
	return releasePlanReadableLabel(value)
}

func releaseLifecycleHandoff(result *GitHubActionsReleaseResult) string {
	if result == nil {
		return "Not recorded"
	}
	if result.DispatchState != "" {
		return "Dispatch " + releaseLifecycleReadableValue(string(result.DispatchState))
	}
	return releaseLifecycleReadableValue(string(result.ExecutionState))
}

func releaseLifecycleShortSHA(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 12 {
		return value[:12]
	}
	return lifecycleFallback(value, "not retained")
}

func lifecycleFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func attachLifecycleFailurePresentation(response *plugin.Response, command string, failure *CommandFailure) {
	if response == nil || failure == nil || !isLifecyclePresentationCommand(command) {
		return
	}
	title := "Release Refused"
	status := "Refused"
	if command == "resume" {
		title = "Resume Refused"
	} else if releaseLifecycleRejectedFailure(failure.Code) {
		title = "Release Rejected"
		status = "Rejected"
	}
	response.PresentationProperties = &presentation.Properties{
		Title: title,
		Properties: []presentation.Property{
			{Label: "Status", Value: status, Role: presentation.StyleError, Emphasized: true},
			{Label: "Code", Value: failure.Code},
			{Label: "Reason", Value: safeLifecycleFailureReason(failure.responseMessage()), Role: presentation.StyleError},
			{Label: "Retry safety", Value: "Re-evaluate after resolving the reported condition", Role: presentation.StyleWarning},
			{Label: "Next safe action", Value: lifecycleFailureNextAction(command), Emphasized: true},
		},
	}
}

var (
	lifecycleAbsolutePathPattern  = regexp.MustCompile(`(^|[\s(=])(/[^\s,;]+)`)
	lifecycleURLCredentialPattern = regexp.MustCompile(`(?i)(https?://)[^/@\s]+@`)
	lifecycleBearerPattern        = regexp.MustCompile(`(?i)(authorization:\s*bearer|bearer)\s+\S+`)
)

func safeLifecycleFailureReason(message string) string {
	message = lifecycleURLCredentialPattern.ReplaceAllString(message, "$1")
	message = lifecycleBearerPattern.ReplaceAllString(message, "$1 [redacted]")
	return lifecycleAbsolutePathPattern.ReplaceAllString(message, "${1}repository-local path")
}

func isLifecyclePresentationCommand(command string) bool {
	switch command {
	case "patch", "minor", "major", "resume":
		return true
	default:
		return false
	}
}

func releaseLifecycleRejectedFailure(code string) bool {
	return strings.Contains(code, "REJECTED") || code == "V2_GITHUB_ACTIONS_RELEASE_FAILED"
}

func lifecycleFailureNextAction(command string) string {
	if command == "resume" {
		return "Inspect the exact recovery reason and local evidence; do not retry until the reported condition is resolved."
	}
	return "Resolve the reported release condition, verify local state, then rerun the same command."
}
