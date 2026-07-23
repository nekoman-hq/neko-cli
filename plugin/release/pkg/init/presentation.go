package init

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func initializeV2SummaryPresentation(result initializeV2Result) *presentation.Properties {
	unit := result.Unit
	nextSteps := buildV2NextSteps(unit)
	return &presentation.Properties{
		Title: "Release Initialization",
		Properties: []presentation.Property{
			{Label: "Result", Value: "Repository initialized", Role: presentation.StyleSuccess, Emphasized: true},
			{Label: "Initialized unit", Value: unit.UnitID, Emphasized: true},
			{Label: "Version", Value: unit.Version},
			{Label: "Executor", Value: string(unit.Executor)},
			{Label: "Delivery", Value: string(unit.Delivery)},
			{Label: "Configuration", Value: config.V2ConfigPath(".")},
			{Label: "State", Value: config.V2StatePath(".")},
			{Label: "Next action", Value: firstSetupNextStep(nextSteps)},
		},
	}
}

func addV2UnitSummaryPresentation(result addV2UnitResult) *presentation.Properties {
	unit := result.Unit
	return &presentation.Properties{
		Title: "Release Unit Added",
		Properties: []presentation.Property{
			{Label: "Result", Value: "Unit appended", Role: presentation.StyleSuccess, Emphasized: true},
			{Label: "Added unit", Value: unit.UnitID, Emphasized: true},
			{Label: "Version", Value: unit.Version},
			{Label: "Executor", Value: string(unit.Executor)},
			{Label: "Delivery", Value: string(unit.Delivery)},
			{Label: "Configuration", Value: config.V2ConfigPath(".")},
			{Label: "State", Value: config.V2StatePath(".")},
			{Label: "Next action", Value: fmt.Sprintf("Run 'neko release validate --unit %s --show'", unit.UnitID)},
		},
	}
}

func firstSetupNextStep(steps []string) string {
	if len(steps) == 0 {
		return "Run 'neko release validate --show'"
	}
	return steps[0]
}

func initializeV2DetailPresentation(result initializeV2Result) *presentation.Table {
	resolved := setupResolvedUnitTable("Resolved Configuration", result.Unit)
	resolved.Details.Properties = append(resolved.Details.Properties,
		presentation.Property{Label: "Force behavior", Value: initializeForceBehavior(result)},
	)
	resolved.Following = setupArtifactTable([]setupArtifactFact{
		{Path: config.V2ConfigPath("."), Action: setupActionLabel(result.ConfigAction), Outcome: "Written"},
		{Path: config.V2StatePath("."), Action: setupActionLabel(result.StateAction), Outcome: "Written"},
	})
	resolved.Following.Following = setupValidationTable("Initialization", "Generated pair valid", "Persisted atomically")
	resolved.Following.Following.Following = setupLimitationsTable(
		"Workflow files are referenced but not created by init.",
		"Release execution and publication are not started.",
		"Plugin registry output is not generated.",
	)
	return resolved
}

func addV2UnitDetailPresentation(result addV2UnitResult) *presentation.Table {
	resolved := setupResolvedUnitTable("Resolved Unit", result.Unit)
	resolved.Following = &presentation.Table{
		Title: "Existing Unit Comparison", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "check", Label: "Check", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "details", Label: "Details"},
		},
		Rows: []map[string]any{{
			"check": "Duplicate identity", "status": "Not present",
			"details": fmt.Sprintf("Compared with %d existing unit(s)", result.ExistingUnitCount),
		}},
	}
	resolved.Following.Following = setupArtifactTable([]setupArtifactFact{
		{Path: config.V2ConfigPath("."), Action: "Update", Outcome: "Written"},
		{Path: config.V2StatePath("."), Action: "Update", Outcome: "Written"},
	})
	resolved.Following.Following.Following = setupValidationTable("Unit addition", "Updated pair valid", "Persisted atomically")
	resolved.Following.Following.Following.Following = setupLimitationsTable(
		"Existing unit identities are never replaced.",
		"Workflow files are referenced but not created by unit-add.",
		"Release execution and publication are not started.",
	)
	return resolved
}

func setupResolvedUnitTable(title string, unit v2InitConfig) *presentation.Table {
	properties := []presentation.Property{
		{Label: "Unit", Value: unit.UnitID},
		{Label: "Display name", Value: unit.DisplayName},
		{Label: "Version", Value: unit.Version},
		{Label: "Kind", Value: unit.Kind},
		{Label: "Executor", Value: string(unit.Executor)},
		{Label: "Delivery", Value: string(unit.Delivery)},
		{Label: "Workflow", Value: setupSafePath(unit.Workflow)},
		{Label: "Tag prefix", Value: unit.TagPrefix},
		{Label: "Working directory", Value: setupSafePath(unit.WorkingDirectory)},
		{Label: "Declared paths", Value: setupSafePaths(unit.Paths)},
	}
	if unit.Kind == pluginKind {
		properties = append(properties,
			presentation.Property{Label: "Plugin name", Value: unit.Plugin.Name},
			presentation.Property{Label: "Plugin manifest", Value: setupSafePath(unit.Plugin.Manifest)},
			presentation.Property{Label: "Plugin asset prefix", Value: unit.Plugin.AssetPrefix},
			presentation.Property{Label: "Plugin binary", Value: unit.Plugin.BinaryName},
			presentation.Property{Label: "Plugin index action", Value: "Not generated; validate separately"},
		)
	}
	return &presentation.Table{
		Title: title, DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "version", Label: "Version", Essential: true},
			{Key: "kind", Label: "Kind", Essential: true},
			{Key: "executor", Label: "Executor"},
			{Key: "delivery", Label: "Delivery"},
			{Key: "workflow", Label: "Workflow"},
		},
		Rows: []map[string]any{{
			"unit": unit.UnitID, "version": unit.Version, "kind": unit.Kind,
			"executor": string(unit.Executor), "delivery": string(unit.Delivery),
			"workflow": setupSafePath(unit.Workflow),
		}},
		Details: &presentation.Properties{SectionTitle: "Complete resolved unit", Properties: properties},
	}
}

