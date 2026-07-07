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
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

func HandleHistory() (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Starting release history")

	if config.V2ConfigExists(".") {
		return release.V2ExecutionUnavailableResponse("history"), nil
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
