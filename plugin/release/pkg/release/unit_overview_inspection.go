package release

import (
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func deriveCanonicalUnitOverviewRows(repository *releaseconfig.ReleaseRepository) []unitOverviewRow {
	rows := make([]unitOverviewRow, 0, len(repository.Units))
	for _, unit := range repository.Units {
		row := unitOverviewRow{
			ID: unit.ID, DisplayName: unit.DisplayName,
			Version: unit.Version, ConfiguredVersion: unit.Version,
			TagPrefix: unit.TagPrefix, Executor: unit.ExecutorType,
			Delivery: unit.Delivery, WorkflowPath: unit.Workflow,
			WorkingDirectory: unit.WorkingDirectory,
			Alignment:        unitOverviewAligned, Issues: make([]unitOverviewIssue, 0),
			ConfigPresent: true, StatePresent: true,
		}
		if tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix); err == nil {
			row.TagShape = tagSpec.Format("<version>")
			row.ConfiguredTag = tagSpec.Format(unit.Version)
		}
		rows = append(rows, row)
	}
	return rows
}

func derivePartialUnitOverviewRows(
	config *releaseconfig.V2ReleaseConfig,
	state *releaseconfig.V2ReleaseState,
) []unitOverviewRow {
	rows := make([]unitOverviewRow, 0)
	stateUnits := map[string]releaseconfig.V2UnitState{}
	if state != nil && state.Units != nil {
		stateUnits = state.Units
	}
	usedStateUnits := map[string]struct{}{}
	configUnits := []releaseconfig.V2Unit(nil)
	if config != nil {
		configUnits = config.Units
	}
	configValid := make([]bool, len(configUnits))
	for index, unit := range configUnits {
		row := newConfiguredUnitOverviewRow(unit)
		unitState, statePresent := stateUnits[unit.ID]
		if statePresent {
			usedStateUnits[unit.ID] = struct{}{}
			row.StatePresent = true
			applyUnitOverviewVersion(&row, unitState.Version)
		} else {
			appendUnitOverviewIssue(&row, unitOverviewIssueWarning, "UNIT_STATE_MISSING", "The unit has config but no state entry.", "Add the unit's current canonical version to .neko/release.state.json.")
		}

		if err := validateOneUnitOverviewConfig(unit); err != nil {
			code, message, remediation := classifyUnitOverviewConfigError(err)
			appendUnitOverviewIssue(&row, unitOverviewIssueError, code, message, remediation)
		} else {
			configValid[index] = true
		}
		applyUnitOverviewTagFacts(&row)
		row.Alignment = classifyUnitOverviewAlignment(row)
		rows = append(rows, row)
	}

	appendUnitOverviewConfigConflicts(rows, configUnits, configValid)
	for index := range rows {
		rows[index].Alignment = classifyUnitOverviewAlignment(rows[index])
	}

	stateOnlyIDs := make([]string, 0)
	for id := range stateUnits {
		if _, used := usedStateUnits[id]; !used {
			stateOnlyIDs = append(stateOnlyIDs, id)
		}
	}
	sort.Strings(stateOnlyIDs)
	for _, id := range stateOnlyIDs {
		row := unitOverviewRow{
			ID: id, Alignment: unitOverviewStateOnly,
			Issues: make([]unitOverviewIssue, 0), StatePresent: true,
		}
		appendUnitOverviewIssue(&row, unitOverviewIssueWarning, "UNIT_CONFIG_MISSING", "The unit has state but no config entry.", "Add the unit's canonical release metadata to .neko/release.config.json or remove stale state.")
		applyUnitOverviewVersion(&row, stateUnits[id].Version)
		row.Alignment = classifyUnitOverviewAlignment(row)
		rows = append(rows, row)
	}
	return rows
}

func newConfiguredUnitOverviewRow(unit releaseconfig.V2Unit) unitOverviewRow {
	workingDirectory := unit.WorkingDirectory
	if workingDirectory == "" {
		workingDirectory = "."
	}
	return unitOverviewRow{
		ID: unit.ID, DisplayName: unit.DisplayName,
		TagPrefix: unit.TagPrefix, Executor: string(unit.Executor.Type),
		Delivery: string(unit.Executor.Delivery), WorkflowPath: unit.Executor.Workflow,
		WorkingDirectory: workingDirectory,
		Alignment:        unitOverviewConfigOnly, Issues: make([]unitOverviewIssue, 0),
		ConfigPresent: true,
	}
}

func applyUnitOverviewVersion(row *unitOverviewRow, configured string) {
	row.ConfiguredVersion = configured
	version, err := releaseconfig.CanonicalReleaseVersion(configured)
	if err != nil {
		appendUnitOverviewIssue(row, unitOverviewIssueError, "UNIT_VERSION_INVALID", "The state version is not canonical semantic versioning.", "Set the unit state version to a canonical semantic version.")
		return
	}
	row.Version = version
}