type setupArtifactFact struct {
	Path    string
	Action  string
	Outcome string
}

func setupArtifactTable(facts []setupArtifactFact) *presentation.Table {
	rows := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		rows = append(rows, map[string]any{
			"artifact": setupSafePath(fact.Path), "action": fact.Action, "outcome": fact.Outcome,
		})
	}
	return &presentation.Table{
		Title: "Artifact Write Plan", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "artifact", Label: "Artifact", Essential: true},
			{Key: "action", Label: "Action", Essential: true},
			{Key: "outcome", Label: "Outcome", Essential: true},
		},
		Rows: rows,
	}
}

func setupValidationTable(scope, validation, persistence string) *presentation.Table {
	return &presentation.Table{
		Title: "Validation Facts", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "check", Label: "Check", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "details", Label: "Details"},
		},
		Rows: []map[string]any{
			{"check": "Command scope", "status": "Valid", "details": scope},
			{"check": "Configuration and state", "status": "Valid", "details": validation},
			{"check": "Write outcome", "status": "Complete", "details": persistence},
		},
	}
}

func setupLimitationsTable(statements ...string) *presentation.Table {
	rows := make([]map[string]any, 0, len(statements))
	for _, statement := range statements {
		rows = append(rows, map[string]any{"scope": "Command boundary", "details": statement})
	}
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "scope", Label: "Scope", Essential: true},
			{Key: "details", Label: "Details", Essential: true},
		},
		Rows: rows,
	}
}

func initializeForceBehavior(result initializeV2Result) string {
	if result.Force {
		return "Enabled; existing V2 artifacts may be replaced as one validated pair"
	}
	return "Not requested; existing V2 artifacts cause a refusal"
}

func setupActionLabel(action string) string {
	if strings.EqualFold(action, "replaced") {
		return "Replace"
	}
	return "Create"
}

func initializationFailurePresentation(failure *commandFailure) *presentation.Table {
	if failure == nil {
		return nil
	}
	title := "Action Required"
	area := "Initialization"
	force := "No"
	remediation := "Correct the reported input or repository state and retry."
	switch failure.code {
	case "CONFIG_EXISTS":
		title, area, force = "Conflict", "V2 configuration", "Yes"
		remediation = "Review the existing pair, then use --force only when replacing both V2 files is intended."
	case "CONFIG_CONFLICT":
		title, area = "Conflict", "Release sources"
		remediation = "Resolve the V1/V2 source conflict explicitly before retrying."
	case "V1_CONFIG_EXISTS":
		title, area = "Conflict", ".release.neko.json"
		remediation = "Run 'neko release migrate' instead; init never overwrites the V1 source."
	case "V2_CONFIG_MISSING":
		title, area = "Action Required", "V2 configuration"
		remediation = "Run 'neko release init' before adding a unit."
	case "PARTIAL_V2_CONFIG":
		title, area = "Conflict", "V2 configuration pair"
		remediation = "Restore or remove the incomplete pair before retrying."
	case "DUPLICATE_UNIT":
		title, area = "Conflict", "Duplicate unit"
		remediation = "Choose a new unit id; unit-add never overwrites an existing identity."
	case "LOAD_ERROR":
		area = "V2 configuration pair"
		remediation = "Repair the unreadable config or state file before retrying."
	case "SAVE_ERROR":
		area = "V2 configuration pair"
		remediation = "Resolve the filesystem write failure; the pair writer preserves atomic recovery evidence."
	}
	return &presentation.Table{
		Title: title,
		Columns: []presentation.Column{
			{Key: "code", Label: "Code", Essential: true},
			{Key: "area", Label: "Area", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "reason", Label: "Reason", Essential: true},
			{Key: "force", Label: "Force applicable", Essential: true},
			{Key: "remediation", Label: "Remediation", Essential: true},
		},
		Rows: []map[string]any{{
			"code": failure.code, "area": area, "status": "Refused", "reason": failure.message,
			"force": force, "remediation": remediation,
		}},
	}
}

func setupSafePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "not configured"
	}
	if filepath.IsAbs(value) {
		return "repository-relative artifact"
	}
	return filepath.ToSlash(value)
}

func setupSafePaths(values []string) string {
	if len(values) == 0 {
		return "none configured"
	}
	safe := make([]string, 0, len(values))
	for _, value := range values {
		safe = append(safe, setupSafePath(value))
	}
	return strings.Join(safe, "\n")
}
