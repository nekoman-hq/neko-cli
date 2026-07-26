package evidence

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func attachEvidencePresentation(
	response *plugin.Response,
	request evidenceQueryRequest,
	result evidenceQueryResult,
) {
	if response == nil {
		return
	}
	response.PresentationProperties = &presentation.Properties{
		Title:      "Evidence Summary",
		Properties: evidenceSummaryProperties(request, result),
	}
	response.PresentationTable = evidencePresentationTables(result)
}

func evidenceSummaryProperties(request evidenceQueryRequest, result evidenceQueryResult) []presentation.Property {
	status := "No evidence found"
	nextAction := "No release evidence files matched the selected scope."
	role := presentation.StyleMuted
	if len(result.Records) > 0 {
		status = "Evidence available"
		nextAction = "Review actionable classifications before continuing or archiving evidence."
		role = presentation.StyleInfo
	}
	if evidenceHasActionableFindings(result) {
		status = "Action required"
		nextAction = "Follow each actionable finding; preserve uncertain or conflicting evidence."
		role = presentation.StyleWarning
	}
	return []presentation.Property{
		{Label: "Scope", Value: evidenceScopeLabel(request), Emphasized: true},
		{Label: "Family filter", Value: evidenceFilterValue(request.Family)},
		{Label: "Unit filter", Value: evidenceFilterValue(request.Unit)},
		{Label: "Identity filter", Value: evidenceFilterValue(request.IdentityPrefix)},
		{Label: "Evidence records", Value: fmt.Sprintf("%d", len(result.Records))},
		{Label: "Diagnostics", Value: fmt.Sprintf("%d", len(result.Diagnostics))},
		{Label: "Status", Value: status, Role: role, Emphasized: true},
		{Label: "Next action", Value: nextAction, Emphasized: true},
	}
}

func evidenceScopeLabel(request evidenceQueryRequest) string {
	filters := make([]string, 0, 3)
	if request.Family != "" {
		filters = append(filters, "family="+request.Family)
	}
	if request.Unit != "" {
		filters = append(filters, "unit="+request.Unit)
	}
	if request.IdentityPrefix != "" {
		filters = append(filters, "identity="+request.IdentityPrefix)
	}
	if len(filters) == 0 {
		return "All local release evidence"
	}
	return strings.Join(filters, ", ")
}

func evidenceFilterValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "All"
	}
	return value
}

func evidenceHasActionableFindings(result evidenceQueryResult) bool {
	if len(result.Diagnostics) > 0 {
		return true
	}
	for _, record := range result.Records {
		if record.Family == FamilyDispatch {
			linkage := evidenceLinkedExecution(result, record)
			if linkage == "unlinked" || linkage == "ambiguous" {
				return true
			}
		}
		switch record.Classification {
		case ClassificationCompleted:
			continue
		default:
			return true
		}
	}
	return false
}

func evidencePresentationTables(result evidenceQueryResult) *presentation.Table {
	tables := []*presentation.Table{
		evidenceInventoryTable(result),
		evidenceFamilyDetailTable("Execution Evidence", FamilyReleaseExecution, result),
		evidenceFamilyDetailTable("Dispatch Evidence", FamilyDispatch, result),
		evidenceOtherDetailTable(result),
		evidenceLinkageTable(result),
		evidenceLocalGitTable(result),
		evidenceClassificationTable(result),
		evidenceRecoveryTable(result),
		evidenceLimitationsTable(),
	}
	return chainEvidenceTables(tables)
}

func evidenceInventoryTable(result evidenceQueryResult) *presentation.Table {
	return &presentation.Table{
		Title: "Evidence Inventory",
		Columns: []presentation.Column{
			{Key: "family", Label: "Family", Essential: true},
			{Key: "identity", Label: "Identity", Essential: true},
			{Key: "state", Label: "State", Essential: true},
			{Key: "classification", Label: "Classification", Essential: true},
			{Key: "action", Label: "Action", Essential: true},
			{Key: "unit", Label: "Unit"},
			{Key: "version", Label: "Version"},
			{Key: "tag", Label: "Tag"},
			{Key: "linked_execution", Label: "Linked execution"},
		},
		Rows: evidenceInventoryRows(result),
		Note: "Default output keeps every classification and action; --describe adds safe forensic detail.",
	}
}

