package pluginindex

import (
	"fmt"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func attachPluginIndexPresentation(response *plugin.Response, result pluginIndexCommandResult) {
	if response == nil || result.Mode == pluginIndexRenderMode {
		return
	}
	switch result.Mode {
	case pluginIndexCheckMode:
		response.PresentationProperties = pluginIndexCheckSummary(result)
	case pluginIndexPersistMode:
		response.PresentationProperties = pluginIndexPersistSummary(result)
	}
	response.PresentationTable = pluginIndexDetailTables(result)
}

func pluginIndexCheckSummary(result pluginIndexCommandResult) *presentation.Properties {
	return &presentation.Properties{
		Title: "Plugin Index Check",
		Properties: []presentation.Property{
			{Label: "Source scope", Value: "V2 release config, state, and declared plugin manifests"},
			{Label: "Repository", Value: result.Repository, Emphasized: true},
			{Label: "Validation", Value: "Valid", Role: presentation.StyleSuccess, Emphasized: true},
			{Label: "Repositories", Value: 1},
			{Label: "Plugins", Value: result.Plugins},
			{Label: "Next action", Value: "Use --output-file <path> to persist the validated schema-v1 artifact.", Emphasized: true},
		},
	}
}

func pluginIndexPersistSummary(result pluginIndexCommandResult) *presentation.Properties {
	return &presentation.Properties{
		Title: "Plugin Index Write",
		Properties: []presentation.Property{
			{Label: "Result", Value: "Written", Role: presentation.StyleSuccess, Emphasized: true},
			{Label: "Output file", Value: pluginIndexSafeTargetLabel(result.Target), Emphasized: true},
			{Label: "Format", Value: pluginIndexFormattingLabel(result.Pretty)},
			{Label: "Repositories", Value: 1},
			{Label: "Plugins", Value: result.Plugins},
			{Label: "Validation", Value: "Valid", Role: presentation.StyleSuccess},
			{Label: "Next action", Value: "Inspect the generated artifact; publication remains a separate external operation.", Emphasized: true},
		},
	}
}

func pluginIndexDetailTables(result pluginIndexCommandResult) *presentation.Table {
	tables := []*presentation.Table{
		pluginIndexSourceResolutionTable(result),
		pluginIndexRepositoryInventoryTable(result),
		pluginIndexPluginInventoryTable(result),
		pluginIndexValidationChecksTable(result),
	}
	if result.Mode == pluginIndexPersistMode {
		tables = append(tables, pluginIndexWritePlanTable(result))
	}
	tables = append(tables, pluginIndexLimitationsTable(result.Mode))
	return chainPluginIndexTables(tables)
}

func pluginIndexSourceResolutionTable(result pluginIndexCommandResult) *presentation.Table {
	rows := []map[string]any{
		{"source": "V2 release configuration", "location": ".neko/release.config.json", "result": "Resolved"},
		{"source": "V2 release state", "location": ".neko/release.state.json", "result": "Resolved"},
	}
	for _, entry := range pluginIndexResultEntries(result) {
		rows = append(rows, map[string]any{
			"source":   "Plugin manifest " + entry.Name,
			"location": entry.Manifest,
			"result":   "Resolved",
		})
	}
	return &presentation.Table{
		Title: "Source Resolution", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "source", Label: "Source", Essential: true},
			{Key: "location", Label: "Repository path", Essential: true},
			{Key: "result", Label: "Result", Essential: true},
		},
		Rows: rows,
	}
}

func pluginIndexRepositoryInventoryTable(result pluginIndexCommandResult) *presentation.Table {
	return &presentation.Table{
		Title: "Repository Inventory", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "repository", Label: "Repository", Essential: true},
			{Key: "count", Label: "Count", Essential: true},
			{Key: "ordering", Label: "Ordering", Essential: true},
		},
		Rows: []map[string]any{{
			"repository": result.Repository,
			"count":      1,
			"ordering":   "Single requested repository identifier",
		}},
	}
}

