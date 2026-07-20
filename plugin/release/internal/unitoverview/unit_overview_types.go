package unitoverview

import "sort"

const unitOverviewCommandName = "units"

type unitOverviewStatus string

const (
	unitOverviewValid         unitOverviewStatus = "valid"
	unitOverviewHasIssues     unitOverviewStatus = "has_issues"
	unitOverviewSourceInvalid unitOverviewStatus = "source_invalid"
)

type unitOverviewAlignment string

const (
	unitOverviewAligned    unitOverviewAlignment = "aligned"
	unitOverviewConfigOnly unitOverviewAlignment = "config_only"
	unitOverviewStateOnly  unitOverviewAlignment = "state_only"
	unitOverviewInvalid    unitOverviewAlignment = "invalid"
)

type unitOverviewIssueSeverity string

const (
	unitOverviewIssueError   unitOverviewIssueSeverity = "error"
	unitOverviewIssueWarning unitOverviewIssueSeverity = "warning"
)

type unitOverviewRequest struct {
	RepositoryRoot string
}

type unitOverviewIssue struct {
	Severity    unitOverviewIssueSeverity `json:"severity"`
	Unit        string                    `json:"unit,omitempty"`
	Code        string                    `json:"code"`
	Message     string                    `json:"message"`
	Remediation string                    `json:"remediation"`
}

//nolint:govet // Field order follows the stable machine contract.
type unitOverviewRow struct {
	ID                string                `json:"id"`
	DisplayName       string                `json:"display_name,omitempty"`
	Version           string                `json:"version,omitempty"`
	ConfiguredVersion string                `json:"configured_version,omitempty"`
	TagPrefix         string                `json:"tag_prefix,omitempty"`
	TagShape          string                `json:"tag_shape,omitempty"`
	ConfiguredTag     string                `json:"configured_tag,omitempty"`
	Executor          string                `json:"executor,omitempty"`
	Delivery          string                `json:"delivery,omitempty"`
	WorkflowPath      string                `json:"workflow_path,omitempty"`
	WorkingDirectory  string                `json:"working_directory,omitempty"`
	Alignment         unitOverviewAlignment `json:"alignment"`
	Issues            []unitOverviewIssue   `json:"issues"`
	ConfigPresent     bool                  `json:"-"`
	StatePresent      bool                  `json:"-"`
}

type unitOverviewSummary struct {
	Total         int  `json:"total"`
	Aligned       int  `json:"aligned"`
	Incomplete    int  `json:"incomplete"`
	Invalid       int  `json:"invalid"`
	WorkflowPaths int  `json:"workflow_paths"`
	SourceUsable  bool `json:"source_usable"`
}

//nolint:govet // Field order follows the stable machine contract.
type unitOverviewResult struct {
	Status        unitOverviewStatus  `json:"status"`
	Summary       unitOverviewSummary `json:"summary"`
	Units         []unitOverviewRow   `json:"units"`
	WorkflowPaths []string            `json:"workflow_paths"`
	SourceIssue   *unitOverviewIssue  `json:"source_issue,omitempty"`
	SourceUsable  bool                `json:"-"`
}

func finalizeUnitOverviewResult(result *unitOverviewResult) {
	if result == nil {
		return
	}
	sort.SliceStable(result.Units, func(left, right int) bool {
		return result.Units[left].ID < result.Units[right].ID
	})
	workflowPaths := map[string]struct{}{}
	result.Summary = unitOverviewSummary{Total: len(result.Units), SourceUsable: result.SourceUsable}
	for index := range result.Units {
		row := &result.Units[index]
		sortUnitOverviewIssues(row.Issues)
		switch row.Alignment {
		case unitOverviewAligned:
			result.Summary.Aligned++
		case unitOverviewConfigOnly, unitOverviewStateOnly:
			result.Summary.Incomplete++
		case unitOverviewInvalid:
			result.Summary.Invalid++
		}
		if row.WorkflowPath != "" {
			workflowPaths[row.WorkflowPath] = struct{}{}
		}
	}
	result.WorkflowPaths = make([]string, 0, len(workflowPaths))
	for path := range workflowPaths {
		result.WorkflowPaths = append(result.WorkflowPaths, path)
	}
	sort.Strings(result.WorkflowPaths)
	result.Summary.WorkflowPaths = len(result.WorkflowPaths)
	switch {
	case result.SourceIssue != nil:
		result.Status = unitOverviewSourceInvalid
	case result.Summary.Incomplete > 0 || result.Summary.Invalid > 0:
		result.Status = unitOverviewHasIssues
	default:
		result.Status = unitOverviewValid
	}
}

func sortUnitOverviewIssues(issues []unitOverviewIssue) {
	sort.SliceStable(issues, func(left, right int) bool {
		if unitOverviewIssueSeverityRank(issues[left].Severity) != unitOverviewIssueSeverityRank(issues[right].Severity) {
			return unitOverviewIssueSeverityRank(issues[left].Severity) < unitOverviewIssueSeverityRank(issues[right].Severity)
		}
		if issues[left].Unit != issues[right].Unit {
			return issues[left].Unit < issues[right].Unit
		}
		if issues[left].Code != issues[right].Code {
			return issues[left].Code < issues[right].Code
		}
		return issues[left].Message < issues[right].Message
	})
}

func unitOverviewIssueSeverityRank(severity unitOverviewIssueSeverity) int {
	if severity == unitOverviewIssueError {
		return 0
	}
	if severity == unitOverviewIssueWarning {
		return 1
	}
	return 2
}
