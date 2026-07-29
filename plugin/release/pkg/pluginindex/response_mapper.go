package pluginindex

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

type pluginIndexResponseClock interface {
	Now() time.Time
}

type systemPluginIndexResponseClock struct{}

func (systemPluginIndexResponseClock) Now() time.Time {
	return time.Now()
}

func mapPluginIndexCommandResponse(result pluginIndexCommandResult, timestamp time.Time) *plugin.Response {
	data, rendererHint := pluginIndexResponseData(result)
	response := &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   CommandName,
			Timestamp: timestamp,
		},
		Data:         data,
		RendererHint: rendererHint,
	}
	attachPluginIndexPresentation(response, result)
	return response
}

func pluginIndexResponseData(result pluginIndexCommandResult) (map[string]any, string) {
	switch result.Mode {
	case pluginIndexCheckMode:
		return map[string]any{
			"items": []map[string]any{
				{"property": "Status", "value": "ok"},
				{"property": "Plugins", "value": result.Plugins},
				{"property": "Repository", "value": result.Repository},
			},
		}, ""
	case pluginIndexPersistMode:
		return map[string]any{
			"items": []map[string]any{
				{"property": "Status", "value": "written"},
				{"property": "Output", "value": result.OutputPath},
				{"property": "Plugins", "value": result.Plugins},
				{"property": "Repository", "value": result.Repository},
			},
		}, ""
	default:
		return map[string]any{"raw": result.RawOutput}, "raw-json"
	}
}
