//nolint:staticcheck // Migration presentation intentionally names the deprecated V1 source contract.
package migrate

import (
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func migrationSummaryPresentation(result migrationCommandResult) *presentation.Properties {
	plan := result.plan
	status, role := migrationOutcomeLabel(result.outcome)
	return &presentation.Properties{
		Title: "Release Migration",
		Properties: []presentation.Property{
			{Label: "Result", Value: status, Role: role, Emphasized: true},
			{Label: "Source contract", Value: strings.ToUpper(string(plan.sourceFormat))},
			{Label: "Destination contract", Value: "V2"},
			{Label: "Dry run", Value: migrationYesNo(result.outcome == migrationPreviewed)},
			{Label: "Planned actions", Value: len(plan.actions)},
			{Label: "Configuration", Value: releaseconfig.V2ConfigPath(".")},
			{Label: "State", Value: releaseconfig.V2StatePath(".")},
			{Label: "Archive", Value: migrationSummaryArchive(result)},
			{Label: "Next action", Value: migrationNextAction(result)},
		},
	}
}

func migrationSummaryArchive(result migrationCommandResult) string {
	if result.outcome == migrationAlreadyCompleted {
		return "Not required"
	}
	return backupFileName
}

func migrationOutcomeLabel(outcome migrationCommandOutcome) (string, presentation.StyleRole) {
	switch outcome {
	case migrationPreviewed:
		return "Migration plan ready", presentation.StyleInfo
	case migrationCompleted:
		return "Migration completed", presentation.StyleSuccess
	case migrationAlreadyCompleted:
		return "Already migrated; no changes required", presentation.StyleSuccess
	default:
		return "Migration status unavailable", presentation.StyleWarning
	}
}

func migrationNextAction(result migrationCommandResult) string {
	switch result.outcome {
	case migrationPreviewed:
		return "Run 'neko release migrate' without --dry-run after reviewing the plan"
	case migrationCompleted:
		return "Run 'neko release validate --show'"
	case migrationAlreadyCompleted:
		return "No migration action required"
	default:
		return "Review the migration state before retrying"
	}
}

func migrationYesNo(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func migrationDetailPresentation(result migrationCommandResult) *presentation.Table {
	plan := result.plan
	source := &presentation.Table{
		Title: "Source Facts", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "contract", Label: "Contract", Essential: true},
			{Key: "artifact", Label: "Artifact", Essential: true},
			{Key: "state", Label: "State", Essential: true},
		},
		Rows: []map[string]any{{
			"contract": strings.ToUpper(string(plan.sourceFormat)),
			"artifact": migrationSourceArtifact(plan),
			"state":    migrationSourceState(plan),
		}},
	}
	source.Details = &presentation.Properties{
		SectionTitle: "Normalized source",
		Properties: []presentation.Property{
			{Label: "Unit", Value: migrationValueOrNotApplicable(plan.unitID)},
			{Label: "Version", Value: migrationValueOrNotApplicable(plan.version)},
			{Label: "Tag prefix", Value: migrationValueOrNotApplicable(plan.tagPrefix)},
			{Label: "Executor", Value: migrationValueOrNotApplicable(plan.executor)},
			{Label: "Delivery", Value: migrationValueOrNotApplicable(plan.delivery)},
		},
	}
	source.Following = migrationResolvedV2Table(plan)
	source.Following.Following = migrationGeneratedArtifactsTable(result)
	source.Following.Following.Following = migrationOrderedPlanTable(result)
	source.Following.Following.Following.Following = migrationArchiveJournalTable(result)
	source.Following.Following.Following.Following.Following = migrationValidationTable(result)
	source.Following.Following.Following.Following.Following.Following = migrationWritePlanTable(result)
	source.Following.Following.Following.Following.Following.Following.Following = migrationLimitationsTable()
	return source
}

func migrationSourceArtifact(plan migrationPlan) string {
	if plan.sourceFormat == migrationSourceV1 {
		return releaseconfig.V1FileName
	}
	return releaseconfig.V2Directory + "/"
}

