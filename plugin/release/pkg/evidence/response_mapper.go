package evidence

import (
	"fmt"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
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
		RendererHint: "table",
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
