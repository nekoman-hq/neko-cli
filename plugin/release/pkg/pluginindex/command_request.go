package pluginindex

import "fmt"

type pluginIndexCommandMode string

const (
	pluginIndexCheckMode   pluginIndexCommandMode = "check"
	pluginIndexRenderMode  pluginIndexCommandMode = "render"
	pluginIndexPersistMode pluginIndexCommandMode = "persist"
)

type pluginIndexCommandRequest struct {
	Repository string
	OutputPath string
	Mode       pluginIndexCommandMode
	Pretty     bool
}

func parsePluginIndexCommandRequest(flags map[string]any) (pluginIndexCommandRequest, error) {
	outputPath := pluginIndexFlagString(flags, "output")
	check := pluginIndexFlagBool(flags, "check")
	if check && outputPath != "" {
		return pluginIndexCommandRequest{}, fmt.Errorf("--check cannot be used with --output")
	}

	repository := pluginIndexFlagString(flags, "repository")
	if repository == "" {
		repository = DefaultRepository
	}
	request := pluginIndexCommandRequest{
		Repository: repository,
		OutputPath: outputPath,
		Mode:       pluginIndexRenderMode,
		Pretty:     pluginIndexFlagBoolDefault(flags, "pretty", true),
	}
	if check {
		request.Mode = pluginIndexCheckMode
	} else if outputPath != "" {
		request.Mode = pluginIndexPersistMode
	}
	return request, nil
}

func pluginIndexFlagString(flags map[string]any, name string) string {
	if value, ok := flags[name].(string); ok {
		return value
	}
	return ""
}

func pluginIndexFlagBool(flags map[string]any, name string) bool {
	if value, ok := flags[name].(bool); ok {
		return value
	}
	return false
}

func pluginIndexFlagBoolDefault(flags map[string]any, name string, fallback bool) bool {
	if value, ok := flags[name].(bool); ok {
		return value
	}
	return fallback
}