func migrationSourceState(plan migrationPlan) string {
	switch plan.kind {
	case completedMigrationPlanKind:
		return "Already migrated"
	case recoveryMigrationPlan:
		return "Interrupted migration recovery"
	default:
		return "Active migration source"
	}
}

func migrationValueOrNotApplicable(value string) string {
	if strings.TrimSpace(value) == "" {
		return "not applicable"
	}
	return value
}

func migrationResolvedV2Table(plan migrationPlan) *presentation.Table {
	workflow := defaultMigratedWorkflow
	workingDirectory := "."
	declaredPaths := "**"
	if plan.kind == completedMigrationPlanKind {
		workflow = "existing V2 configuration"
		workingDirectory = "existing V2 configuration"
		declaredPaths = "existing V2 configuration"
	}
	return &presentation.Table{
		Title: "Resolved V2 Configuration", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "version", Label: "Version", Essential: true},
			{Key: "executor", Label: "Executor", Essential: true},
			{Key: "delivery", Label: "Delivery"},
			{Key: "workflow", Label: "Workflow"},
			{Key: "tag_prefix", Label: "Tag Prefix"},
		},
		Rows: []map[string]any{{
			"unit": migrationValueOrNotApplicable(plan.unitID), "version": migrationValueOrNotApplicable(plan.version),
			"executor": migrationValueOrNotApplicable(plan.executor), "delivery": migrationValueOrNotApplicable(plan.delivery),
			"workflow": workflow, "tag_prefix": migrationValueOrNotApplicable(plan.tagPrefix),
		}},
		Details: &presentation.Properties{
			SectionTitle: "Resolved target",
			Properties: []presentation.Property{
				{Label: "Schema", Value: "V2"},
				{Label: "Working directory", Value: workingDirectory},
				{Label: "Declared paths", Value: declaredPaths},
				{Label: "Configuration", Value: releaseconfig.V2ConfigPath(".")},
				{Label: "State", Value: releaseconfig.V2StatePath(".")},
			},
		},
	}
}

func migrationGeneratedArtifactsTable(result migrationCommandResult) *presentation.Table {
	action := migrationTargetAction(result)
	return &presentation.Table{
		Title: "Generated Artifacts", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "artifact", Label: "Artifact", Essential: true},
			{Key: "content", Label: "Content", Essential: true},
			{Key: "action", Label: "Action", Essential: true},
		},
		Rows: []map[string]any{
			{"artifact": releaseconfig.V2ConfigPath("."), "content": "Canonical V2 release configuration", "action": action},
			{"artifact": releaseconfig.V2StatePath("."), "content": "Canonical V2 release state", "action": action},
		},
		Note: "Generated JSON remains available through --output json; human output summarizes the artifacts.",
	}
}

func migrationTargetAction(result migrationCommandResult) string {
	if result.plan.kind == completedMigrationPlanKind {
		return "Retain"
	}
	if result.outcome == migrationPreviewed {
		return "Would write"
	}
	if result.plan.kind == recoveryMigrationPlan && result.plan.targetOperation == retainMigrationTarget {
		return "Retained from recovery"
	}
	return "Written"
}

func migrationOrderedPlanTable(result migrationCommandResult) *presentation.Table {
	rows := make([]map[string]any, 0, len(result.plan.actions))
	for index, action := range result.plan.actions {
		rows = append(rows, map[string]any{
			"order": index + 1, "action": action, "outcome": migrationPlannedActionOutcome(result),
		})
	}
	return &presentation.Table{
		Title: "Ordered Migration Plan", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "order", Label: "#", Essential: true},
			{Key: "action", Label: "Action", Essential: true},
			{Key: "outcome", Label: "Outcome", Essential: true},
		},
		Rows: rows,
	}
}

func migrationPlannedActionOutcome(result migrationCommandResult) string {
	switch result.outcome {
	case migrationPreviewed:
		return "Planned only"
	case migrationCompleted:
		return "Completed"
	case migrationAlreadyCompleted:
		return "No change required"
	default:
		return "Unknown"
	}
}

