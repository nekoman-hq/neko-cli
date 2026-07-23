package release

import (
	"path/filepath"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

// resumePresentationFacts is a presentation-only projection of the
// established Resume outcome variants. Recovery status, eligibility, and retry
// safety remain owned by the authoritative recovery assessment.
//
//nolint:govet // Fields follow the recovery decision order.
type resumePresentationFacts struct {
	unit               string
	version            string
	tag                string
	executionJournal   string
	dispatchJournal    string
	executionIdentity  string
	dispatchIdentity   string
	journalPhase       string
	pendingAction      string
	recoveryStatus     string
	executionState     string
	dispatchState      string
	guidance           string
	commitSHA          string
	workflow           string
	knownFiles         []string
	safeToContinue     bool
	dryRun             bool
	completedExecution bool
	assessmentOutcome  bool
}

func attachResumePresentation(response *plugin.Response, outcome ResumeCommandOutcome) {
	if response == nil {
		return
	}
	facts, ok := resumeFacts(outcome)
	if !ok {
		return
	}
	response.PresentationProperties = &presentation.Properties{
		Title:      "Resume Summary",
		Properties: resumeSummaryProperties(facts),
	}
	response.PresentationTable = resumePresentationTables(facts)
}

func resumeFacts(outcome ResumeCommandOutcome) (resumePresentationFacts, bool) {
	switch result := outcome.(type) {
	case *ResumeAssessment:
		return resumePresentationFacts{
			unit:              result.UnitID,
			version:           result.NextVersion,
			tag:               result.Tag,
			executionJournal:  result.ExecutionJournalPath,
			executionIdentity: resumeJournalIdentity(result.ExecutionJournalPath),
			journalPhase:      string(result.State),
			pendingAction:     string(result.PendingAction),
			recoveryStatus:    string(result.RecoveryStatus),
			guidance:          result.Guidance,
			knownFiles:        append([]string(nil), result.KnownFilePaths...),
			safeToContinue:    result.SafeToContinue,
			dryRun:            true,
			assessmentOutcome: true,
		}, true
	case *GitHubActionsReleaseResult:
		return resumePresentationFacts{
			unit:               result.Unit,
			version:            result.Version,
			tag:                result.Tag,
			executionJournal:   result.ExecutionJournalPath,
			dispatchJournal:    result.DispatchJournalPath,
			executionIdentity:  resumeJournalIdentity(result.ExecutionJournalPath),
			dispatchIdentity:   resumeJournalIdentity(result.DispatchJournalPath),
			journalPhase:       string(result.ExecutionState),
			pendingAction:      string(ReleaseExecutionPendingNone),
			recoveryStatus:     string(result.ExecutionState),
			executionState:     string(result.ExecutionState),
			dispatchState:      string(result.DispatchState),
			guidance:           result.RecoveryGuidance,
			commitSHA:          result.CommitSHA,
			workflow:           result.Workflow,
			completedExecution: true,
		}, true
	default:
		return resumePresentationFacts{}, false
	}
}

func resumeSummaryProperties(facts resumePresentationFacts) []presentation.Property {
	properties := []presentation.Property{
		{Label: "Execution identity", Value: facts.executionIdentity, Emphasized: true},
		{Label: "Unit", Value: facts.unit},
		{Label: "Version", Value: facts.version},
		{Label: "Tag", Value: facts.tag},
		{Label: "Journal phase", Value: releaseLifecycleReadableValue(facts.journalPhase), Emphasized: true},
		{Label: "Pending action", Value: releaseLifecycleReadableValue(facts.pendingAction)},
		{
			Label: "Recovery status", Value: releaseLifecycleReadableValue(facts.recoveryStatus),
			Role: resumeRecoveryRole(facts),
		},
		{
			Label: "Resume eligibility", Value: resumeEligibility(facts),
			Role: resumeEligibilityRole(facts), Emphasized: true,
		},
		{
			Label: "Retry safety", Value: resumeRetrySafety(facts),
			Role: resumeEligibilityRole(facts),
		},
	}
	if facts.dryRun {
		properties = append(properties,
			presentation.Property{Label: "Dry run", Value: "Yes; recovery assessment only", Role: presentation.StyleInfo},
			presentation.Property{
				Label: "Mutation boundary",
				Value: "No continuation, journal write, Git mutation, push, or dispatch was performed",
				Role:  presentation.StyleMuted,
			},
		)
	}
	return append(properties, presentation.Property{
		Label: "Next action", Value: facts.guidance, Emphasized: true,
	})
}

func resumeRecoveryRole(facts resumePresentationFacts) presentation.StyleRole {
	status := strings.ToLower(facts.recoveryStatus)
	switch {
	case strings.Contains(status, "conflict"), strings.Contains(status, "corrupt"),
		strings.Contains(status, "reject"), strings.Contains(status, "unknown"):
		return presentation.StyleError
	case facts.safeToContinue:
		return presentation.StyleSuccess
	case facts.completedExecution:
		return presentation.StyleSuccess
	default:
		return presentation.StyleWarning
	}
}

func resumeEligibility(facts resumePresentationFacts) string {
	switch {
	case facts.completedExecution:
		return "Continuation completed"
	case facts.safeToContinue:
		return "Eligible"
	default:
		return "Not eligible"
	}
}

func resumeRetrySafety(facts resumePresentationFacts) string {
	switch {
	case facts.completedExecution:
		return "No retry required"
	case facts.safeToContinue:
		return "Safe"
	default:
		return "Unsafe until the reported condition is resolved"
	}
}

func resumeEligibilityRole(facts resumePresentationFacts) presentation.StyleRole {
	if facts.safeToContinue || facts.completedExecution {
		return presentation.StyleSuccess
	}
	return presentation.StyleError
}

func resumePresentationTables(facts resumePresentationFacts) *presentation.Table {
	tables := []*presentation.Table{
		resumeContinuationTable(facts, !facts.dryRun),
		resumeJournalTable(facts),
		resumeGitEvidenceTable(facts),
		resumeAssessmentTable(facts),
		resumeHandoffTable(facts),
		resumeLimitationsTable(facts),
	}
	return chainResumeTables(tables)
}

func chainResumeTables(tables []*presentation.Table) *presentation.Table {
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

func resumeContinuationTable(facts resumePresentationFacts, describeOnly bool) *presentation.Table {
	action := releaseLifecycleReadableValue(facts.pendingAction)
	if strings.TrimSpace(facts.pendingAction) == "" || facts.pendingAction == string(ReleaseExecutionPendingNone) {
		action = "Recovery assessment guidance"
	}
	status := "Would continue only after this dry-run assessment"
	if !facts.safeToContinue {
		status = "Refused by the authoritative recovery assessment"
	}
	if facts.completedExecution {
		status = "Completed"
	}
	return &presentation.Table{
		Title: "Planned Continuation", DescribeOnly: describeOnly,
		Columns: []presentation.Column{
			{Key: "action", Label: "Action", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "reason", Label: "Reason", Essential: true},
		},
		Rows: []map[string]any{{
			"action": action, "status": status, "reason": facts.guidance,
		}},
		Note: resumeContinuationNote(facts),
	}
}

func resumeContinuationNote(facts resumePresentationFacts) string {
	if facts.dryRun {
		return "Mutation boundary: dry-run performs assessment only and does not continue the release."
	}
	return "The resulting state is the established Resume command outcome."
}

func resumeJournalTable(facts resumePresentationFacts) *presentation.Table {
	rows := []map[string]any{
		{
			"journal": "Execution", "identity": facts.executionIdentity,
			"state": releaseLifecycleReadableValue(facts.journalPhase),
			"path":  releaseLifecycleSafePath(facts.executionJournal),
		},
	}
	if facts.dispatchJournal != "" || facts.dispatchIdentity != "" {
		rows = append(rows, map[string]any{
			"journal": "Dispatch", "identity": lifecycleFallback(facts.dispatchIdentity, "not retained"),
			"state": releaseLifecycleReadableValue(facts.dispatchState),
			"path":  releaseLifecycleSafePath(facts.dispatchJournal),
		})
	}
	return &presentation.Table{
		Title: "Recovery Journal", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "journal", Label: "Journal", Essential: true},
			{Key: "identity", Label: "Identity", Essential: true},
			{Key: "state", Label: "State", Essential: true},
			{Key: "path", Label: "Path"},
		},
		Rows: rows,
		Details: &presentation.Properties{
			SectionTitle: "Release identity",
			Properties: []presentation.Property{
				{Label: "Unit", Value: facts.unit},
				{Label: "Version", Value: facts.version},
				{Label: "Tag", Value: facts.tag},
				{Label: "Release commit", Value: lifecycleFallback(releaseLifecycleShortSHA(facts.commitSHA), "not retained in the established outcome")},
				{Label: "Pending action", Value: releaseLifecycleReadableValue(facts.pendingAction)},
				{Label: "Execution journal path", Value: releaseLifecycleSafePath(facts.executionJournal)},
			},
		},
	}
}

