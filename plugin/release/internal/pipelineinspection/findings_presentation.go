package pipelineinspection

import (
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

var pipelineFindingColumns = []presentation.Column{
	{Key: "check", Label: "Check", Essential: true},
	{Key: "status", Label: "Status", RoleKey: "status_role", Essential: true},
	{Key: "details", Label: "Details", Essential: true},
	{Key: "subject", Label: "Subject"},
}

func pipelineFindingsPresentation(result *pipelineResult) *presentation.Table {
	rows := pipelineFindingRows(result)
	if len(rows) == 0 {
		return nil
	}
	return &presentation.Table{
		Title: "Findings", Columns: append([]presentation.Column(nil), pipelineFindingColumns...), Rows: rows,
	}
}

func pipelineFindingRows(result *pipelineResult) []map[string]any {
	findings := pipelineFindingCollection{seen: make(map[string]struct{})}
	appendPipelineVerificationFindings(&findings, result.Verification.Facts)
	runtimeFinding := appendPipelineJournalFindings(&findings, result)
	if appendPipelineDispatchFindings(&findings, result.Dispatch) {
		runtimeFinding = true
	}
	if appendPipelineLocalGitFindings(&findings, result.LocalGit) {
		runtimeFinding = true
	}
	if appendPipelineRecoveryFindings(&findings, result) {
		runtimeFinding = true
	}
	if appendPipelineManualFindings(&findings, result.ManualIntervention) {
		runtimeFinding = true
	}
	if !runtimeFinding {
		appendPipelineStatusFinding(&findings, result)
	}
	return findings.rows
}

type pipelineFindingCollection struct {
	seen map[string]struct{}
	rows []map[string]any
}

func (findings *pipelineFindingCollection) append(check, status, details, subject string) {
	check = strings.TrimSpace(check)
	status = strings.TrimSpace(status)
	details = strings.TrimSpace(details)
	subject = strings.TrimSpace(subject)
	if check == "" || status == "" || details == "" {
		return
	}
	key := strings.Join([]string{check, status, details, subject}, "\x00")
	if _, exists := findings.seen[key]; exists {
		return
	}
	findings.seen[key] = struct{}{}
	findings.rows = append(findings.rows, map[string]any{
		"check": check, "status": status, "status_role": string(pipelineFindingStatusRole(status)),
		"details": details, "subject": subject,
	})
}

func appendPipelineVerificationFindings(findings *pipelineFindingCollection, facts []VerificationFact) {
	for _, fact := range facts {
		if !actionableVerificationStatus(fact.Status) {
			continue
		}
		details := fact.Evidence
		if details == "" {
			details = humanVerificationStatus(fact.Status) + " verification result."
		}
		findings.append(humanVerificationCategory(fact.Category), humanVerificationStatus(fact.Status), details, fact.Subject)
	}
}

func appendPipelineJournalFindings(findings *pipelineFindingCollection, result *pipelineResult) bool {
	appended := false
	if result.Execution.Validity == "conflict" || result.Execution.UnresolvedCount > 1 {
		findings.append("Execution journals", "Invalid", "Multiple unresolved executions prevent a safe journal selection.", "")
		appended = true
	}
	for _, observation := range result.Execution.Observations {
		if observation.Valid && observation.Problem == "" {
			continue
		}
		details := firstPipelineFindingDetail(observation.Problem, "Execution journal evidence is invalid.")
		findings.append("Execution journal", "Invalid", details, observation.Reference)
		appended = true
	}
	for _, observation := range result.Dispatch.Observations {
		if observation.Valid && observation.Problem == "" {
			continue
		}
		details := firstPipelineFindingDetail(observation.Problem, "Dispatch journal evidence is invalid.")
		findings.append("Dispatch journal", "Invalid", details, observation.Reference)
		appended = true
	}
	return appended
}

func appendPipelineDispatchFindings(findings *pipelineFindingCollection, dispatch pipelineDispatch) bool {
	switch dispatch.State {
	case "rejected":
		findings.append("Workflow dispatch", "Rejected", "The workflow dispatch request was rejected.", dispatch.WorkflowPath)
		return true
	case "unknown":
		findings.append("Workflow dispatch", "Unknown", "The durable workflow dispatch outcome is unknown.", dispatch.WorkflowPath)
		return true
	default:
		return false
	}
}

func appendPipelineLocalGitFindings(findings *pipelineFindingCollection, localGit pipelineLocalGit) bool {
	appended := false
	if localGit.ExpectedCommit != "" {
		switch {
		case !localGit.CommitExists:
			findings.append("Expected release commit", "Missing", "The expected local release commit is missing.", localGit.ExpectedCommit)
			appended = true
		case !localGit.CommitContentVerified:
			findings.append("Expected release commit", "Mismatched", "The local release commit content does not match the recorded evidence.", localGit.ExpectedCommit)
			appended = true
		case !localGit.HeadContainsExpectedCommit:
			findings.append("Expected release commit", "Invalid", "The expected release commit is not reachable from local HEAD.", localGit.ExpectedCommit)
			appended = true
		}
	}
	if localGit.ExpectedTag != "" {
		switch {
		case !localGit.TagExists:
			findings.append("Expected unit tag", "Missing", "The expected local unit tag is missing.", localGit.ExpectedTag)
			appended = true
		case !localGit.TagMatchesExpectedCommit:
			findings.append("Expected unit tag", "Mismatched", "The local unit tag does not target the expected release commit.", localGit.ExpectedTag)
			appended = true
		}
	}
	if localGit.Problem != "" && !localGit.Consistent {
		findings.append("Local Git evidence", "Invalid", localGit.Problem, "")
		appended = true
	}
	return appended
}

func appendPipelineRecoveryFindings(findings *pipelineFindingCollection, result *pipelineResult) bool {
	if result.Recovery.Evaluated && (result.Status == pipelineBlocked || result.Status == pipelineUncertain) {
		status := "Blocked"
		fallback := "Recovery cannot continue safely without review."
		if result.Status == pipelineUncertain {
			status = "Uncertain"
			fallback = "Recovery evidence is uncertain and requires review."
		}
		details := firstPipelineFindingDetail(
			result.Recovery.Guidance, firstString(result.Recovery.Reasons),
			result.Recovery.ResumeRefusal, result.Recovery.Classification, fallback,
		)
		findings.append("Recovery", status, humanPipelineReason(details), "")
		return true
	}
	return false
}

func appendPipelineManualFindings(findings *pipelineFindingCollection, manual pipelineManualIntervention) bool {
	for _, reason := range manual.Reasons {
		findings.append("Manual intervention", "Required", humanPipelineReason(reason), "")
	}
	if manual.Required && len(manual.Reasons) == 0 {
		findings.append("Manual intervention", "Required", "Manual review is required before continuing.", "")
	}
	return len(manual.Reasons) > 0 || manual.Required
}

func appendPipelineStatusFinding(findings *pipelineFindingCollection, result *pipelineResult) {
	switch result.Status {
	case pipelineInvalid:
		findings.append("Runtime evidence", "Invalid", "Local runtime evidence could not be selected safely.", "")
	case pipelineBlocked:
		findings.append("Recovery", "Blocked", "Recovery cannot continue safely without review.", "")
	case pipelineUncertain:
		findings.append("Recovery", "Uncertain", "Recovery evidence is uncertain and requires review.", "")
	case pipelineRejected:
		findings.append("Workflow dispatch", "Rejected", "The workflow dispatch request was rejected.", result.Dispatch.WorkflowPath)
	}
}

func actionableVerificationStatus(status VerificationStatus) bool {
	switch status {
	case VerificationFailed, VerificationUnauthorized, VerificationRateLimited, VerificationUnavailable:
		return true
	default:
		return false
	}
}

func pipelineFindingStatusRole(status string) presentation.StyleRole {
	switch status {
	case "Failed", "Invalid", "Rejected", "Missing", "Mismatched":
		return presentation.StyleError
	case "Unauthorized", "Rate limited", "Unavailable", "Unknown", "Blocked", "Uncertain", "Required":
		return presentation.StyleWarning
	default:
		return presentation.StyleDefault
	}
}

func firstPipelineFindingDetail(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
