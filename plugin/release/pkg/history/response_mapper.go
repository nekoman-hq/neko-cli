package history

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

type historyResponseClock interface {
	Now() time.Time
}

type systemHistoryResponseClock struct{}

func (systemHistoryResponseClock) Now() time.Time {
	return time.Now()
}

func mapHistoryQueryResponse(result historyQueryResult, failure *historyQueryFailure, timestamp time.Time) *plugin.Response {
	metadataValue := plugin.ResponseMetadata{
		Plugin:    metadata.PluginName,
		Version:   metadata.Version,
		Command:   "history",
		Timestamp: timestamp,
	}
	if failure != nil {
		response := &plugin.Response{
			Status:   "error",
			Metadata: metadataValue,
			Error: &plugin.ResponseError{
				Code:    failure.Code,
				Message: failure.Message,
			},
		}
		response.SetExitCode(1)
		return response
	}

	items := make([]map[string]any, 0, len(result.Entries))
	for _, entry := range result.Entries {
		item := map[string]any{
			"version": entry.Version,
			"from":    entry.From,
			"commits": entry.Commits,
		}
		if result.SourceFormat == config.SourceFormatV2 {
			item["unit"] = entry.Unit
		}
		items = append(items, item)
	}
	response := &plugin.Response{
		Status:   "success",
		Metadata: metadataValue,
		Data: map[string]any{
			"items": items,
		},
	}
	response.SetExitCode(0)
	return response
}
