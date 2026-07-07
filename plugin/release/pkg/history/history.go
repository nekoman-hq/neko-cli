package history

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      29.12.2025
*/

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

func HandleHistory(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Starting release history")

	repository, err := config.LoadReleaseRepository(".")
	if err != nil {
		return errorResponse("history", "CONFIG_INVALID", err.Error()), nil
	}
	unit, err := config.ResolveReleaseUnit(repository, getFlagString(req.Flags, "unit"), config.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return errorResponse("history", "UNIT_RESOLUTION_FAILED", err.Error()), nil
	}

	if repository.SourceFormat == config.SourceFormatV2 {
		return handleV2History(*unit)
	}

	tagList := git.GetTags()
	log.PluginV(log.Exec, "Found %d tags", len(tagList))

	// Build tag history with commit counts between tags
	items := make([]map[string]any, 0, len(tagList))
	for i := range tagList {
		var commitCount int
		var from string

		if i == 0 {
			commitCount = git.CountCommitsBetween("", tagList[i])
			from = ""
		} else {
			commitCount = git.CountCommitsBetween(tagList[i-1], tagList[i])
			from = tagList[i-1]
		}

		items = append(items, map[string]any{
			"version": tagList[i],
			"from":    from,
			"commits": commitCount,
		})
	}

	log.PluginPrint(log.Exec, "Release history completed")

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "history",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": items,
		},
	}, nil
}

func handleV2History(unit config.ReleaseUnit) (*plugin.Response, error) {
	spec, err := config.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return errorResponse("history", "TAG_SPEC_INVALID", err.Error()), nil
	}
	unitTags, err := git.UnitTagsInHistory(spec)
	if err != nil {
		return errorResponse("history", "GIT_HISTORY_FAILED", err.Error()), nil
	}

	items := make([]map[string]any, 0, len(unitTags))
	for i, tag := range unitTags {
		var from string
		if i > 0 {
			from = unitTags[i-1].Tag
		}
		commitCount, err := git.CountCommitsBetweenPaths(from, tag.Tag, unit.Paths)
		if err != nil {
			return errorResponse("history", "GIT_HISTORY_FAILED", err.Error()), nil
		}
		items = append(items, map[string]any{
			"unit":    unit.ID,
			"version": tag.Tag,
			"from":    from,
			"commits": commitCount,
		})
	}

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "history",
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