func resumeGitEvidenceTable(facts resumePresentationFacts) *presentation.Table {
	rows := []map[string]any{
		{
			"evidence": "Release commit",
			"status":   resumeRetainedEvidenceStatus(facts.commitSHA),
			"value":    releaseLifecycleShortSHA(facts.commitSHA),
		},
		{
			"evidence": "Unit tag",
			"status":   "Evaluated by recovery assessment",
			"value":    facts.tag,
		},
		{
			"evidence": "Worktree and index",
			"status":   "Evaluated by recovery assessment",
			"value":    "Detailed state is not retained in ResumeCommandOutcome",
		},
	}
	return &presentation.Table{
		Title: "Local Git Evidence", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "evidence", Label: "Evidence", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "value", Label: "Value"},
		},
		Rows: rows,
	}
}

func resumeRetainedEvidenceStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Not retained in the established outcome"
	}
	return "Recorded"
}

func resumeAssessmentTable(facts resumePresentationFacts) *presentation.Table {
	return &presentation.Table{
		Title: "Recovery Assessment", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "decision", Label: "Decision", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "reason", Label: "Reason", Essential: true},
		},
		Rows: []map[string]any{
			{
				"decision": "Recovery status",
				"status":   releaseLifecycleReadableValue(facts.recoveryStatus),
				"reason":   facts.guidance,
			},
			{"decision": "Resume eligibility", "status": resumeEligibility(facts), "reason": facts.guidance},
			{"decision": "Retry safety", "status": resumeRetrySafety(facts), "reason": facts.guidance},
		},
	}
}