func evidenceInventoryRows(result evidenceQueryResult) []map[string]any {
	rows := make([]map[string]any, 0, len(result.Records)+len(result.Diagnostics)+1)
	for _, record := range result.Records {
		rows = append(rows, map[string]any{
			"family": record.Family, "identity": record.Identity,
			"state": evidenceValue(record.State), "classification": record.Classification,
			"action": evidenceInventoryAction(result, record), "unit": evidenceValue(record.Unit),
			"version": evidenceValue(record.Version), "tag": evidenceValue(record.Tag),
			"linked_execution": evidenceLinkedExecution(result, record),
		})
	}
	for _, diagnostic := range result.Diagnostics {
		rows = append(rows, map[string]any{
			"family": diagnostic.Family, "identity": evidenceDiagnosticIdentity(diagnostic),
			"state": diagnostic.Code, "classification": diagnostic.Classification,
			"action": diagnostic.Guidance, "unit": "not applicable",
			"version": "not applicable", "tag": "not applicable",
			"linked_execution": "not applicable",
		})
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]any{
			"family": "none", "identity": "not applicable", "state": "missing",
			"classification": ClassificationCompleted,
			"action":         "No release evidence files were found.",
		})
	}
	return rows
}

func evidenceFamilyDetailTable(title, family string, result evidenceQueryResult) *presentation.Table {
	rows := make([]map[string]any, 0)
	for _, record := range result.Records {
		if record.Family != family {
			continue
		}
		rows = append(rows, evidenceDetailRow(record))
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Family != family {
			continue
		}
		rows = append(rows, evidenceDiagnosticDetailRow(diagnostic))
	}
	if len(rows) == 0 {
		return nil
	}
	return &presentation.Table{
		Title: title, DescribeOnly: true,
		Columns: evidenceDetailColumns(),
		Rows:    rows,
	}
}

func evidenceOtherDetailTable(result evidenceQueryResult) *presentation.Table {
	rows := make([]map[string]any, 0)
	for _, record := range result.Records {
		if record.Family == FamilyReleaseExecution || record.Family == FamilyDispatch {
			continue
		}
		rows = append(rows, evidenceDetailRow(record))
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Family == FamilyReleaseExecution || diagnostic.Family == FamilyDispatch {
			continue
		}
		rows = append(rows, evidenceDiagnosticDetailRow(diagnostic))
	}
	if len(rows) == 0 {
		return nil
	}
	return &presentation.Table{
		Title: "Other Evidence", DescribeOnly: true,
		Columns: evidenceDetailColumns(),
		Rows:    rows,
	}
}

func evidenceDetailColumns() []presentation.Column {
	return []presentation.Column{
		{Key: "family", Label: "Family", Essential: true},
		{Key: "identity", Label: "Identity", Essential: true},
		{Key: "state", Label: "State", Essential: true},
		{Key: "classification", Label: "Classification", Essential: true},
		{Key: "unit", Label: "Unit"},
		{Key: "version", Label: "Version"},
		{Key: "tag", Label: "Tag"},
		{Key: "pending", Label: "Pending action"},
		{Key: "path", Label: "Source path"},
		{Key: "digest", Label: "Digest SHA-256"},
	}
}

func evidenceDetailRow(record EvidenceRecord) map[string]any {
	return map[string]any{
		"family": record.Family, "identity": record.Identity,
		"state": evidenceValue(record.State), "classification": record.Classification,
		"unit": evidenceValue(record.Unit), "version": evidenceValue(record.Version),
		"tag": evidenceValue(record.Tag), "pending": evidenceValue(record.PendingAction),
		"path": safeEvidenceHumanPath(record.Path), "digest": record.DigestSHA256,
	}
}

