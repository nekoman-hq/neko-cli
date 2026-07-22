package pipelineinspection

import (
	"strconv"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func chainPipelineTables(tables ...*presentation.Table) *presentation.Table {
	var first *presentation.Table
	var previous *presentation.Table
	for _, table := range tables {
		if table == nil {
			continue
		}
		if first == nil {
			first = table
		}
		if previous != nil {
			previous.Following = table
		}
		previous = table
	}
	return first
}

func pipelineExecutionEvidencePresentation(result *pipelineResult) *presentation.Table {
	if !result.Execution.Present && result.Execution.JournalCount == 0 && len(result.Execution.Observations) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(result.Execution.Observations))
	for index, observation := range result.Execution.Observations {
		validity := "Valid"
		if !observation.Valid {
			validity = "Invalid"
		} else if observation.Unresolved {
			validity = "Unresolved"
		}
		selection := ""
		if result.Execution.Identity != "" && observation.Identity == result.Execution.Identity {
			selection = "Selected"
		}
		rows = append(rows, map[string]any{
			"journal": index + 1, "state": humanMachineValue(observation.State), "validity": validity,
			"selection": selection, "identity": observation.Identity, "reference": observation.Reference,
			"problem": observation.Problem,
		})
	}
	return &presentation.Table{
		Title: "Execution Evidence", DescribeOnly: true, Rows: rows,
		Columns: []presentation.Column{
			{Key: "journal", Label: "Journal", Essential: true},
			{Key: "state", Label: "State", Essential: true},
			{Key: "validity", Label: "Validity", Essential: true},
			{Key: "selection", Label: "Selection"},
			{Key: "identity", Label: "Identity"},
			{Key: "reference", Label: "Reference"},
			{Key: "problem", Label: "Problem"},
		},
	}
}

func pipelineDispatchEvidencePresentation(result *pipelineResult) *presentation.Table {
	if !result.Dispatch.Present && result.Dispatch.JournalCount == 0 && len(result.Dispatch.Observations) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(result.Dispatch.Observations))
	for index, observation := range result.Dispatch.Observations {
		validity := "Valid"
		if !observation.Valid {
			validity = "Invalid"
		}
		selection := ""
		if result.Dispatch.Identity != "" && observation.Identity == result.Dispatch.Identity {
			selection = "Selected"
		}
		rows = append(rows, map[string]any{
			"journal": index + 1, "state": humanMachineValue(observation.State), "validity": validity,
			"correlation": humanMachineValue(observation.Correlation), "selection": selection,
			"identity": observation.Identity, "reference": observation.Reference, "problem": observation.Problem,
		})
	}
	return &presentation.Table{
		Title: "Dispatch Evidence", DescribeOnly: true, Rows: rows,
		Columns: []presentation.Column{
			{Key: "journal", Label: "Journal", Essential: true},
			{Key: "state", Label: "State", Essential: true},
			{Key: "validity", Label: "Validity", Essential: true},
			{Key: "correlation", Label: "Correlation"},
			{Key: "selection", Label: "Selection"},
			{Key: "identity", Label: "Identity"},
			{Key: "reference", Label: "Reference"},
			{Key: "problem", Label: "Problem"},
		},
	}
}

func pipelineLocalGitEvidencePresentation(result *pipelineResult) *presentation.Table {
	if result.LocalGit.Scope == "" {
		return nil
	}
	rows := []map[string]any{
		{"fact": "Inspection scope", "value": humanPipelineLocalGit(result)},
		{"fact": "Remote freshness", "value": humanPipelineRemoteGit(result.LocalGit)},
	}
	appendEvidenceValue := func(label, value string) {
		if value != "" {
			rows = append(rows, map[string]any{"fact": label, "value": value})
		}
	}
	appendEvidenceValue("Branch", result.LocalGit.Branch)
	appendEvidenceValue("HEAD", result.LocalGit.Head)
	appendEvidenceValue("Index", humanMachineValue(result.LocalGit.IndexState))
	appendEvidenceValue("Worktree", humanMachineValue(result.LocalGit.WorktreeState))
	if result.LocalGit.ExpectedCommit != "" {
		appendEvidenceValue("Expected commit", result.LocalGit.ExpectedCommit)
		appendEvidenceValue("Commit exists", humanBoolean(result.LocalGit.CommitExists))
		appendEvidenceValue("Commit content verified", humanBoolean(result.LocalGit.CommitContentVerified))
		appendEvidenceValue("HEAD contains commit", humanBoolean(result.LocalGit.HeadContainsExpectedCommit))
	}
	if result.LocalGit.ExpectedTag != "" {
		appendEvidenceValue("Expected tag", result.LocalGit.ExpectedTag)
		appendEvidenceValue("Tag exists", humanBoolean(result.LocalGit.TagExists))
		appendEvidenceValue("Tag target", result.LocalGit.TagTarget)
		appendEvidenceValue("Tag matches commit", humanBoolean(result.LocalGit.TagMatchesExpectedCommit))
	}
	appendEvidenceValue("Problem", result.LocalGit.Problem)
	return pipelineFactValueTable("Local Git Evidence", rows)
}

func pipelineRecoveryEvidencePresentation(result *pipelineResult) *presentation.Table {
	if !result.Recovery.Evaluated && len(result.Recovery.Reasons) == 0 && result.Recovery.Guidance == "" && !result.ManualIntervention.Required {
		return nil
	}
	rows := []map[string]any{
		{"fact": "Classification", "value": humanMachineValue(result.Recovery.Classification)},
		{"fact": "Safe to continue", "value": humanBoolean(result.Recovery.SafeToContinue)},
		{"fact": "Resume eligible", "value": humanBoolean(result.Recovery.ResumeEligible)},
		{"fact": "Retry safety", "value": humanMachineValue(result.Recovery.RetrySafety)},
	}
	appendRecoveryValue := func(label, value string) {
		if value != "" {
			rows = append(rows, map[string]any{"fact": label, "value": humanPipelineReason(value)})
		}
	}
	appendRecoveryValue("Resume operation", result.Recovery.ResumeOperation)
	appendRecoveryValue("Resume refusal", result.Recovery.ResumeRefusal)
	appendRecoveryValue("Guidance", result.Recovery.Guidance)
	for index, reason := range result.Recovery.Reasons {
		appendRecoveryValue("Recovery reason "+strconv.Itoa(index+1), reason)
	}
	for index, reason := range result.ManualIntervention.Reasons {
		appendRecoveryValue("Manual reason "+strconv.Itoa(index+1), reason)
	}
	return pipelineFactValueTable("Recovery", rows)
}

func pipelineFactValueTable(title string, rows []map[string]any) *presentation.Table {
	return &presentation.Table{
		Title: title, DescribeOnly: true, Rows: rows,
		Columns: []presentation.Column{
			{Key: "fact", Label: "Fact", Essential: true},
			{Key: "value", Label: "Value", Essential: true},
		},
	}
}

func humanBoolean(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}