func resumeHandoffTable(facts resumePresentationFacts) *presentation.Table {
	rows := []map[string]any{
		{
			"phase": "Continuation", "state": resumeEligibility(facts),
			"evidence": releaseLifecycleReadableValue(facts.pendingAction),
		},
		{
			"phase": "Execution", "state": releaseLifecycleReadableValue(facts.executionState),
			"evidence": releaseLifecycleSafePath(facts.executionJournal),
		},
	}
	if facts.dispatchState != "" || facts.workflow != "" {
		rows = append(rows, map[string]any{
			"phase": "Dispatch handoff", "state": releaseLifecycleReadableValue(facts.dispatchState),
			"evidence": releasePlanSafePath(facts.workflow),
		})
	}
	return &presentation.Table{
		Title: "Continuation and Handoff", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "phase", Label: "Phase", Essential: true},
			{Key: "state", Label: "State", Essential: true},
			{Key: "evidence", Label: "Evidence"},
		},
		Rows: rows,
		Details: &presentation.Properties{
			SectionTitle: "Recovery guidance",
			Properties: []presentation.Property{
				{Label: "Guidance", Value: facts.guidance},
				{Label: "Mutation boundary", Value: resumeMutationBoundary(facts)},
			},
		},
	}
}

func resumeMutationBoundary(facts resumePresentationFacts) string {
	if facts.dryRun {
		return "Assessment only; no continuation or mutation was performed"
	}
	return "Only the continuation selected by the authoritative Resume operation was executed"
}

func resumeLimitationsTable(facts resumePresentationFacts) *presentation.Table {
	rows := []map[string]any{
		{
			"limitation": "Remote inspection",
			"statement":  "Remote evidence was not inspected by Resume presentation mapping.",
		},
		{
			"limitation": "Outcome boundary",
			"statement":  "Commit, tag, worktree, dispatch linkage, and refusal details are shown only when retained by ResumeCommandOutcome.",
		},
		{
			"limitation": "Policy ownership",
			"statement":  "Journal selection, recovery status, eligibility, and retry safety remain owned by the authoritative Resume use case.",
		},
	}
	if facts.dryRun {
		rows = append(rows, map[string]any{
			"limitation": "Dry-run boundary",
			"statement":  "No journal write, Git mutation, push, dispatch, upload, or publication was performed.",
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

func resumeJournalIdentity(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	identity := strings.TrimSuffix(base, filepath.Ext(base))
	if identity == "" || identity == "." {
		return "unresolved"
	}
	return identity
}