func evidenceDiagnosticDetailRow(diagnostic EvidenceDiagnostic) map[string]any {
	return map[string]any{
		"family": diagnostic.Family, "identity": evidenceDiagnosticIdentity(diagnostic),
		"state": diagnostic.Code, "classification": diagnostic.Classification,
		"unit": "not applicable", "version": "not applicable",
		"tag": "not applicable", "pending": "manual-inspection",
		"path": safeEvidenceHumanPath(diagnostic.Path), "digest": "not available",
	}
}

func evidenceLinkageTable(result evidenceQueryResult) *presentation.Table {
	rows := make([]map[string]any, 0)
	for _, record := range result.Records {
		if record.Family != FamilyDispatch {
			continue
		}
		rows = append(rows, map[string]any{
			"dispatch":  record.Identity,
			"execution": evidenceLinkedExecution(result, record),
			"status":    evidenceLinkageStatus(result, record),
			"reason":    "Linkage is reported only when retained by the inspected evidence inventory.",
		})
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]any{
			"dispatch": "none", "execution": "not applicable",
			"status": "No dispatch evidence", "reason": "No dispatch journal matched the selected scope.",
		})
	}
	return &presentation.Table{
		Title: "Linkage", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "dispatch", Label: "Dispatch identity", Essential: true},
			{Key: "execution", Label: "Execution identity", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "reason", Label: "Reason"},
		},
		Rows: rows,
	}
}

func evidenceLinkedExecution(result evidenceQueryResult, dispatch EvidenceRecord) string {
	if dispatch.Family != FamilyDispatch {
		return "not applicable"
	}
	matches := make([]string, 0, 1)
	for _, execution := range result.Records {
		if execution.Family != FamilyReleaseExecution {
			continue
		}
		if execution.dispatchIdentity == dispatch.Identity {
			matches = append(matches, execution.Identity)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		return "ambiguous"
	}
	return "unlinked"
}

func evidenceInventoryAction(result evidenceQueryResult, record EvidenceRecord) string {
	if record.Family == FamilyDispatch && evidenceLinkedExecution(result, record) == "unlinked" {
		return "Dispatch evidence is unlinked from the selected execution inventory. Preserve it and inspect the exact identity. " + record.Guidance
	}
	if record.Family == FamilyDispatch && evidenceLinkedExecution(result, record) == "ambiguous" {
		return "Multiple executions reference this dispatch identity. Preserve the conflicting evidence and recover manually. " + record.Guidance
	}
	return record.Guidance
}

func evidenceLinkageStatus(result evidenceQueryResult, dispatch EvidenceRecord) string {
	switch evidenceLinkedExecution(result, dispatch) {
	case "unlinked":
		return "Unlinked"
	case "ambiguous":
		return "Ambiguous"
	default:
		return "Linked"
	}
}

func evidenceLocalGitTable(result evidenceQueryResult) *presentation.Table {
	rows := make([]map[string]any, 0)
	for _, record := range result.Records {
		if record.Family != FamilyReleaseExecution {
			continue
		}
		rows = append(rows,
			map[string]any{
				"identity": record.Identity, "evidence": "Release commit",
				"status": evidenceJournalGitStatus(record.releaseCommitSHA),
				"value":  evidenceValue(record.releaseCommitSHA),
			},
			map[string]any{
				"identity": record.Identity, "evidence": "Unit tag",
				"status": evidenceJournalGitStatus(record.tagTargetSHA),
				"value":  evidenceTagJournalValue(record),
			},
		)
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]any{
			"identity": "not applicable", "evidence": "Local Git",
			"status": "No execution evidence", "value": "No local Git claim is made.",
		})
	}
	return &presentation.Table{
		Title: "Local Git Evidence", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "identity", Label: "Execution identity", Essential: true},
			{Key: "evidence", Label: "Evidence", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "value", Label: "Value"},
		},
		Rows: rows,
	}
}

func evidenceJournalGitStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Not recorded in journal"
	}
	return "Recorded in journal; local Git not re-inspected"
}

func evidenceTagJournalValue(record EvidenceRecord) string {
	if strings.TrimSpace(record.tagTargetSHA) == "" {
		return evidenceValue(record.Tag)
	}
	return record.Tag + " → " + record.tagTargetSHA
}

