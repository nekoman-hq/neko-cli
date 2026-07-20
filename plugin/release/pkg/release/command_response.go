package release

import (
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

// MapCommandFailure maps an expected application failure using an explicitly
// supplied timestamp.
func MapCommandFailure(command string, failure *CommandFailure, timestamp time.Time) *plugin.Response {
	if failure == nil {
		return nil
	}
	return &plugin.Response{
		Status:   "error",
		Metadata: commandResponseMetadata(command, timestamp),
		Error: &plugin.ResponseError{
			Code:    failure.Code,
			Message: failure.responseMessage(),
			Details: cloneResponseDetails(failure.Details),
		},
	}
}

func successTableResponse(command string, timestamp time.Time, items []map[string]any) *plugin.Response {
	return &plugin.Response{
		Status:       "success",
		Metadata:     commandResponseMetadata(command, timestamp),
		Data:         map[string]any{"items": items},
		RendererHint: "table",
	}
}

func commandResponseMetadata(command string, timestamp time.Time) plugin.ResponseMetadata {
	return plugin.ResponseMetadata{
		Plugin:    metadata.PluginName,
		Version:   metadata.Version,
		Command:   command,
		Timestamp: timestamp,
	}
}

func cloneResponseDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}
	clone := make(map[string]any, len(details))
	for key, value := range details {
		clone[key] = value
	}
	return clone
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