func applyUnitOverviewTagFacts(row *unitOverviewRow) {
	if hasUnitOverviewIssue(row.Issues, "UNIT_TAG_PREFIX_INVALID") {
		return
	}
	tagSpec, err := canonicalUnitOverviewTagSpec(row.TagPrefix)
	if err != nil {
		appendUnitOverviewIssue(row, unitOverviewIssueError, "UNIT_TAG_PREFIX_INVALID", "The configured tag prefix is not canonical.", "Use a safe, non-empty repository-relative tag prefix.")
		return
	}
	row.TagShape = tagSpec.Format("<version>")
	if row.Version != "" {
		row.ConfiguredTag = tagSpec.Format(row.Version)
	}
}

func canonicalUnitOverviewTagSpec(prefix string) (releaseconfig.TagSpec, error) {
	probe := releaseconfig.V2Unit{
		ID: "overview", Paths: []string{"**"}, TagPrefix: prefix,
		Executor: releaseconfig.V2Executor{
			Type: releaseconfig.ExecutorGoReleaser, Delivery: releaseconfig.DeliveryGitHubActions,
			Workflow: ".github/workflows/release.yml",
		},
	}
	if err := validateOneUnitOverviewConfig(probe); err != nil {
		return releaseconfig.TagSpec{}, err
	}
	return releaseconfig.NewTagSpec(prefix)
}

func validateOneUnitOverviewConfig(unit releaseconfig.V2Unit) error {
	return releaseconfig.ValidateV2ReleaseConfigStructure(&releaseconfig.V2ReleaseConfig{
		SchemaVersion: 2,
		Units:         []releaseconfig.V2Unit{unit},
	})
}

func classifyUnitOverviewConfigError(err error) (string, string, string) {
	message := err.Error()
	switch {
	case strings.Contains(message, "tagPrefix"):
		return "UNIT_TAG_PREFIX_INVALID", "The configured tag prefix is invalid.", "Use the canonical safe tag-prefix policy for the unit."
	case strings.Contains(message, "delivery"):
		return "UNIT_DELIVERY_INVALID", "The configured V2 delivery is invalid.", "Use github-actions delivery for the Release V2 unit."
	case strings.Contains(message, "executor"):
		return "UNIT_EXECUTOR_INVALID", "The configured release executor is invalid.", "Select a supported canonical Release V2 executor."
	case strings.Contains(message, "workflow"):
		return "UNIT_WORKFLOW_PATH_INVALID", "The configured workflow path is invalid.", "Use a canonical repository-relative .github/workflows/*.yml or .yaml path."
	default:
		return "UNIT_CONFIG_INVALID", "The configured unit metadata is invalid.", "Repair the unit using the canonical Release V2 config policy."
	}
}

func appendUnitOverviewConfigConflicts(
	rows []unitOverviewRow,
	units []releaseconfig.V2Unit,
	valid []bool,
) {
	for left := 0; left < len(units); left++ {
		for right := left + 1; right < len(units); right++ {
			if !valid[left] || !valid[right] {
				continue
			}
			err := releaseconfig.ValidateV2ReleaseConfigStructure(&releaseconfig.V2ReleaseConfig{
				SchemaVersion: 2,
				Units:         []releaseconfig.V2Unit{units[left], units[right]},
			})
			if err == nil {
				continue
			}
			if strings.Contains(err.Error(), "tagPrefix") && strings.Contains(err.Error(), "overlaps") {
				for _, index := range []int{left, right} {
					appendUnitOverviewIssue(&rows[index], unitOverviewIssueError, "UNIT_TAG_PREFIX_CONFLICT", "The unit tag prefix overlaps another configured unit.", "Assign every release unit a distinct, non-overlapping tag prefix.")
				}
				continue
			}
			if units[left].ID == units[right].ID {
				for _, index := range []int{left, right} {
					appendUnitOverviewIssue(&rows[index], unitOverviewIssueError, "UNIT_CONFIG_INVALID", "The unit id is configured more than once.", "Keep exactly one config entry for each canonical unit id.")
				}
			}
		}
	}
}

func classifyUnitOverviewAlignment(row unitOverviewRow) unitOverviewAlignment {
	for _, issue := range row.Issues {
		if issue.Severity == unitOverviewIssueError {
			return unitOverviewInvalid
		}
	}
	if row.ConfigPresent && !row.StatePresent {
		return unitOverviewConfigOnly
	}
	if !row.ConfigPresent && row.StatePresent {
		return unitOverviewStateOnly
	}
	return unitOverviewAligned
}

func appendUnitOverviewIssue(
	row *unitOverviewRow,
	severity unitOverviewIssueSeverity,
	code, message, remediation string,
) {
	if hasUnitOverviewIssue(row.Issues, code) {
		return
	}
	row.Issues = append(row.Issues, unitOverviewIssue{
		Severity: severity, Unit: row.ID, Code: code,
		Message: message, Remediation: remediation,
	})
}

func hasUnitOverviewIssue(issues []unitOverviewIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func unitOverviewIssueCodes(issues []unitOverviewIssue) []any {
	codes := make([]any, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.Code)
	}
	return codes
}
