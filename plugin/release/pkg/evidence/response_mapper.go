package evidence

import (
	"fmt"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

type evidenceResponseClock interface {
	Now() time.Time
}

type systemEvidenceResponseClock struct{}

func (systemEvidenceResponseClock) Now() time.Time {
	return time.Now()
}

func mapEvidenceQueryResponse(result evidenceQueryResult, timestamp time.Time) *plugin.Response {
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   CommandName,
			Timestamp: timestamp,
		},
		Data: map[string]any{
			"items":       evidenceResponseItems(result),
			"evidence":    result.Records,
			"diagnostics": result.Diagnostics,
		},
		RendererHint:      "table",
		PresentationTable: evidenceSummaryTable(),
	}
}

func mapEvidenceDetailResponse(result evidenceQueryResult, timestamp time.Time) *plugin.Response {
	record := result.Records[0]
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   CommandName,
			Timestamp: timestamp,
		},
		Data: map[string]any{
			"items":       evidenceDetailItems(record),
			"evidence":    result.Records,
			"diagnostics": result.Diagnostics,
		},
		RendererHint: "table",
	}
}

func evidenceSummaryTable() *presentation.Table {
	return &presentation.Table{Columns: []presentation.Column{
		{Key: "family", Label: "Family", Essential: true},
		{Key: "state", Label: "State", Essential: true},
		{Key: "classification", Label: "Classification", Essential: true},
		{Key: "safe_to_resume", Label: "Resume", Essential: true},
		{Key: "manual_recovery", Label: "Recovery", Essential: true},
		{Key: "unit", Label: "Unit"},
		{Key: "version", Label: "Version"},
		{Key: "tag", Label: "Tag"},
		{Key: "pending_action", Label: "Pending action"},
		{Key: "automatic_continuation", Label: "Automatic"},
		{Key: "lifecycle", Label: "Lifecycle"},
	}}
}

func evidenceDetailItems(record EvidenceRecord) []map[string]any {
	return []map[string]any{
		{"property": "Family", "value": record.Family},
		{"property": "Identity", "value": record.Identity},
		{"property": "Owner", "value": record.Owner},
		{"property": "Unit", "value": emptyEvidenceValue(record.Unit)},
		{"property": "Version", "value": emptyEvidenceValue(record.Version)},
		{"property": "Tag", "value": emptyEvidenceValue(record.Tag)},
		{"property": "State", "value": emptyEvidenceValue(record.State)},
		{"property": "Pending Action", "value": emptyEvidenceValue(record.PendingAction)},
		{"property": "Classification", "value": record.Classification},
		{"property": "Safe To Resume", "value": fmt.Sprintf("%t", record.SafeToResume)},
		{"property": "Automatic Continuation", "value": fmt.Sprintf("%t", record.AutomaticContinuation)},
		{"property": "Manual Recovery", "value": fmt.Sprintf("%t", record.ManualRecovery)},
		{"property": "Lifecycle Allowed", "value": fmt.Sprintf("%t", record.LifecycleAllowed)},
		{"property": "Lifecycle Operation", "value": emptyEvidenceValue(record.LifecycleOperation)},
		{"property": "Guidance", "value": record.Guidance},
		{"property": "Path", "value": record.Path},
		{"property": "Digest SHA-256", "value": record.DigestSHA256},
		{"property": "Created At", "value": emptyEvidenceValue(record.CreatedAt)},
		{"property": "Updated At", "value": emptyEvidenceValue(record.UpdatedAt)},
	}
}

func mapEvidenceArchiveResponse(result evidenceArchiveResult, timestamp time.Time) *plugin.Response {
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   ArchiveCommandName,
			Timestamp: timestamp,
		},
		Data: map[string]any{
			"items": []map[string]any{
				{"property": "Family", "value": result.Family},
				{"property": "Identity", "value": result.Identity},
				{"property": "Digest", "value": result.DigestSHA256},
				{"property": "Source", "value": result.SourcePath},
				{"property": "Archive", "value": result.ArchivePath},
				{"property": "Status", "value": "archived"},
			},
		},
		RendererHint: "table",
	}
}

func evidenceResponseItems(result evidenceQueryResult) []map[string]any {
	items := make([]map[string]any, 0, len(result.Records)+len(result.Diagnostics)+1)
	for _, record := range result.Records {
		items = append(items, map[string]any{
			"family":                 record.Family,
			"identity":               record.Identity,
			"owner":                  record.Owner,
			"unit":                   emptyEvidenceValue(record.Unit),
			"version":                emptyEvidenceValue(record.Version),
			"tag":                    emptyEvidenceValue(record.Tag),
			"state":                  emptyEvidenceValue(record.State),
			"pending_action":         emptyEvidenceValue(record.PendingAction),
			"classification":         record.Classification,
			"safe_to_resume":         fmt.Sprintf("%t", record.SafeToResume),
			"automatic_continuation": fmt.Sprintf("%t", record.AutomaticContinuation),
			"manual_recovery":        fmt.Sprintf("%t", record.ManualRecovery),
			"lifecycle":              lifecycleEvidenceValue(record),
			"digest":                 record.DigestSHA256,
			"path":                   record.Path,
			"guidance":               record.Guidance,
		})
	}
	for _, diagnostic := range result.Diagnostics {
		items = append(items, map[string]any{
			"family":                 diagnostic.Family,
			"identity":               "diagnostic",
			"owner":                  "evidence inspection",
			"unit":                   "not applicable",
			"version":                "not applicable",
			"tag":                    "not applicable",
			"state":                  diagnostic.Code,
			"pending_action":         "manual-inspection",
			"classification":         diagnostic.Classification,
			"safe_to_resume":         "false",
			"automatic_continuation": "false",
			"manual_recovery":        "true",
			"lifecycle":              "blocked",
			"digest":                 "not applicable",
			"path":                   diagnostic.Path,
			"guidance":               diagnostic.Guidance,
		})
	}
	if len(items) == 0 {
		items = append(items, map[string]any{
			"family":                 "none",
			"identity":               "not applicable",
			"owner":                  "evidence inspection",
			"unit":                   "not applicable",
			"version":                "not applicable",
			"tag":                    "not applicable",
			"state":                  "missing",
			"pending_action":         "none",
			"classification":         "completed",
			"safe_to_resume":         "false",
			"automatic_continuation": "false",
			"manual_recovery":        "false",
			"lifecycle":              "not applicable",
			"digest":                 "not applicable",
			"path":                   "not applicable",
			"guidance":               "No release evidence files were found.",
		})
	}
	return items
}

func lifecycleEvidenceValue(record EvidenceRecord) string {
	if record.LifecycleAllowed {
		return record.LifecycleOperation
	}
	return "blocked"
}

func emptyEvidenceValue(value string) string {
	if value == "" {
		return "not applicable"
	}
	return value
}