func evidenceClassificationTable(result evidenceQueryResult) *presentation.Table {
	rows := make([]map[string]any, 0, len(result.Records)+len(result.Diagnostics))
	for _, record := range result.Records {
		rows = append(rows, map[string]any{
			"identity": record.Identity, "classification": record.Classification,
			"reason": record.Guidance,
		})
	}
	for _, diagnostic := range result.Diagnostics {
		rows = append(rows, map[string]any{
			"identity":       evidenceDiagnosticIdentity(diagnostic),
			"classification": diagnostic.Classification, "reason": diagnostic.Guidance,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]any{
			"identity": "not applicable", "classification": ClassificationCompleted,
			"reason": "No evidence files were found.",
		})
	}
	return &presentation.Table{
		Title: "Classification", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "identity", Label: "Identity", Essential: true},
			{Key: "classification", Label: "Classification", Essential: true},
			{Key: "reason", Label: "Reason", Essential: true},
		},
		Rows: rows,
	}
}

func evidenceRecoveryTable(result evidenceQueryResult) *presentation.Table {
	rows := make([]map[string]any, 0, len(result.Records)+len(result.Diagnostics))
	for _, record := range result.Records {
		rows = append(rows, map[string]any{
			"identity":  record.Identity,
			"resume":    fmt.Sprintf("%t", record.SafeToResume),
			"automatic": fmt.Sprintf("%t", record.AutomaticContinuation),
			"manual":    fmt.Sprintf("%t", record.ManualRecovery),
			"lifecycle": lifecycleEvidenceValue(record),
			"guidance":  record.Guidance,
		})
	}
	for _, diagnostic := range result.Diagnostics {
		rows = append(rows, map[string]any{
			"identity": evidenceDiagnosticIdentity(diagnostic),
			"resume":   "false", "automatic": "false", "manual": "true",
			"lifecycle": "blocked", "guidance": diagnostic.Guidance,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]any{
			"identity": "not applicable", "resume": "false", "automatic": "false",
			"manual": "false", "lifecycle": "not applicable",
			"guidance": "No evidence files were found.",
		})
	}
	return &presentation.Table{
		Title: "Recovery Relevance", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "identity", Label: "Identity", Essential: true},
			{Key: "resume", Label: "Resume eligible", Essential: true},
			{Key: "manual", Label: "Manual recovery", Essential: true},
			{Key: "guidance", Label: "Guidance", Essential: true},
			{Key: "automatic", Label: "Automatic continuation"},
			{Key: "lifecycle", Label: "Archive lifecycle"},
		},
		Rows: rows,
	}
}

func evidenceLimitationsTable() *presentation.Table {
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "limitation", Label: "Limitation", Essential: true},
			{Key: "statement", Label: "Statement", Essential: true},
		},
		Rows: []map[string]any{
			{
				"limitation": "Read-only boundary",
				"statement":  "Evidence inspection does not resume, repair, retry, archive, or mutate journals.",
			},
			{
				"limitation": "Remote evidence",
				"statement":  "Remote Git and workflow state are not inspected.",
			},
			{
				"limitation": "Local Git ownership",
				"statement":  "Local recovery verification remains owned by authoritative Resume and Pipeline capabilities.",
			},
		},
	}
}

func chainEvidenceTables(tables []*presentation.Table) *presentation.Table {
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

func evidenceDiagnosticIdentity(diagnostic EvidenceDiagnostic) string {
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(diagnostic.Path)), filepath.Ext(diagnostic.Path))
	if safeEvidenceHash(base) {
		return base
	}
	return "diagnostic"
}

func safeEvidenceHumanPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	for _, marker := range []string{"/.git/", "/.neko/"} {
		if index := strings.LastIndex(path, marker); index >= 0 {
			return path[index+1:]
		}
	}
	if path == "." || path == "" {
		return "not applicable"
	}
	if filepath.IsAbs(path) {
		return "repository-local evidence"
	}
	return path
}