func migrationArchiveJournalTable(result migrationCommandResult) *presentation.Table {
	archiveAction, journalAction := "Not required", "Not created"
	if result.plan.kind != completedMigrationPlanKind {
		if result.outcome == migrationPreviewed {
			archiveAction, journalAction = "Would archive", "Would create and remove"
		} else {
			archiveAction, journalAction = "Archived", "Removed after successful completion"
		}
		if result.plan.kind == recoveryMigrationPlan {
			journalAction = "Recovered and removed after completion"
		}
	}
	return &presentation.Table{
		Title: "Archive and Journal", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "artifact", Label: "Artifact", Essential: true},
			{Key: "policy", Label: "Policy", Essential: true},
			{Key: "outcome", Label: "Outcome", Essential: true},
		},
		Rows: []map[string]any{
			{"artifact": backupFileName, "policy": "Byte-identical V1 archive", "outcome": archiveAction},
			{"artifact": releaseconfig.V2Directory + "/" + journalFileName, "policy": "Recoverable write evidence", "outcome": journalAction},
		},
	}
}

func migrationValidationTable(result migrationCommandResult) *presentation.Table {
	writeStatus := "Completed and verified"
	if result.outcome == migrationPreviewed {
		writeStatus = "Not executed during dry-run"
	}
	if result.outcome == migrationAlreadyCompleted {
		writeStatus = "Existing V2 pair validated"
	}
	return &presentation.Table{
		Title: "Validation Facts", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "check", Label: "Check", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "details", Label: "Details"},
		},
		Rows: []map[string]any{
			{"check": "Source", "status": "Valid", "details": "Supported release source contract"},
			{"check": "Generated pair", "status": "Valid", "details": "Canonical V2 config and state agree"},
			{"check": "Filesystem outcome", "status": writeStatus, "details": "Pair writer and migration journal preserve recovery evidence"},
		},
	}
}

func migrationWritePlanTable(result migrationCommandResult) *presentation.Table {
	return &presentation.Table{
		Title: "Write Plan", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "scope", Label: "Scope", Essential: true},
			{Key: "decision", Label: "Decision", Essential: true},
			{Key: "outcome", Label: "Outcome", Essential: true},
		},
		Rows: []map[string]any{
			{"scope": "V2 target pair", "decision": migrationTargetAction(result), "outcome": migrationPlannedActionOutcome(result)},
			{"scope": "V1 source archive", "decision": migrationSourceWriteDecision(result), "outcome": migrationPlannedActionOutcome(result)},
			{"scope": "Migration journal", "decision": migrationJournalWriteDecision(result), "outcome": migrationPlannedActionOutcome(result)},
		},
	}
}

func migrationSourceWriteDecision(result migrationCommandResult) string {
	if result.plan.kind == completedMigrationPlanKind {
		return "Retain"
	}
	if result.outcome == migrationPreviewed {
		return "Would archive"
	}
	return "Archived"
}

func migrationJournalWriteDecision(result migrationCommandResult) string {
	if result.plan.kind == completedMigrationPlanKind {
		return "None"
	}
	if result.outcome == migrationPreviewed {
		return "Would create and remove"
	}
	return "Removed after completion"
}

func migrationLimitationsTable() *presentation.Table {
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "scope", Label: "Scope", Essential: true},
			{Key: "details", Label: "Details", Essential: true},
		},
		Rows: []map[string]any{
			{"scope": "Source support", "details": "Only the supported root V1 contract is converted automatically."},
			{"scope": "Workflow", "details": "Migration references the default workflow but does not create it."},
			{"scope": "Release execution", "details": "No tag, dispatch, upload, publication, or release is performed."},
		},
	}
}

func migrationFailureProperties() *presentation.Properties {
	return &presentation.Properties{
		Title: "Migration Request Refused",
		Properties: []presentation.Property{
			{Label: "Result", Value: "Migration did not complete", Role: presentation.StyleError, Emphasized: true},
		},
	}
}