func pluginIndexPluginInventoryTable(result pluginIndexCommandResult) *presentation.Table {
	rows := make([]map[string]any, 0, result.Plugins)
	for _, entry := range pluginIndexResultEntries(result) {
		rows = append(rows, map[string]any{
			"name": entry.Name, "unit": entry.Unit, "version": entry.Version, "tag": entry.Tag,
			"tag_prefix": entry.TagPrefix, "manifest": entry.Manifest, "asset_prefix": entry.AssetPrefix,
			"binary_name": entry.BinaryName, "description": entry.Description,
		})
	}
	return &presentation.Table{
		Title: "Plugin Inventory", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "name", Label: "Plugin", Essential: true},
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "version", Label: "Version", Essential: true},
			{Key: "tag", Label: "Tag", Essential: true},
			{Key: "tag_prefix", Label: "Tag prefix"},
			{Key: "manifest", Label: "Manifest"},
			{Key: "asset_prefix", Label: "Asset prefix"},
			{Key: "binary_name", Label: "Binary name"},
			{Key: "description", Label: "Description"},
		},
		Rows: rows,
		Note: "Rows preserve the schema-v1 plugin-name ordering used by the raw artifact.",
	}
}

func pluginIndexValidationChecksTable(result pluginIndexCommandResult) *presentation.Table {
	return &presentation.Table{
		Title: "Validation Checks", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "check", Label: "Check", Essential: true},
			{Key: "result", Label: "Result", Essential: true},
			{Key: "facts", Label: "Facts", Essential: true},
		},
		Rows: []map[string]any{
			{"check": "Schema", "result": "Valid", "facts": fmt.Sprintf("schemaVersion=%d", pluginIndexResultSchema(result))},
			{"check": "Repository", "result": "Valid", "facts": "One non-empty repository identifier"},
			{"check": "Plugin records", "result": "Valid", "facts": fmt.Sprintf("%d complete plugin records", result.Plugins)},
			{"check": "Plugin ordering", "result": "Valid", "facts": "Ascending by plugin name"},
			{"check": "Duplicate unit ids", "result": "None", "facts": "Validated before manifest reads"},
			{"check": "Duplicate plugin names", "result": "None", "facts": "Every public plugin name is unique"},
			{"check": "Duplicate tags", "result": "None", "facts": "Every generated release tag is unique"},
			{"check": "Manifest consistency", "result": "Valid", "facts": "Manifest names and versions match V2 metadata and state"},
		},
	}
}

func pluginIndexWritePlanTable(result pluginIndexCommandResult) *presentation.Table {
	scope := "Repository-relative target"
	if result.Target.External {
		scope = "Explicit external artifact target"
	}
	return &presentation.Table{
		Title: "Write Plan", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "fact", Label: "Fact", Essential: true},
			{Key: "value", Label: "Value", Essential: true},
		},
		Rows: []map[string]any{
			{"fact": "Target", "value": pluginIndexSafeTargetLabel(result.Target)},
			{"fact": "Target scope", "value": scope},
			{"fact": "Formatting", "value": pluginIndexFormattingLabel(result.Pretty) + " schema-v1 JSON with trailing newline"},
			{"fact": "Validation", "value": "Source and target validation completed before write"},
			{"fact": "Parent directories", "value": "Create missing parents with mode 0755"},
			{"fact": "Replacement", "value": "Target-local atomic create or replace; preserve existing file mode"},
			{"fact": "Final outcome", "value": "Selected target written successfully"},
		},
	}
}

func pluginIndexLimitationsTable(mode pluginIndexCommandMode) *presentation.Table {
	mutation := "No files are written in check mode"
	if mode == pluginIndexPersistMode {
		mutation = "Only the selected output-file target is written"
	}
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "boundary", Label: "Boundary", Essential: true},
			{Key: "details", Label: "Details", Essential: true},
		},
		Rows: []map[string]any{
			{"boundary": "Source", "details": "Local V2 config, state, and plugin manifests only"},
			{"boundary": "Mutation", "details": mutation},
			{"boundary": "Remote state", "details": "No network, remote release, registry, or publication state is inspected"},
			{"boundary": "Publication", "details": "This command does not upload, publish, dispatch, tag, or commit"},
		},
	}
}

func pluginIndexResultEntries(result pluginIndexCommandResult) []PluginEntry {
	if result.Index == nil {
		return nil
	}
	return result.Index.Plugins
}

func pluginIndexResultSchema(result pluginIndexCommandResult) int {
	if result.Index == nil {
		return SchemaVersion
	}
	return result.Index.SchemaVersion
}

func pluginIndexFormattingLabel(pretty bool) string {
	if pretty {
		return "Pretty"
	}
	return "Compact"
}

func pluginIndexSafeTargetLabel(target pluginIndexOutputTarget) string {
	if target.ConfiguredPath == "" {
		return "not applicable"
	}
	if target.External {
		return filepath.ToSlash(filepath.Join("external", filepath.Base(target.AbsolutePath)))
	}
	return target.ConfiguredPath
}

func chainPluginIndexTables(tables []*presentation.Table) *presentation.Table {
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