func safeEvidenceArchiveHumanPath(path string) string {
	safePath := safeEvidenceHumanPath(path)
	extension := filepath.Ext(safePath)
	stem := strings.TrimSuffix(safePath, extension)
	separator := strings.LastIndex(stem, "-")
	if separator < 0 {
		return safePath
	}
	digest := stem[separator+1:]
	if !safeEvidenceHash(digest) {
		return safePath
	}
	return stem[:separator+1] + abbreviatedEvidenceHash(digest) + extension
}

func evidenceValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "not applicable"
	}
	return value
}

func attachEvidenceArchivePresentation(response *plugin.Response, result evidenceArchiveResult) {
	if response == nil {
		return
	}
	response.PresentationProperties = &presentation.Properties{
		Title: "Evidence Archive Result",
		Properties: []presentation.Property{
			{Label: "Family", Value: result.Family},
			{Label: "Identity", Value: result.Identity, Emphasized: true},
			{Label: "Confirmation", Value: "Confirmed", Role: presentation.StyleSuccess},
			{Label: "Digest verification", Value: "Matched", Role: presentation.StyleSuccess},
			{Label: "Archive result", Value: "Archived", Role: presentation.StyleSuccess, Emphasized: true},
			{Label: "Source", Value: safeEvidenceHumanPath(result.SourcePath)},
			{Label: "Archive target", Value: safeEvidenceArchiveHumanPath(result.ArchivePath)},
			{Label: "Next action", Value: "Retain the archive for audit; no repository or Git state changed."},
		},
	}
	response.PresentationTable = chainEvidenceTables([]*presentation.Table{
		evidenceArchiveValidationTable(result),
		evidenceArchivePlanTable(result),
		evidenceArchiveLimitationsTable(),
	})
}

func evidenceArchiveValidationTable(result evidenceArchiveResult) *presentation.Table {
	digest := abbreviatedEvidenceHash(result.DigestSHA256)
	return &presentation.Table{
		Title: "Archive Validation", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "guard", Label: "Guard", Essential: true},
			{Key: "result", Label: "Result", Essential: true},
			{Key: "evidence", Label: "Evidence"},
		},
		Rows: []map[string]any{
			{"guard": "Family", "result": "Supported", "evidence": result.Family},
			{"guard": "Identity", "result": "Exact match", "evidence": result.Identity},
			{"guard": "Expected digest", "result": "Matched", "evidence": digest},
			{"guard": "Actual digest", "result": "Matched", "evidence": digest},
			{"guard": "Source classification", "result": ClassificationCompleted, "evidence": "archive-completed"},
			{"guard": "Explicit confirmation", "result": "Confirmed", "evidence": "--confirm-archive"},
		},
	}
}

func evidenceArchivePlanTable(result evidenceArchiveResult) *presentation.Table {
	return &presentation.Table{
		Title: "Guarded Write Plan", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "step", Label: "Step", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "path", Label: "Path"},
		},
		Rows: []map[string]any{
			{"step": "Read selected evidence", "status": "Verified", "path": safeEvidenceHumanPath(result.SourcePath)},
			{"step": "Write exact private archive", "status": "Completed", "path": safeEvidenceArchiveHumanPath(result.ArchivePath)},
			{"step": "Verify archive bytes", "status": "Completed", "path": abbreviatedEvidenceHash(result.DigestSHA256)},
			{"step": "Remove selected source", "status": "Completed", "path": safeEvidenceHumanPath(result.SourcePath)},
		},
	}
}

func evidenceArchiveLimitationsTable() *presentation.Table {
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "limitation", Label: "Limitation", Essential: true},
			{Key: "statement", Label: "Statement", Essential: true},
		},
		Rows: []map[string]any{
			{
				"limitation": "Selected evidence only",
				"statement":  "The guarded operation archives one exact completed evidence identity.",
			},
			{
				"limitation": "No recovery mutation",
				"statement":  "Archival does not resume, repair, retry, commit, tag, push, or dispatch.",
			},
		},
	}
}

func abbreviatedEvidenceHash(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return evidenceValue(value)
	}
	return value[:12]
}
