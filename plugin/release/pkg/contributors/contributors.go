package contributors

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

func HandleContributors(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Collecting contributors")

	repository, err := config.LoadReleaseRepository(".")
	if err != nil {
		return errorResponse("contributors", "CONFIG_INVALID", err.Error()), nil
	}
	unit, err := config.ResolveReleaseUnit(repository, getFlagString(req.Flags, "unit"), config.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return errorResponse("contributors", "UNIT_RESOLUTION_FAILED", err.Error()), nil
	}

	var contributors []git.Contributor
	if repository.SourceFormat == config.SourceFormatV2 {
		contributors, err = git.ContributorsForPaths(unit.Paths)
	} else {
		contributors, err = git.Contributors()
	}
	if err != nil {
		return errorResponse("contributors", "GIT_CONTRIBUTORS_FAILED", err.Error()), nil
	}

	items := make([]map[string]any, 0, len(contributors))
	for _, c := range contributors {
		items = append(items, map[string]any{
			"author":  c.Author,
			"commits": c.Commits,
		})
	}

	log.PluginPrint(log.Exec, "Successfully collected contributors")

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "contributors",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": items,
		},
	}, nil
}

func errorResponse(command, code, message string) *plugin.Response {
	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   command,
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

func getFlagString(flags map[string]any, key string) string {
	if value, ok := flags[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}
