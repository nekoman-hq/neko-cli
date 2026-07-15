package contributors

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

type contributorsResponseClock interface {
	Now() time.Time
}

type systemContributorsResponseClock struct{}

func (systemContributorsResponseClock) Now() time.Time {
	return time.Now()
}

func mapContributorsQueryResponse(result contributorsQueryResult, failure *contributorsQueryFailure, timestamp time.Time) *plugin.Response {
	metadataValue := plugin.ResponseMetadata{
		Plugin:    metadata.PluginName,
		Version:   metadata.Version,
		Command:   "contributors",
		Timestamp: timestamp,
	}
	if failure != nil {
		return &plugin.Response{
			Status:   "error",
			Metadata: metadataValue,
			Error: &plugin.ResponseError{
				Code:    failure.Code,
				Message: failure.Message,
			},
		}
	}

	items := make([]map[string]any, 0, len(result.Contributors))
	for _, contributor := range result.Contributors {
		items = append(items, map[string]any{
			"author":  contributor.Author,
			"commits": contributor.Commits,
		})
	}
	return &plugin.Response{
		Status:   "success",
		Metadata: metadataValue,
		Data: map[string]any{
			"items": items,
		},
	}
}