func migrationFailurePresentation(failure *migrationFailure) *presentation.Table {
	area, reason, nextAction := migrationFailureFacts(failure)
	writeOutcome := migrationFailureWriteOutcome(failure)
	recovery := "No migration write started"
	if failure != nil && failure.kind != migrationPlanningFailure {
		recovery = "Inspect retained migration journal and repository artifacts"
	}
	if failure != nil && failure.manualRecoveryRequired {
		recovery = "Manual recovery required; retain all journal and target evidence"
	}
	return &presentation.Table{
		Title: "Migration Blocked",
		Columns: []presentation.Column{
			{Key: "code", Label: "Code", Essential: true},
			{Key: "area", Label: "Area", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "reason", Label: "Reason", Essential: true},
			{Key: "write", Label: "Write outcome", Essential: true},
			{Key: "recovery", Label: "Recovery", Essential: true},
			{Key: "next", Label: "Next action", Essential: true},
		},
		Rows: []map[string]any{{
			"code": "MIGRATION_FAILED", "area": area, "status": "Refused", "reason": reason, "write": writeOutcome,
			"recovery": recovery, "next": nextAction,
		}},
	}
}

func migrationFailureWriteOutcome(failure *migrationFailure) string {
	if failure == nil || failure.kind == migrationPlanningFailure {
		return "No files written"
	}
	return "Stopped; recovery evidence may remain"
}

func migrationFailureFacts(failure *migrationFailure) (area, reason, nextAction string) {
	area = "Migration"
	reason = "Migration prerequisites were not satisfied."
	nextAction = "Correct the repository state and retry."
	if failure == nil || failure.cause == nil {
		return area, reason, nextAction
	}
	message := failure.Error()
	if failure.kind == migrationPlanningFailure {
		switch {
		case strings.Contains(message, "parse V1 config"):
			return "V1 source", "The V1 release configuration is not valid JSON.", "Repair .release.neko.json and retry."
		case strings.Contains(message, "ReleaseSystem") || strings.Contains(message, "validate"):
			return "V1 source", "The V1 release configuration is unsupported or invalid.", "Correct the reported V1 field and retry."
		case strings.Contains(message, "nested V1"):
			return "Nested V1 source", "A nested V1 source cannot be converted as one repository unit.", "Create an explicit V2 multi-unit configuration."
		case strings.Contains(message, "no release configuration"):
			return "Release source", "No supported release configuration was found.", "Restore a root V1 source or initialize V2 explicitly."
		case strings.Contains(message, "incomplete V2"):
			return "V2 target pair", "The V2 config/state pair is incomplete.", "Restore or remove the incomplete pair before retrying."
		case strings.Contains(message, "active V1 config and V2"):
			return "V1/V2 source conflict", "Active V1 and V2 targets coexist without migration recovery evidence.", "Resolve the source conflict explicitly; migration will not overwrite V2."
		case strings.Contains(message, "backup") && strings.Contains(message, "differs"):
			return "V1 archive", "The existing V1 archive differs from the active source.", "Compare and preserve the intended archive before retrying."
		case strings.Contains(message, "journal") || strings.Contains(message, "recovery"):
			return "Migration recovery", "Migration recovery evidence is incomplete or inconsistent.", "Preserve the journal and artifacts, then resolve the reported recovery mismatch."
		}
	}
	switch failure.kind {
	case migrationTargetPersistenceFailure:
		return "V2 target pair", "The V2 configuration/state pair could not be persisted.", "Resolve the filesystem failure and retry using retained recovery evidence."
	case migrationTargetVerificationFailure:
		return "V2 target verification", "The persisted V2 target did not match the migration plan.", "Do not archive V1; inspect the retained target and journal evidence."
	case migrationSourceCleanupFailure:
		return "V1 archive", "The validated V1 source could not be archived safely.", "Preserve source, target, and journal evidence before retrying."
	case migrationSourceVerificationFailure:
		return "V1 archive verification", "The V1 archive could not be verified.", "Inspect the retained archive and journal evidence."
	case migrationJournalFailure:
		return "Migration journal", "The recovery journal could not be updated or removed.", "Preserve all artifacts and resume only after inspecting journal state."
	case migrationRestorationFailure:
		return "Migration recovery", "Automatic restoration did not complete.", "Perform manual recovery using the retained journal and artifact evidence."
	default:
		return area, reason, nextAction
	}
}
